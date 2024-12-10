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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EstIssuerSpec defines the desired state of EstIssuer
type EstIssuerSpec struct {
	// DNS name of the portal.
	// +kubebuilder:validation:Required
	Hostname string `json:"hostname"`

	// Port number of the portal
	// +kubebuilder:validation:Required
	Port int `json:"port"`

	// Interface label as described in RFC 7030 Sec. 3.2.2. Labels are added to the “well-known” path to enable one portal to support multiple issuers.
	// +kubebuilder:validation:Optional
	Label string `json:"label,omitempty"`

	// /.well-known/est
	// +kubebuilder:validation:Optional
	WellKnown string `json:"wellKnown,omitempty"`

	// The root certificate the portal issues under. The certificate must be in PEM encoding, and then base64 encoded
	// +kubebuilder:validation:Required
	Cacert string `json:"cacert"`

	// The name of a Secret holding the EST Portal credential. est-operator supports HTTP Basic Authentication for initial enrollment.
	// +kubebuilder:validation:Required
	AuthSecretName string `json:"authSecretName"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EstIssuer is the Schema for the estissuers API
type EstIssuer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EstIssuerSpec   `json:"spec,omitempty"`
	Status EstIssuerStatus `json:"status,omitempty"`
}

type EstIssuerStatus struct {
	// +kubebuilder:validation:Optional
	Ready bool `json:"ready,omitempty"`

	// https://github.com/kubernetes-sigs/cli-utils/blob/master/pkg/kstatus/README.md
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

//+kubebuilder:object:root=true

// EstIssuerList contains a list of EstIssuer
type EstIssuerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EstIssuer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EstIssuer{}, &EstIssuerList{})
}
