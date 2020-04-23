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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ManagedClusterPhase string

const (
	// ManagedClusterPending means the cluster is in a pending state
	ManagedClusterPending ManagedClusterPhase = "Pending"

	// ManagedClusterRunning means the cluster is running
	ManagedClusterRunning ManagedClusterPhase = "Running"

	// ManagedClusterTermination means the cluster is in the state of termination
	ManagedClusterTerminating ManagedClusterPhase = "Terminating"
)

// ManagedClusterSpec defines the desired state of ManagedCluster
type ManagedClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of ManagedCluster. Edit ManagedCluster_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// ManagedClusterStatus defines the observed state of ManagedCluster
type ManagedClusterStatus struct {
	// Phase is the current lifecycle phase of the managed cluster
	Phase ManagedClusterPhase `json:"phase"`

	// AssignedWorker is the unique identifier of the worker to which the cluster has been assigned
	AssignedWorker *string `json:"assignedWorker,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ManagedCluster is the Schema for the managedclusters API
type ManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedClusterSpec   `json:"spec,omitempty"`
	Status ManagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedClusterList contains a list of ManagedCluster
type ManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedCluster `json:"items"`
}

func init() { // nolint: gochecknoinits
	SchemeBuilder.Register(&ManagedCluster{}, &ManagedClusterList{})
}
