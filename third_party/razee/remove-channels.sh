#!/usr/bin/env bash

subscriptionsUUID=$(curl --request POST \
  --url https://config.satellite.cloud.ibm.com/graphql \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"query":"query {subscriptions(orgId: \"d322c41cf7aec2058677012167460554\") {\n\t  uuid\n\t}}"}' | jq -r -c '.data.subscriptions[].uuid')

echo "existing subscription uuids: $subscriptionsUUID"

# delete all subscriptions
for UUID in $subscriptionsUUID
do
  echo "deleting subscription uuid: $UUID"
  curl --request POST \
  --url https://config.satellite.cloud.ibm.com/graphql \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"query":"mutation ($orgId: String!, $uuid: String!) {\n  removeSubscription(orgId: $orgId, uuid: $uuid) {\n    success\n  }\n}\n\n\n","variables":{"orgId":"d322c41cf7aec2058677012167460554","uuid":"'"$UUID"'"}}'
done


channelsUUID=$(curl --request POST \
  --url https://config.satellite.cloud.ibm.com/graphql \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"query":"query {channels(orgId: \"d322c41cf7aec2058677012167460554\") {\n\t  uuid\n\t}}"}' | jq -r -c '.data.channels[].uuid')

echo "existing channel uuids: $channelsUUID"

# delete all channels
for UUID in $channelsUUID
do
  echo "deleting channel uuid: $UUID"
  curl --request POST \
  --url https://config.satellite.cloud.ibm.com/graphql \
  --header "Authorization: Bearer $TOKEN" \
  --header 'Content-Type: application/json' \
  --data '{"query":"mutation ($orgId: String!, $uuid: String!) {\n  removeChannel(orgId: $orgId, uuid: $uuid) {\n    success\n  }\n}\n\n\n","variables":{"orgId":"d322c41cf7aec2058677012167460554","uuid":"'"$UUID"'"}}'
done