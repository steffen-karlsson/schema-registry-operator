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

	corev1 "k8s.io/api/core/v1"
)

// SchemaRegistrySpec defines the desired state of SchemaRegistry
type SchemaRegistrySpec struct {
	// Used to define the version of the schema registry
	Image ContainerImage `json:"image"`

	// Used to define the number of replicas
	Replicas int32 `json:"replicas,omitempty"`

	// Used to define the compatibility level of the schema registry, one of NONE (default), BACKWARD, BACKWARD_TRANSITIVE, FORWARD, FORWARD_TRANSITIVE, FULL, FULL_TRANSITIVE
	CompatibilityLevel string `json:"compatibilityLevel,omitempty,oneOf=NONE,BACKWARD,BACKWARD_TRANSITIVE,FORWARD,FORWARD_TRANSITIVE,FULL,FULL_TRANSITIVE" default:"NONE"`

	// The desired compute resource requirements of Pods in the cluster.
	// +kubebuilder:default:={limits: {cpu: "2000m", memory: "2Gi"}, requests: {cpu: "1000m", memory: "2Gi"}}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Used to define the ingress specifications of the schema registry
	Ingress SchemaRegistryIngress `json:"ingress,omitempty"`

	// Used to define the metrics specifications of the schema registry
	Metrics SchemaRegistryMetrics `json:"metrics,omitempty"`

	// Used to define the Kafka configuration
	KafkaConfig KafkaConfig `json:"kafkaConfig,omitempty"`

	// Used to define the debug mode
	Debug bool `json:"debug,omitempty"`
}

type ContainerImage struct {
	// Used to define the version of the schema registry
	Version string `json:"tag,omitempty"`

	// Used to define the repository where the image is stored
	Repository string `json:"repository,omitempty"`

	// Used to define the pull policy
	PullPolicy *corev1.PullPolicy `json:"pullPolicy,omitempty"`
}

// SchemaRegistryIngress defines the desired state of the ingress
type SchemaRegistryIngress struct {
	// Used to define if the ingress is enabled
	Enabled bool `json:"enabled,omitempty"`

	// Used to define the host
	Host string `json:"host,omitempty"`
}

// SchemaRegistryMetrics defines the desired state of the metrics
type SchemaRegistryMetrics struct {
	// Used to define if the metrics are enabled
	Enabled bool `json:"enabled,omitempty"`

	// Used to define the port
	Port int32 `json:"port,omitempty"`
}

// KafkaConfig defines the desired state of the Kafka configuration
type KafkaConfig struct {
	// Used to define the Kafka bootstrap servers
	BootstrapServers []string `json:"bootstrapServers,omitempty"`

	// Used to define the Kafka authentication
	Authentication KafkaConfigAuthentication `json:"authentication,omitempty"`
}

// KafkaConfigAuthentication defines the desired state of the Kafka authentication
type KafkaConfigAuthentication struct {
	// Used to define the type of authentication
	SaslJaasConfig ValueFrom `json:"saslJaasConfig,omitempty"`
}

// type ValueFrom defines the desired state of the value from
type ValueFrom struct {
	// Used to define the value from the field
	Source *corev1.EnvVarSource `json:"valueFrom,omitempty"`
}

// SchemaRegistryStatus defines the observed state of SchemaRegistry
type SchemaRegistryStatus struct {
	// Used to define the status message of the schema registry
	Message string `json:"message"`

	// Used to define if the schema registry is ready
	Ready bool `json:"ready"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.image.version",description="The version of the schema registry"
// +kubebuilder:printcolumn:name="Compatibility Level",type="string",JSONPath=".spec.compatibilityLevel",description="The compatibility level of the schema registry"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas",description="The number of Coherence Pods for this role"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="The readiness of the schema registry"

// SchemaRegistry is the Schema for the schemaregistries API
type SchemaRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaRegistrySpec   `json:"spec,omitempty"`
	Status SchemaRegistryStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchemaRegistryList contains a list of SchemaRegistry
type SchemaRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SchemaRegistry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SchemaRegistry{}, &SchemaRegistryList{})
}
