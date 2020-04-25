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

package main

import (
	"flag"
	"os"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	realzap "go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	kubeadmv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capbkv1alpha3 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kcpv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	carpv1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/controllers"
	"github.com/juan-lee/carp/internal/azure"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() { // nolint: gochecknoinits
	_ = setupScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(
		zap.New(
			zap.RawZapOpts(realzap.AddCaller()),
			func(o *zap.Options) {
				o.Development = true
			},
		),
	)
	settings, err := azure.GetSettings()
	if err != nil {
		setupLog.Error(err, "failed to get azure settings")
		os.Exit(1)
	}

	if settings[auth.ClientID] == "" ||
		settings[auth.ClientSecret] == "" ||
		settings[auth.TenantID] == "" ||
		settings[auth.SubscriptionID] == "" {
		secretLen := len(settings[auth.ClientID])
		setupLog.WithValues(
			"app", settings[auth.ClientID],
			"tenant", settings[auth.ClientID],
			"subscription", settings[auth.ClientID],
			"length of secret", secretLen,
		).Error(err, "azure credentials not fully populated")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "4e0d400a.cluster.x-k8s.io",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ManagedClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ManagedCluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedCluster")
		os.Exit(1)
	}
	if err = (&controllers.WorkerReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Worker"),
		Scheme:        mgr.GetScheme(),
		AzureSettings: settings,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Worker")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupScheme(scheme *runtime.Scheme) error {
	schemeFn := []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		capzv1alpha3.AddToScheme,
		capiv1alpha3.AddToScheme,
		capbkv1alpha3.AddToScheme,
		kubeadmv1beta1.AddToScheme,
		kcpv1alpha3.AddToScheme,
		carpv1alpha1.AddToScheme,
	}
	for _, fn := range schemeFn {
		fn := fn
		if err := fn(scheme); err != nil {
			return err
		}
	}
	return nil
}
