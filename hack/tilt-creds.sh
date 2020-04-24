#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..

cd "${REPO_ROOT}"

export AZURE_SUBSCRIPTION_ID_B64="$(cat sp.json | jq -r .subscriptionId | base64 | tr -d '\n')"
export AZURE_TENANT_ID_B64="$(cat sp.json | jq -r .tenantId  | base64 | tr -d '\n')"
export AZURE_CLIENT_ID_B64="$(cat sp.json | jq -r .clientId | base64 | tr -d '\n')"
export AZURE_CLIENT_SECRET_B64="$(cat sp.json | jq -r .clientSecret | base64 | tr -d '\n')"

SECRET="$(cat tilt-settings.json | envsubst)"

echo "$SECRET" > tilt-settings.json
