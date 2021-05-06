/*
Copyright 2021 amsy810.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CertCheckSpec defines the desired state of CertCheck
type CertCheckSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Selector  *metav1.LabelSelector `json:"selector,omitempty"`
	Threshold int                   `json:"threshold,omitempty"`
}

// CertCheckStatus defines the observed state of CertCheck
type CertCheckStatus struct {
	TargetCertsCount int           `json:"targetCertsCount,omitempty"`
	Certificates     []Certificate `json:"certificates,omitempty"`
}

type Certificate struct {
	Name      string      `json:"name,omitempty"`
	NotBefore metav1.Time `json:"notBefore,omitempty"`
	NotAfter  metav1.Time `json:"notAfter,omitempty"`
	Active    bool        `json:"active,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CertCheck is the Schema for the certchecks API
type CertCheck struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertCheckSpec   `json:"spec,omitempty"`
	Status CertCheckStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CertCheckList contains a list of CertCheck
type CertCheckList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CertCheck `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CertCheck{}, &CertCheckList{})
}
