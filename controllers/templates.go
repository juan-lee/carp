package controllers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capbkv1alpha3 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	kcpv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"

	carpv1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
)

func getMachineDeployment(worker *carpv1alpha1.Worker) *capiv1alpha3.MachineDeployment {
	return &capiv1alpha3.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: worker.Name,
		},
		Spec: capiv1alpha3.MachineDeploymentSpec{
			ClusterName: worker.Name,
			Replicas:    to.Int32Ptr(worker.Spec.Replicas),
			Selector:    metav1.LabelSelector{},
			Template: capiv1alpha3.MachineTemplateSpec{
				Spec: capiv1alpha3.MachineSpec{
					ClusterName: worker.Name,
					Bootstrap: capiv1alpha3.Bootstrap{
						ConfigRef: &v1.ObjectReference{
							APIVersion: "bootstrap.cluster.x-k8s.io/v1alpha3",
							Name:       worker.Name,
							Kind:       "KubeadmConfigTemplate",
						},
					},
					InfrastructureRef: v1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
						Name:       worker.Name,
						Kind:       "AzureMachineTemplate",
					},
					Version: to.StringPtr(worker.Spec.Version),
				},
			},
		},
	}
}

func getMachineTemplate(cluster, location string) *capzv1alpha3.AzureMachineTemplate {
	return &capzv1alpha3.AzureMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
		Spec: capzv1alpha3.AzureMachineTemplateSpec{
			Template: capzv1alpha3.AzureMachineTemplateResource{
				Spec: capzv1alpha3.AzureMachineSpec{
					Location: location,
					OSDisk: capzv1alpha3.OSDisk{
						DiskSizeGB: 1024,
						ManagedDisk: capzv1alpha3.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
						OSType: "Linux",
					},
					VMSize: "Standard_D8s_v3",
				},
			},
		},
	}
}

func getCluster(cluster, location string, settings map[string]string) *capiv1alpha3.Cluster {
	return &capiv1alpha3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
		Spec: capiv1alpha3.ClusterSpec{
			ClusterNetwork: &capiv1alpha3.ClusterNetwork{
				Pods: &capiv1alpha3.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16"},
				},
			},
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: "controlplane.cluster.x-k8s.io/v1alpha3",
				Kind:       "KubeadmControlPlane",
				Name:       cluster,
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
				Kind:       "AzureCluster",
				Name:       cluster,
			},
		},
	}
}

func getAzureCluster(cluster, location string) *capzv1alpha3.AzureCluster {
	return &capzv1alpha3.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
		Spec: capzv1alpha3.AzureClusterSpec{
			Location: location,
			NetworkSpec: capzv1alpha3.NetworkSpec{
				Vnet: capzv1alpha3.VnetSpec{
					Name: fmt.Sprintf("%s-vnet", cluster),
				},
			},
			ResourceGroup: cluster,
		},
	}
}

func getKubeadmControlPlane(cluster, location string, settings map[string]string) (*kcpv1alpha3.KubeadmControlPlane, error) {
	data, err := getCloudProviderConfig(cluster, location, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud provider config")
	}
	replicas := int32(1)
	controlplane := &kcpv1alpha3.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
		Spec: kcpv1alpha3.KubeadmControlPlaneSpec{
			Replicas: &replicas,
			Version:  "v1.17.4",
			InfrastructureTemplate: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
				Kind:       "AzureMachineTemplate",
				Name:       cluster,
			},
			KubeadmConfigSpec: capbkv1alpha3.KubeadmConfigSpec{
				ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
					APIServer: kubeadmv1beta1.APIServer{
						ControlPlaneComponent: kubeadmv1beta1.ControlPlaneComponent{
							ExtraArgs: map[string]string{
								"cloud-config":   "/etc/kubernetes/azure.json",
								"cloud-provider": "azure",
							},
							ExtraVolumes: []kubeadmv1beta1.HostPathMount{
								{
									HostPath:  "/etc/kubernetes/azure.json",
									MountPath: "/etc/kubernetes/azure.json",
									Name:      "cloud-config",
									ReadOnly:  true,
								},
							},
						},
						TimeoutForControlPlane: &metav1.Duration{
							Duration: time.Minute * 20,
						},
					},
					ControllerManager: kubeadmv1beta1.ControlPlaneComponent{
						ExtraArgs: map[string]string{
							"allocate-node-cidrs": "false",
							"cloud-config":        "/etc/kubernetes/azure.json",
							"cloud-provider":      "azure",
						},
						ExtraVolumes: []kubeadmv1beta1.HostPathMount{
							{
								HostPath:  "/etc/kubernetes/azure.json",
								MountPath: "/etc/kubernetes/azure.json",
								Name:      "cloud-config",
								ReadOnly:  true,
							},
						},
					},
				},
				InitConfiguration: &kubeadmv1beta1.InitConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"cloud-config":   "/etc/kubernetes/azure.json",
							"cloud-provider": "azure",
						},
						Name: "{{ ds.meta_data[\"local_hostname\"] }}",
					},
				},
				JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						KubeletExtraArgs: map[string]string{
							"cloud-config":   "/etc/kubernetes/azure.json",
							"cloud-provider": "azure",
						},
						Name: "{{ ds.meta_data[\"local_hostname\"] }}",
					},
				},
				Files: []capbkv1alpha3.File{
					{
						Owner:       "root:root",
						Path:        "/etc/kubernetes/azure.json",
						Permissions: "0644",
						Content:     data,
					},
				},
				UseExperimentalRetryJoin: true,
			},
		},
	}
	return controlplane, nil
}

