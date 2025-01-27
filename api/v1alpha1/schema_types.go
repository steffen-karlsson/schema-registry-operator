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
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/steffen-karlsson/schema-registry-operator/pkg/hash"
)

// SchemaSpec defines the desired state of Schema
type SchemaSpec struct {
	// +kubebuilder:default:="VALUE"
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Target is immutable"
	// Used to define the schema target, one of VALUE (default), KEY
	Target string `json:"target,oneOf=KEY,VALUE" default:"VALUE"`

	// +kubebuilder:default:="AVRO"
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Type is immutable"
	// Used to define the schema type, one of AVRO (default), PROTOBUF, JSON
	Type string `json:"type,oneOf=AVRO,PROTOBUF,JSON" default:"AVRO"`

	// Used to define the schema content
	Content string `json:"content"`

	// +kubebuilder:default:="NONE"
	// +kubebuilder:validation:Optional
	// Used to define the compatibility level of the schema, one of NONE (default), BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE
	CompatibilityLevel string `json:"compatibilityLevel,oneOf=NONE,BACKWARD,BACKWARD_TRANSITIVE,FORWARD,FORWARD_TRANSITIVE,FULL,FULL_TRANSITIVE" default:"NONE"`

	// +kubebuilder:default:=false
	// +kubebuilder:validation:Optional
	// Used to define if the schema should be normalized, default is false
	Normalize bool `json:"normalize" default:"false"`
}

// SchemaStatus defines the observed state of Schema
type SchemaStatus struct {
	// Used to define the latest version of the schema
	LatestVersion int `json:"latestVersion"`

	// Used to define the status message of the schema
	Message string `json:"message"`

	// Used to define the schema registry error
	SchemaRegistryError string `json:"schemaRegistryError"`

	// Used to define if the schema is ready
	Ready bool `json:"ready"`

	// Used to define the last transition time
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.target",description="The target of the schema"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="The type of the schema"
// +kubebuilder:printcolumn:name="Compatibility Level",type="string",JSONPath=".spec.compatibilityLevel",description="The compatibility level of the schema"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="The readiness of the schema"

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

func (s *Schema) Hash() (uint32, error) {
	return hash.Hash(s.Spec.Content)
}

func (s *Schema) GetSubject() string {
	return s.Name + "-" + strings.ToLower(s.Spec.Target)
}
