/*
Copyright 2020 Juan-Lee Pang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// nolint: dupl
package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capbkv1alpha3 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	kcpv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
)

// WorkerReconciler reconciles a Worker object
type WorkerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=workers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=workers/status,verbs=get;update;patch

func (r *WorkerReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("worker", req.NamespacedName)

	var worker infrastructurev1alpha1.Worker
	if err := r.Get(ctx, req.NamespacedName, &worker); err != nil {
		log.Error(err, "unable to fetch worker")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	defer func() {
		if err := r.Status().Update(ctx, &worker); err != nil && reterr == nil {
			log.Error(err, "failed to update worker status")
			reterr = err
		}
	}()

	if worker.Status.AvailableCapacity == nil {
		worker.Status.AvailableCapacity = &worker.Spec.Capacity
		worker.Status.LastScheduledTime = metav1.Now()
	}

	// need to handle update to capacity

	worker.Status.Phase = infrastructurev1alpha1.WorkerRunning

	return ctrl.Result{}, nil
}

func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.Worker{}).
		Complete(r)
}

func getAzureSettings() (map[string]string, error) {
	file, fileErr := auth.GetSettingsFromFile()
	if fileErr != nil {
		env, envErr := auth.GetSettingsFromEnvironment()
		if envErr != nil {
			return nil, fmt.Errorf("failed to get settings from file: %s\n\n failed to get settings from environment: %s", fileErr.Error(), envErr.Error())
		}
		return env.Values, nil
	}
	return file.Values, nil
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

var (
	kubeadmConfigTemplate = capbkv1alpha3.KubeadmConfigTemplate{}

	cluster      = capiv1alpha3.Cluster{}
	controlplane = kcpv1alpha3.KubeadmControlPlane{}

	azureCluster                  = capzv1alpha3.AzureCluster{}
	machineDeploymentControlPlane = capiv1alpha3.MachineDeployment{}
	machineDeploymentWorker       = capiv1alpha3.MachineDeployment{}
	machineTemplateControlPlane   = capzv1alpha3.AzureMachineTemplate{}
	machineTemplateWorker         = capzv1alpha3.AzureMachineTemplate{}
)

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
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{},
				},
			},
		},
	}, nil
}
