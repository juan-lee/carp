#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

cd "${REPO_ROOT}"

export AZURE_SUBSCRIPTION_ID_B64="$(cat sp.json | jq -r .subscriptionId | tr -d '\n' | base64)"
export AZURE_TENANT_ID_B64="$(cat sp.json | jq -r .tenantId  | tr -d '\n' | base64)"
export AZURE_CLIENT_ID_B64="$(cat sp.json | jq -r .clientId | tr -d '\n' | base64)"
export AZURE_CLIENT_SECRET_B64="$(cat sp.json | jq -r .clientSecret | tr -d '\n' | base64)"

SECRET="$(cat tilt-settings.json | envsubst)"

echo "$SECRET" > tilt-settings.json
