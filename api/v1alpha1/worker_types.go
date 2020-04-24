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

type WorkerPhase string

const (
	// WorkerPending means the cluster is in a state where it should not be accepting new control planes,
	// possibly because of some other operation such as creating, updating, or scaling
	WorkerPending WorkerPhase = "Pending"

	// WorkerRunning means the cluster is running and able to host control planes
	WorkerRunning WorkerPhase = "Running"

	// WorkerTermination means the cluster is in the state of termination
	WorkerTerminating WorkerPhase = "Terminating"
)

// WorkerSpec defines the desired state of Worker
type WorkerSpec struct {
	// Version is the version of Kubernetes running on this worker
	// cluster.
	Version string `json:"version"`
	// Location is the Azure region for this cluster.
	Location string `json:"location"`
	// Capacity is the total number of managed control planes that can be scheduled to this cluster
	Capacity int32 `json:"capacity"`
	//	Replicas is the number of worker machines in this worker cluster.
	Replicas int32 `json:"replicas"`
}

// WorkerStatus defines the observed state of Worker
type WorkerStatus struct {
	// Phase is the current lifecycle phase of the worker cluster
	Phase WorkerPhase `json:"phase"`

	// AvailableCapacity is the difference of the total capacity and current capacity for managed control planes
	AvailableCapacity *int32 `json:"availableCapacity,omitempty"`

	// LastScheduledTime is the last time that a managed control plane was scheduled to this cluster
	LastScheduledTime metav1.Time `json:"lastScheduledTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Worker is the Schema for the workers API
type Worker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSpec   `json:"spec,omitempty"`
	Status WorkerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkerList contains a list of Worker
type WorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Worker `json:"items"`
}

func init() { // nolint: gochecknoinits
	SchemeBuilder.Register(&Worker{}, &WorkerList{})
}
