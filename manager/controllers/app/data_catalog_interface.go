// Copyright 2020 IBM Corp.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"time"

	"encoding/json"

	"google.golang.org/grpc"

	app "github.com/ibm/the-mesh-for-data/manager/apis/app/v1alpha1"
	"github.com/ibm/the-mesh-for-data/manager/controllers/app/modules"
	"github.com/ibm/the-mesh-for-data/manager/controllers/utils"
	dc "github.com/ibm/the-mesh-for-data/pkg/connectors/protobuf"
	"github.com/ibm/the-mesh-for-data/pkg/serde"
	"github.com/ibm/the-mesh-for-data/pkg/vault"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetConnectionDetails calls the data catalog service
func GetConnectionDetails(req *modules.DataInfo, input *app.M4DApplication) error {
	// Set up a connection to the data catalog interface server.
	conn, err := grpc.Dial(utils.GetDataCatalogServiceAddress(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	c := dc.NewDataCatalogServiceClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var response *dc.CatalogDatasetInfo
	var credentialPath string
	if input.Spec.SecretRef != "" {
		credentialPath = vault.PathForReadingKubeSecret(input.Namespace, input.Spec.SecretRef)
	}

	if response, err = c.GetDatasetInfo(ctx, &dc.CatalogDatasetRequest{
		CredentialPath: credentialPath,
		DatasetId:      req.Context.DataSetID,
	}); err != nil {
		return err
	}

	details := response.GetDetails()
	protocol, err := utils.GetProtocol(details)
	if err != nil {
		return err
	}
	format, err := utils.GetDataFormat(details)
	if err != nil {
		return err
	}

	connection, err := serde.ToRawExtension(details.DataStore)
	if err != nil {
		return err
	}

	req.DataDetails = &modules.DataDetails{
		Name: details.Name,
		Interface: app.InterfaceDetails{
			Protocol:   protocol,
			DataFormat: format,
		},
		Geography:  details.Geo,
		Connection: *connection,
		Metadata:   details.Metadata,
	}
	req.VaultSecretPath = ""
	if details.CredentialsInfo != nil {
		req.VaultSecretPath = details.CredentialsInfo.VaultSecretPath
	}
	return nil
}

// GetCredentials calls the credentials manager service
// TODO: Choose appropriate catalog connector based on the datacatalog service indicated as part of datasetID
func GetCredentials(req *modules.DataInfo, input *app.M4DApplication) error {
	// Set up a connection to the data catalog interface server.
	conn, err := grpc.Dial(utils.GetCredentialsManagerServiceAddress(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	defer conn.Close()
	c := dc.NewDataCredentialServiceClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var credentialPath string
	if input.Spec.SecretRef != "" {
		credentialPath = vault.PathForReadingKubeSecret(input.Namespace, input.Spec.SecretRef)
	}

	dataCredentials, err := c.GetCredentialsInfo(ctx, &dc.DatasetCredentialsRequest{
		DatasetId:      req.Context.DataSetID,
		CredentialPath: credentialPath})
	if err != nil {
		return err
	}
	req.Credentials = dataCredentials

	return nil
}

// RegisterAsset registers a new asset in the specified catalog
// Input arguments:
// - catalogID: the destination catalog identifier
// - info: connection and credential details
// Returns:
// - an error if happened
// - the new asset identifier
func (r *M4DApplicationReconciler) RegisterAsset(catalogID string, info *app.DatasetDetails, input *app.M4DApplication) (string, error) {
	// Set up a connection to the data catalog interface server.
	conn, err := grpc.Dial(utils.GetDataCatalogServiceAddress(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return "", err
	}
	defer conn.Close()
	c := dc.NewDataCatalogServiceClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	datasetDetails := &dc.DatasetDetails{}
	if err := serde.FromRawExtention(info.Details, datasetDetails); err != nil {
		return "", err
	}
	var creds *dc.Credentials
	if creds, err = SecretToCredentials(r.Client, types.NamespacedName{Name: info.SecretRef, Namespace: utils.GetSystemNamespace()}); err != nil {
		return "", err
	}
	var credentialPath string
	if input.Spec.SecretRef != "" {
		credentialPath = vault.PathForReadingKubeSecret(input.Namespace, input.Spec.SecretRef)
	}

	response, err := c.RegisterDatasetInfo(ctx, &dc.RegisterAssetRequest{
		Creds:                creds,
		DatasetDetails:       datasetDetails,
		DestinationCatalogId: catalogID,
		CredentialPath:       credentialPath,
	})
	if err != nil {
		return "", err
	}
	return response.GetAssetId(), nil
}

var translationMap = map[string]string{
	"accessKeyID":        "access_key",
	"accessKey":          "access_key",
	"secretAccessKey":    "secret_key",
	"SecretKey":          "secret_key",
	"apiKey":             "api_key",
	"resourceInstanceId": "resource_instance_id",
}

// SecretToCredentialMap fetches a secret and converts into a map matching credentials proto
func SecretToCredentialMap(cl client.Client, secretRef types.NamespacedName) (map[string]interface{}, error) {
	// fetch a secret
	secret := &corev1.Secret{}
	if err := cl.Get(context.Background(), secretRef, secret); err != nil {
		return nil, err
	}
	credsMap := make(map[string]interface{})
	for key, val := range secret.Data {
		if translated, found := translationMap[key]; found {
			credsMap[translated] = string(val)
		} else {
			credsMap[key] = string(val)
		}
	}
	return credsMap, nil
}

// SecretToCredentials fetches a secret and constructs Credentials structure
func SecretToCredentials(cl client.Client, secretRef types.NamespacedName) (*dc.Credentials, error) {
	credsMap, err := SecretToCredentialMap(cl, secretRef)
	if err != nil {
		return nil, err
	}
	jsonStr, err := json.Marshal(credsMap)
	if err != nil {
		return nil, err
	}
	var creds dc.Credentials
	if err := json.Unmarshal(jsonStr, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}
