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

// SchemaSpec defines the desired state of Schema
type SchemaSpec struct {
	// Used to define the schema name, should match a topic name, if the schema should be attached to the topic
	Name string `json:"name"`

	// Used to define the schema target, one of VALUE (default), KEY
	Target string `json:"target,oneOf=KEY,VALUE" default:"VALUE"`

	// Used to define the schema type, one of AVRO (default), PROTOBUF, JSON
	Type string `json:"type,oneOf=AVRO,PROTOBUF,JSON" default:"AVRO"`

	// Used to define the schema content
	Content string `json:"content"`

	// Used to define the compatibility level of the schema, one of NONE (default), BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE
	CompatibilityLevel string `json:"compatibilityLevel,oneOf=NONE,BACKWARD,BACKWARD_TRANSITIVE,FORWARD,FORWARD_TRANSITIVE,FULL,FULL_TRANSITIVE" default:"NONE"`
}

// SchemaStatus defines the observed state of Schema
type SchemaStatus struct {
	// Used to define the latest version of the schema
	LatestVersion int `json:"latestVersion"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Schema is the Schema for the schemas API
type Schema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaSpec   `json:"spec,omitempty"`
	Status SchemaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchemaList contains a list of Schema
type SchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Schema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schema{}, &SchemaList{})
}
