/*
Copyright 2024.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type RunnerSpecPipelineModule struct {
	// Function is the name of the function that the runner will call from the module
	Function string `json:"function"`

	// Args is the name of a secret that contains the function arguments.
	// +kubebuilder:validation:Optional
	Args string `json:"args,omitempty"`

	// Repository is the http(s) url of the git repository
	Repository string `json:"repository"`

	// Revision is the commit sha of the repository branch
	Revision string `json:"revision"`

	// Repository secret is the name of a secret that contains the repository username and password
	// +kubebuilder:validation:Optional
	RepositorySecret string `json:"repositoryAuth,omitempty"`

	// CloudTokenSecret is the token for pipeline observability using dagger cloud.
	// +kubebuilder:validation:Optional
	CloudTokenSecret string `json:"cloudToken,omitempty"`
}

type RunnerSpecApplication struct {
	// Repository is the http(s) url of the git repository
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Repository is immutable"
	Repository string `json:"repository"`

	// Branch is the repository branch that contains the pipeline module
	// +kubebuilder:default:=master
	// +kubebuilder:validation:Optional
	Branch string `json:"branch"`

	// Repository secret is the name of a secret that contains the repository username and password
	// +kubebuilder:validation:Optional
	RepositorySecret string `json:"repositoryAuth,omitempty"`
}

type RunnerEngineSettings struct {
	// Debug
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	Debug bool `json:"debug"`

	// InsecureRootCapabilities
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	InsecureRootCapabilities bool `json:"insecureRootCapabilities"`

	// StorageEnabled
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	StorageEnabled bool `json:"storageEnabled"`

	// CacheSize
	// +kubebuilder:default="20Gi"
	// +kubebuilder:validation:Optional
	CacheSize string `json:"cacheSize"`

	// MagicacheSecret
	// +kubebuilder:validation:Optional
	MagicacheSecret string `json:"magicache"`

	// Memory
	// +kubebuilder:default:="1Gi"
	// +kubebuilder:validation:Optional
	Memory string `json:"memory"`
}

// RunnerSpec defines the desired state of Runner
type RunnerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Module
	Module RunnerSpecPipelineModule `json:"module"`

	// Application
	Application RunnerSpecApplication `json:"application"`

	// Engine
	// +kubebuilder:default:={}
	// +kubebuilder:validation:Optional
	Engine RunnerEngineSettings `json:"engine"`
}

type runnerStatusType string

const (
	Ready     runnerStatusType = "Ready"
	OutOfSync runnerStatusType = "OutOfSync"
	Running   runnerStatusType = "Running"
	Failed    runnerStatusType = "Failed"
	Complete  runnerStatusType = "Complete"
)

// RunnerStatus defines the observed state of Runner
type RunnerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	State    runnerStatusType `json:"state"`
	Checksum string           `json:"appChecksum"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Runner is the Schema for the runners API
type Runner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RunnerSpec   `json:"spec,omitempty"`
	Status RunnerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RunnerList contains a list of Runner
type RunnerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Runner `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Runner{}, &RunnerList{})
}
