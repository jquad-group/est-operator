package v1

type IssuerRef struct {
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// +kubebuilder:validation:Required
	Group string `json:"group"`

	// +kubebuilder:validation:Required
	Name string `json:"name"`
}
