/*
Copyright 2025.

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

type KafkaConfigAuthentication struct {
	// Used to define the type of authentication
	SaslJaasConfig *corev1.EnvVarSource `json:"saslJaasConfig,omitempty"`
}

// SchemaRegistryStatus defines the observed state of SchemaRegistry
type SchemaRegistryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

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
