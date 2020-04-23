# Worker Controller

The worker controller manages a set of Kubernetes clusters onto which pods for
end-user control planes may be scheduled

A CAPZ cluster requires  several components to deploy:
- KubeadmConfigTemplate
- Cluster
- KubeadmControlPlane
- AzureCluster
- MachineDeployment for control plane
- MachineDeployment for worker nodes
- AzureMachineTemplate for control plane
- AzureMachineTemplate for control plane

