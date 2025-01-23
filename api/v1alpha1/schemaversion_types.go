/*
MIT License

Copyright (c) 2025 Steffen Karlsson

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchemaVersionSpec defines the desired state of SchemaVersion
type SchemaVersionSpec struct {
	// Used to define the schema subject, concatenated with the schema name and target
	Subject string `json:"subject"`

	// Used to define the schema content
	Content string `json:"content"`

	// Used to define the schema version
	Version int32 `json:"version"`
}

// SchemaVersionStatus defines the observed state of SchemaVersion
type SchemaVersionStatus struct {
	// Used to define whether the schema is applied to the schema registry
	Ready bool `json:"ready"`

	// Used to define whether the schema is active, i.e. the latest version
	Active bool `json:"active"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// SchemaVersion is the Schema for the schemaversions API
type SchemaVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaVersionSpec   `json:"spec,omitempty"`
	Status SchemaVersionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Subject",type="string",JSONPath=".spec.subject",description="The schema subject"
// +kubebuilder:printcolumn:name="Version",type="integer",JSONPath=".spec.version",description="The schema version"
// +kubebuilder:printcolumn:name="Active",type="boolean",JSONPath=".status.active",description="Whether the schema is active"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Whether the schema is ready"

// SchemaVersionList contains a list of SchemaVersion
type SchemaVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchemaVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchemaVersion{}, &SchemaVersionList{})
}
