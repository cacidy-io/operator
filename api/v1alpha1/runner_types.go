package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RunnerEngineSettings struct {
	// Debug set the engine log level to debug.
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	Debug bool `json:"debug,omitempty"`

	// InsecureRootCapabilities allows root containers.
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	InsecureRootCapabilities bool `json:"insecureRootCapabilities,omitempty"`

	// StorageEnabled enables the creation of a persistent volume
	// for the engine cache.
	// +kubebuilder:default:=true
	// +kubebuilder:validation:Optional
	StorageEnabled bool `json:"storageEnabled,omitempty"`

	// CacheSize is the size of the engine cache
	// +kubebuilder:default="20Gi"
	// +kubebuilder:validation:Optional
	CacheSize string `json:"cacheSize,omitempty"`

	// MagicacheSecret is the secret for experimental cachin
	// +kubebuilder:validation:Optional
	MagicacheSecret string `json:"magicache,omitempty"`

	// Memory is the pipeline memory size.
	// +kubebuilder:default:="1Gi"
	// +kubebuilder:validation:Optional
	Memory string `json:"memory,omitempty"`
}

// RunnerSpec defines the desired state of Runner
type RunnerSpec struct {
	// Module is the pipeline module
	Module PipelineModule `json:"module"`

	// Application is the source application for the pipeline run
	Application Application `json:"application"`

	// Engine is the pipeline runtime
	// +kubebuilder:default:={}
	// +kubebuilder:validation:Optional
	Engine RunnerEngineSettings `json:"engine,omitempty"`

	// RetentionPeriodDays is the number of days that pipeline jobs
	// will be retained.
	// +kubebuilder:default:=7
	// +kubebuilder:validation:Optional
	RetentionPeriodDays int `json:"retentionPeriodDays,omitempty"`
}

type runnerState string

const (
	// RunnerSynced is the state of the application after a pipeline
	// is created for the latest commit
	RunnerSynced runnerState = "Synced"

	// RunnerOutOfSync is the state of the application when a new
	// commit is detected and a pipeline does not yet exist
	RunnerOutOfSync runnerState = "OutOfSync"

	// RunnerSyncFailed is the state of the application when the
	// latest commit cannot be synced.
	RunnerSyncFailed runnerState = "Failed"
)

// RunnerStatus defines the observed state of Runner
type RunnerStatus struct {
	// Checksum is the last synced source repository commit sha
	Checksum string `json:"checksum"`

	// State contains the sync status of the runner
	State runnerState `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name=Checksum,type=string,JSONPath=.status.checksum
//+kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.state
//+kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp

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
