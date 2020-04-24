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
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
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

func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.Worker{}).
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

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
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

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
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

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update machine template: %w", err)
	}

	return nil
}

func (r *WorkerReconciler) reconcileMachineDeployment(ctx context.Context, worker *infrastructurev1alpha1.Worker) error {
	template := getMachineDeployment(worker.Name, worker.Spec.Version, 3)
	template.Namespace = worker.Namespace

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
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

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
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

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, template, func() error {
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update azure cluster: %w", err)
	}

	return nil
}
