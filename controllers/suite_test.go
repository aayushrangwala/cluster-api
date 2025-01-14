/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"os"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/internal/envtest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// +kubebuilder:scaffold:imports
)

const (
	timeout = time.Second * 30
)

var (
	env *envtest.Environment
	ctx = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	fmt.Println("Creating a new test environment")
	env = envtest.New()

	// Set up a ClusterCacheTracker and ClusterCacheReconciler to provide to controllers
	// requiring a connection to a remote cluster
	tracker, err := remote.NewClusterCacheTracker(
		log.Log,
		env.Manager,
	)
	if err != nil {
		panic(fmt.Sprintf("unable to create cluster cache tracker: %v", err))
	}
	if err := (&remote.ClusterCacheReconciler{
		Client:  env,
		Log:     log.Log,
		Tracker: tracker,
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start ClusterCacheReconciler: %v", err))
	}
	if err := (&ClusterReconciler{
		Client:   env,
		recorder: env.GetEventRecorderFor("cluster-controller"),
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start ClusterReconciler: %v", err))
	}
	if err := (&MachineReconciler{
		Client:   env,
		Tracker:  tracker,
		recorder: env.GetEventRecorderFor("machine-controller"),
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start MachineReconciler: %v", err))
	}
	if err := (&MachineSetReconciler{
		Client:   env,
		Tracker:  tracker,
		recorder: env.GetEventRecorderFor("machineset-controller"),
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start MMachineSetReconciler: %v", err))
	}
	if err := (&MachineDeploymentReconciler{
		Client:   env,
		recorder: env.GetEventRecorderFor("machinedeployment-controller"),
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start MMachineDeploymentReconciler: %v", err))
	}
	if err := (&MachineHealthCheckReconciler{
		Client:   env,
		Tracker:  tracker,
		recorder: env.GetEventRecorderFor("machinehealthcheck-controller"),
	}).SetupWithManager(ctx, env.Manager, controller.Options{MaxConcurrentReconciles: 1}); err != nil {
		panic(fmt.Sprintf("Failed to start MachineHealthCheckReconciler : %v", err))
	}

	go func() {
		fmt.Println("Starting the test environment manager")
		if err := env.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-env.Manager.Elected()
	env.WaitForWebhooks()

	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	SetDefaultEventuallyTimeout(timeout)

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := env.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}

func ContainRefOfGroupKind(group, kind string) types.GomegaMatcher {
	return &refGroupKindMatcher{
		kind:  kind,
		group: group,
	}
}

type refGroupKindMatcher struct {
	kind  string
	group string
}

func (matcher *refGroupKindMatcher) Match(actual interface{}) (success bool, err error) {
	ownerRefs, ok := actual.([]metav1.OwnerReference)
	if !ok {
		return false, errors.Errorf("expected []metav1.OwnerReference; got %T", actual)
	}

	for _, ref := range ownerRefs {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return false, nil
		}
		if ref.Kind == matcher.kind && gv.Group == clusterv1.GroupVersion.Group {
			return true, nil
		}
	}

	return false, nil
}

func (matcher *refGroupKindMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %+v to contain refs of Group %s and Kind %s", actual, matcher.group, matcher.kind)
}

func (matcher *refGroupKindMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected %+v not to contain refs of Group %s and Kind %s", actual, matcher.group, matcher.kind)
}
