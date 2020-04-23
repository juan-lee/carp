# Cluster API Resource Provider

Cluster API may be used to offer Kubernetes as a Service to users. While current
implementations use a Machine-based control plane, managed provider typically
provide pod-based control planes for efficiency. Due to the size of the
hyperscale provider fleets, managing all user control plane pods on a single
underlying Kubernetes cluster is not feasible.

The goal of this provider is to act as an orchestrator over Cluster API
management clusters.

A user requests the CARP operator to create a cluster, and the CARP operator
assigns it to a management cluster. The management cluster follows a standard
CAPI flow, and creates the cluster. 

To bootstrap the management cluster, the CARP operator will itself run an
instance of CAPI + CAPZ. When a user creates a cluster via CARP, CARP tries to
schedule it to a management cluster. To do so, CARP must be aware of management
cluster capacity. When no management cluster has capacity, CARP must create a
new cluster. It does so by creating a CAPI cluster object. When that cluster is
ready, CARP deployed the CAPI + CAPZ cluster to the newly deployed management
cluster. When the CAPI + CAPZ components are ready on the remote management
cluster, CARP will apply the origin user-requested cluster to the remote
management cluster, and wait for it to provision successfully.
