# Develop with Tilt

A Tiltfile exists in the root of the repository which works well if you are
using a gopath friendly configuration. It will deploy CARP, CAPI, CAPZ, CAPBK
and KCP in a Kind cluster for testing. Then you can apply CRs for testing.

## Setup

The setup assumes a Go path. It also requires Azure credentials. The easiest way
it to use an sdk-auth file. Here's how to create-one easily. If you already have
an SP, you can login with it and do `az account show --sdk-auth` to retrieve a
similar format.

```bash
SP=sp.json
az ad sp create-for-rbac --role contributor --scope "/subscriptions/$(az account show | jq -r .id)" --sdk-auth > sp.json
DIR="$(go env GOPATH)"
mkdir -p DIR/sigs.k8s.io/cluster-api
mkdir -p DIR/sigs.k8s.io/cluster-api-provider-azure
git clone https://github.com/kubernetes-sigs/cluster-api DIR/sigs.k8s.io/cluster-api
git clone https://github.com/kubernetes-sigs/cluster-api-provider-azure DIR/sigs.k8s.io/cluster-api-provider-azure

# template tilt secrets from sp.json
./hack/tilt-credss.sh

# bring up cluster/dev env
./hack/kind-with-registry.sh
./hack/tilt-creds.sh # WILL USE sp.json!!!
tilt up
```

You should have a running tilt instance will of of CAPI/CAPZ/CAPBK/CARP running.

Apply some custom resources and test it out!
