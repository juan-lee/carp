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
	"sync"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
)

var mux sync.Mutex

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=managedclusters/status,verbs=get;update;patch

func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.ManagedCluster{}).
		Complete(r)
}

func (r *ManagedClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcluster", req.NamespacedName)

	var mc infrastructurev1alpha1.ManagedCluster
	if err := r.Get(ctx, req.NamespacedName, &mc); err != nil {
		log.Error(err, "unable to fetch managed cluster")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !mc.ObjectMeta.DeletionTimestamp.IsZero() {
		// get worker and increase available capacity
		if err := r.unassignWorker(ctx, &mc); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	mc.Status.Phase = infrastructurev1alpha1.ManagedClusterPending

	defer func() {
		if err := r.Status().Update(ctx, &mc); err != nil && reterr == nil {
			log.Error(err, "failed to update managed cluster status")
			reterr = err
		}
	}()

	if err := r.assignWorker(ctx, &mc); err != nil {
		log.Error(err, "failed to assign worker")
		return ctrl.Result{}, err
	}

	mc.Status.Phase = infrastructurev1alpha1.ManagedClusterRunning

	return ctrl.Result{}, nil
}

func (r *ManagedClusterReconciler) assignWorker(ctx context.Context, mc *infrastructurev1alpha1.ManagedCluster) error {
	mux.Lock()
	defer mux.Unlock()

	if mc.Status.AssignedWorker == nil {
		var workerList infrastructurev1alpha1.WorkerList
		if err := r.List(ctx, &workerList); err != nil {
			return fmt.Errorf("unable to list workers: %+v", err)
		}

		if len(workerList.Items) == 0 {
			return fmt.Errorf("0 workers found")
		}

		selectedWorker := &workerList.Items[0]
		for i := range workerList.Items {
			if validWorker(&workerList.Items[i], &selectedWorker.Status.LastScheduledTime) {
				selectedWorker = &workerList.Items[i]
			}
		}
		if selectedWorker.Status.Phase != infrastructurev1alpha1.WorkerRunning || *selectedWorker.Status.AvailableCapacity == 0 {
			return fmt.Errorf("0 workers found with available capacity")
		}

		mc.Status.AssignedWorker = &selectedWorker.Name
		*selectedWorker.Status.AvailableCapacity--
		selectedWorker.Status.LastScheduledTime = metav1.Now()
		if err := r.Status().Update(ctx, selectedWorker); err != nil {
			return fmt.Errorf("unable to update selected worker status: %+v", err)
		}
	}

	return nil
}

func (r *ManagedClusterReconciler) unassignWorker(ctx context.Context, mc *infrastructurev1alpha1.ManagedCluster) error {
	mux.Lock()
	defer mux.Unlock()

	if mc.Status.AssignedWorker != nil {
		var worker infrastructurev1alpha1.Worker
		// assuming default namespace just for the moment
		if err := r.Get(ctx, types.NamespacedName{Namespace: "default", Name: *mc.Status.AssignedWorker}, &worker); err != nil {
			return err
		}

		mc.Status.AssignedWorker = nil
		*worker.Status.AvailableCapacity++
		if err := r.Status().Update(ctx, &worker); err != nil {
			return fmt.Errorf("unable to update selected worker status: %+v", err)
		}
	}

	return nil
}

func validWorker(worker *infrastructurev1alpha1.Worker, minLastScheduledTime *metav1.Time) bool {
	return worker.Status.Phase == infrastructurev1alpha1.WorkerRunning && *worker.Status.AvailableCapacity > 0 &&
		worker.Status.LastScheduledTime.Before(minLastScheduledTime)
}
