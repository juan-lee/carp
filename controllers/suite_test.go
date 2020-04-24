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

package controllers

import (
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	kubeadmv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	capzv1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capbkv1alpha3 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kcpv1alpha3 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	carpv1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/pkg/azure"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	log       logr.Logger
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	rand.Seed(time.Now().Unix())
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	settings, err := azure.GetSettings()
	Expect(err).NotTo(HaveOccurred())

	log = logf.Log.WithName("carp-test")
	log.WithValues("subscription", settings[auth.SubscriptionID], "app", settings[auth.ClientID], "tenant", settings[auth.TenantID]).Info("using client configuration")

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	By("Starting test env")
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("setting up client scheme")
	err = setupScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	By("setting up a new manager")
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	By("fetching client")
	k8sClient = mgr.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	Expect((&ManagedClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("ManagedCluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr)).NotTo(HaveOccurred())

	Expect((&WorkerReconciler{
		Client:        mgr.GetClient(),
		Log:           ctrl.Log.WithName("controllers").WithName("Worker"),
		Scheme:        mgr.GetScheme(),
		AzureSettings: settings,
	}).SetupWithManager(mgr)).NotTo(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func setupScheme(scheme *runtime.Scheme) error {
	schemeFn := []func(*runtime.Scheme) error{
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
