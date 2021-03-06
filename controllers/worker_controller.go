/*
Copyright 2020 Juan-Lee Pang.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

// nolint: dupl
package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capbkv1alpha3 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kcpv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/internal/remote"
)

// WorkerReconciler reconciles a Worker object
type WorkerReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	AzureSettings map[string]string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=workers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=workers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=azureclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io;bootstrap.cluster.x-k8s.io;controlplane.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=kubeadmconfigs;kubeadmconfigs/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments;machinedeployments/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;patch

func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.Worker{}).
		Owns(&capiv1alpha3.Cluster{}).
		Owns(&kcpv1alpha3.KubeadmControlPlane{}).
		Owns(&capzv1alpha3.AzureCluster{}).
		Owns(&capbkv1alpha3.KubeadmConfigTemplate{}).
		Owns(&capiv1alpha3.MachineDeployment{}).
		Owns(&capzv1alpha3.AzureMachineTemplate{}).
		Complete(r)
}

func (r *WorkerReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("worker", req.NamespacedName)

	var worker infrastructurev1alpha1.Worker
	if err := r.Get(ctx, req.NamespacedName, &worker); err != nil {
		log.Error(err, "unable to fetch worker")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	reconcilers := []func(context.Context, *infrastructurev1alpha1.Worker) error{
		r.reconcileCluster,
		r.reconcileKubeadmConfigTemplate,
		r.reconcileKubeadmControlPlane,
		r.reconcileMachineTemplate,
		r.reconcileMachineDeployment,
		r.reconcileAzureCluster,
		r.reconcileExternal,
	}

	for _, reconcileFn := range reconcilers {
		reconcileFn := reconcileFn
		if err := reconcileFn(ctx, &worker); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to execute reconcile function: %w", err)
		}
	}

	worker.Status.Phase = infrastructurev1alpha1.WorkerPending

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

func (r *WorkerReconciler) reconcileKubeadmControlPlane(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template, err := getKubeadmControlPlane(worker.Name, worker.Spec.Location, r.AzureSettings)
	if err != nil {
		return fmt.Errorf("failed to get azure settings: %w", err)
	}

	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update kubeadm control plane: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileKubeadmConfigTemplate(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template, err := getKubeadmConfigTemplate(worker.Name, worker.Spec.Location, r.AzureSettings)
	if err != nil {
		return fmt.Errorf("failed to get azure settings: %w", err)
	}

	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update kubeadm config template: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileMachineTemplate(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template := getMachineTemplate(worker.Name, worker.Spec.Location)
	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update machine template: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileMachineDeployment(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template := getMachineDeployment(worker)
	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update machine deployment: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileCluster(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template := getCluster(worker.Name, worker.Spec.Location, r.AzureSettings)
	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update cluster: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileAzureCluster(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template := getAzureCluster(worker.Name, worker.Spec.Location)
	template.Namespace = worker.Namespace

	// TODO(ace): Verify -- I believe this is necessary because CreateOrUpdate does a get
	// into the object it receives, so we need to save a copy and capture it
	// into the closure context.
	want := template.DeepCopy()

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		if err := controllerutil.SetControllerReference(worker, want, r.Scheme); err != nil {
			return err
		}
		template = want
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update azure cluster: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileExternal(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	// TODO(ace): don't hardcode
	azureSecret := &corev1.Secret{}
	azureKey := types.NamespacedName{
		Name:      "capz-manager-bootstrap-credentials",
		Namespace: "capz-system",
	}

	// Fetch azure manager credentials to transfer to remote cluster
	if err := r.Get(ctx, azureKey, azureSecret); err != nil {
		return fmt.Errorf("failed to get azure manager secret to apply to cluster: %w", err)
	}

	// Fetch remove kubeconfig
	kubeconfigSecret := &corev1.Secret{}
	kubeconfigKey := types.NamespacedName{
		Name:      fmt.Sprintf("%s-kubeconfig", worker.Name),
		Namespace: worker.Namespace,
	}

	if err := r.Get(ctx, kubeconfigKey, kubeconfigSecret); err != nil {
		return fmt.Errorf("failed to get remote kubeconfig to apply to cluster: %w", err)
	}

	data, ok := kubeconfigSecret.Data[secret.KubeconfigDataName]
	if !ok {
		return fmt.Errorf("missing key %q in secret data", secret.KubeconfigDataName)
	}

	// Construct a kubeclient with it
	remoteClient, err := remote.NewClient(data)
	if err != nil {
		return fmt.Errorf("failed to create REST configuration for worker %s/%s : %w", worker.Namespace, worker.Name, err)
	}

	// Ensure existence of remote namespace
	remoteNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: azureKey.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, remoteClient, remoteNamespace, func() error {
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create remote azure manager namespace")
	}

	// Create fresh copy to avoid copying stuff like UID, resourceVersion
	remoteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureKey.Name,
			Namespace: azureKey.Namespace,
		},
		Data: azureSecret.Data,
	}
	want := remoteSecret.DeepCopy()
	_, err = controllerutil.CreateOrUpdate(ctx, remoteClient, remoteSecret, func() error {
		remoteSecret = want
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create remote azure manager secret")
	}

	_, _, err = remoteClient.Apply("https://raw.githubusercontent.com/juan-lee/cluster-api-provider-azure/hackathon/templates/addons/calico.yaml")

	if err != nil {
		return fmt.Errorf("failed to apply calico config: %w", err)
	}

	return nil
}