func getKubeadmConfigTemplate(cluster, location string, settings map[string]string) (*capbkv1alpha3.KubeadmConfigTemplate, error) {
	data, err := getCloudProviderConfig(cluster, location, settings)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cloud provider config")
	}

	return &capbkv1alpha3.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: cluster,
		},
		Spec: capbkv1alpha3.KubeadmConfigTemplateSpec{
			Template: capbkv1alpha3.KubeadmConfigTemplateResource{
				Spec: capbkv1alpha3.KubeadmConfigSpec{
					Files: []capbkv1alpha3.File{
						{
							Owner:       "root:root",
							Path:        "/etc/kubernetes/azure.json",
							Permissions: "0644",
							Content:     data,
						},
					},
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							KubeletExtraArgs: map[string]string{
								"cloud-config":   "/etc/kubernetes/azure.json",
								"cloud-provider": "azure",
							},
							Name: "{{ ds.meta_data[\"local_hostname\"] }}",
						},
					},
				},
			},
		},
	}, nil
}

// abbreviated version to avoid importing k/k
type CloudProviderConfig struct {
	Cloud                        string `json:"cloud"`
	TenantID                     string `json:"tenantId"`
	SubscriptionID               string `json:"subscriptionId"`
	AadClientID                  string `json:"aadClientId"`
	AadClientSecret              string `json:"aadClientSecret"`
	ResourceGroup                string `json:"resourceGroup"`
	SecurityGroupName            string `json:"securityGroupName"`
	Location                     string `json:"location"`
	VMType                       string `json:"vmType"`
	VnetName                     string `json:"vnetName"`
	VnetResourceGroup            string `json:"vnetResourceGroup"`
	SubnetName                   string `json:"subnetName"`
	RouteTableName               string `json:"routeTableName"`
	LoadBalancerSku              string `json:"loadBalancerSku"`
	MaximumLoadBalancerRuleCount int    `json:"maximumLoadBalancerRuleCount"`
	UseManagedIdentityExtension  bool   `json:"useManagedIdentityExtension"`
	UseInstanceMetadata          bool   `json:"useInstanceMetadata"`
}

func getCloudProviderConfig(cluster, location string, settings map[string]string) (string, error) {
	config := &CloudProviderConfig{
		Cloud:                        settings[auth.EnvironmentName],
		TenantID:                     settings[auth.TenantID],
		SubscriptionID:               settings[auth.SubscriptionID],
		AadClientID:                  settings[auth.ClientID],
		AadClientSecret:              settings[auth.ClientSecret],
		ResourceGroup:                cluster,
		SecurityGroupName:            fmt.Sprintf("%s-node-nsg", cluster),
		Location:                     location,
		VMType:                       "standard",
		VnetName:                     fmt.Sprintf("%s-vnet", cluster),
		VnetResourceGroup:            cluster,
		SubnetName:                   fmt.Sprintf("%s-node-subnet", cluster),
		RouteTableName:               fmt.Sprintf("%s-node-routetable", cluster),
		LoadBalancerSku:              "standard",
		MaximumLoadBalancerRuleCount: 250,
		UseManagedIdentityExtension:  false,
		UseInstanceMetadata:          true,
	}
	b, err := json.Marshal(config)
	return string(b), err
}
