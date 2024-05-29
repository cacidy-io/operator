package v1alpha1

type PipelineModuleArg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PipelineModule struct {
	// Function is the name of the function that the runner will call
	// from the module
	Function string `json:"function"`

	// SourceAs is the source code path argument for the module
	// function call.
	// +kubebuilder:default:=source
	// +kubebuilder:validation:Optional
	SourceAs string `json:"sourceAs,omitempty"`

	// Args is the name of a secret that contains the function
	// arguments
	// +kubebuilder:validation:Optional
	Args []PipelineModuleArg `json:"arguments,omitempty"`

	// Repository is the http(s) url of the git repository
	Repository string `json:"repository"`

	// Revision is the commit sha of the repository branch
	Revision string `json:"revision"`

	// CloudTokenSecret is the token for pipeline observability using
	// dagger cloud.
	// +kubebuilder:validation:Optional
	CloudTokenSecret string `json:"cloudToken,omitempty"`
}

type Application struct {
	// Repository is the http(s) url of the git repository
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Repository is immutable"
	Repository string `json:"repository"`

	// Branch is the repository branch that contains the pipeline
	// module
	// +kubebuilder:default:=master
	// +kubebuilder:validation:Optional
	Branch string `json:"branch"`

	// Secrets contains the application secrets
	// +kubebuilder:validation:Optional
	Secrets string `json:"secrets,omitempty"`
}
