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
	"github.com/cert-manager/issuer-lib/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EstIssuerSpec defines the desired state of EstIssuer
type EstIssuerSpec struct {
	Hostname string `json:"hostname"`

	Port string `json:"port"`

	Cacert string `json:"cacert"`

	AuthSecretName string `json:"authSecretName"`
}

func (vi *EstIssuer) GetStatus() *v1alpha1.IssuerStatus {
	return &vi.Status
}

// GetIssuerTypeIdentifier returns a string that uniquely identifies the
// issuer type. This should be a constant across all instances of this
// issuer type. This string is used as a prefix when determining the
// issuer type for a Kubernetes CertificateSigningRequest resource based
// on the issuerName field. The value should be formatted as follows:
// "<issuer resource (plural)>.<issuer group>". For example, the value
// "simpleclusterissuers.issuer.cert-manager.io" will match all CSRs
// with an issuerName set to eg. "simpleclusterissuers.issuer.cert-manager.io/issuer1".
func (vi *EstIssuer) GetIssuerTypeIdentifier() string {
	// ACTION REQUIRED: Change this to a unique string that identifies your issuer
	return "estissuers.certmanager.jquad.rocks"
}

// issuer-lib requires that we implement the Issuer interface
// so that it can interact with our Issuer resource.
var _ v1alpha1.Issuer = &EstIssuer{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EstIssuer is the Schema for the estissuers API
type EstIssuer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EstIssuerSpec         `json:"spec,omitempty"`
	Status v1alpha1.IssuerStatus `json:"status,omitempty"`
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
