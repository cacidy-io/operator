package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineJobSpec defines the desired state of PipelineJob
type PipelineJobSpec struct {
	// Module is the pipeline module
	Module PipelineModule `json:"module"`

	// Application is the source application for the pipeline run
	Application Application `json:"application"`

	// Revision is the target commit sha in the application
	// repository
	Revision string `json:"revision"`

	// RetentionPeriodDays is the number of days that pipeline jobs
	RetentionPeriodDays int `json:"retentionPeriodDays"`
}

type pipelineState string

const (
	// PipelineStarting is the state when it is first started
	PipelineStarting pipelineState = "Starting"

	// PipelineRunning is the state when the pipeline task is
	// running
	PipelineRunning pipelineState = "Running"

	// PipelineFailed is the state when the pipeline fails
	PipelineFailed pipelineState = "Failed"

	// PipelineComplete is the state when the pipeline completes
	// successfully
	PipelineComplete pipelineState = "Complete"
)

// PipelineJobStatus defines the observed state of PipelineJob
type PipelineJobStatus struct {
	// State contains the status of the pipeline job
	State pipelineState `json:"state"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name=Revision,type=string,JSONPath=.spec.revision
//+kubebuilder:printcolumn:name=State,type=string,JSONPath=.status.state
//+kubebuilder:printcolumn:name=Age,type=date,JSONPath=.metadata.creationTimestamp

// PipelineJob is the Schema for the pipelinejobs API
type PipelineJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineJobSpec   `json:"spec,omitempty"`
	Status PipelineJobStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PipelineJobList contains a list of PipelineJob
type PipelineJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items
	Items []PipelineJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PipelineJob{}, &PipelineJobList{})
}
