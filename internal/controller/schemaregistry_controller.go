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

package controller

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
)

const (
	SchemaRegistryPodName         = "schema-registry-server"
	SchemaRegistryHttpPort        = 8082
	SchemaRegistryHttpsPort       = 8081
	SchemaRegistryHttpPortName    = "sr-http"
	PrometheusExporterPodName     = "prometheus-jmx-exporter"
	PrometheusExporterPodImage    = "bitnami/jmx-exporter:1.1.0"
	PrometheusConfigMapNameSuffix = "jmx-config"
	JmxConfigMapFileName          = "jmx-schema-registry-prometheus.yml"
	JmxConfigMapContent           = `jmxUrl: service:jmx:rmi:///jndi/rmi://localhost:5555/jmxrmi
lowercaseOutputName: true
lowercaseOutputLabelNames: true
ssl: false`
)

// SchemaRegistryReconciler reconciles a SchemaRegistry object
type SchemaRegistryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Upsert creates or updates the given object in the cluster
func (r *SchemaRegistryReconciler) Upsert(ctx context.Context, obj client.Object, exists bool) error {
	if exists {
		return r.Update(ctx, obj)
	}

	return r.Create(ctx, obj)
}

// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaregistries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaregistries/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaregistries/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the SchemaRegistry object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SchemaRegistryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// The purpose is checking if the Custom Resource for the Kind SchemaRegistry
	// is applied on the cluster if not we return nil to stop the reconciliation
	schemaRegistry := &clientv1alpha1.SchemaRegistry{}
	err := r.Get(ctx, req.NamespacedName, schemaRegistry)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			logger.Info("schema registry resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		// If the error is not NotFound then it means that there was an error while trying to get the resource
		// In this way, we will requeue the request
		logger.Error(err, "failed to get schema registry")
		return ctrl.Result{}, err
	}

	// The purpose is to create a deployment for the SchemaRegistry
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: schemaRegistry.Name, Namespace: schemaRegistry.Namespace}, found)
	logger.Info(req.Name + ":" + req.Namespace)
	if err != nil {
		exists := !apierrors.IsNotFound(err)
		if err = r.deploySchemaRegistry(ctx, schemaRegistry, exists, logger); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *SchemaRegistryReconciler) deploySchemaRegistry(
	ctx context.Context,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	exists bool,
	logger logr.Logger,
) error {
	configMap := r.createSchemaRegistryConfigMap(schemaRegistry)
	if err := ctrl.SetControllerReference(schemaRegistry, configMap, r.Scheme); err != nil {
		logger.Error(err, "failed to set controller reference", "configmap", configMap)
		return err
	}

	if err := r.Upsert(ctx, configMap, exists); err != nil {
		logger.Error(err, "failed to create deployment", "configmap", configMap)
		return err
	}

	deployment := r.createSchemaRegistryDeployment(schemaRegistry)
	if err := ctrl.SetControllerReference(schemaRegistry, deployment, r.Scheme); err != nil {
		logger.Error(err, "failed to set controller reference", "deployment", deployment)
		return err
	}

	if err := r.Upsert(ctx, deployment, exists); err != nil {
		logger.Error(err, "failed to create deployment", "deployment", deployment)
		return err
	}

	service := r.createSchemaRegistryService(schemaRegistry)
	if err := ctrl.SetControllerReference(schemaRegistry, service, r.Scheme); err != nil {
		logger.Error(err, "failed to set controller reference", "service", service)
		return err
	}

	if err := r.Upsert(ctx, service, exists); err != nil {
		logger.Error(err, "failed to create service", "service", service)
		return err
	}

	if schemaRegistry.Spec.Ingress.Enabled {
		ingress := r.createSchemaRegistryIngress(schemaRegistry)
		if err := ctrl.SetControllerReference(schemaRegistry, ingress, r.Scheme); err != nil {
			logger.Error(err, "failed to set controller reference", "ingress", ingress)
			return err
		}

		if err := r.Upsert(ctx, ingress, exists); err != nil {
			logger.Error(err, "failed to create ingress", "ingress", ingress)
			return err
		}
	}

	return nil
}

func (r *SchemaRegistryReconciler) createSchemaRegistryDeployment(sr *clientv1alpha1.SchemaRegistry) *appsv1.Deployment {
	objectMeta := metav1.ObjectMeta{
		Labels: r.getSchemaRegistryLabels(sr),
	}

	envs := []corev1.EnvVar{
		{
			Name: "SCHEMA_REGISTRY_HOST_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.podIP",
				},
			},
		},
		{
			Name:  "SCHEMA_REGISTRY_LISTENERS",
			Value: "http://0.0.0.0:8082",
		},
		{
			Name:  "SCHEMA_REGISTRY_INTER_INSTANCE_PROTOCOL",
			Value: "http",
		},
		{
			Name:  "SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS",
			Value: strings.Join(sr.Spec.KafkaConfig.BootstrapServers, ","),
		},
		{
			Name:  "SCHEMA_REGISTRY_KAFKASTORE_GROUP_ID",
			Value: sr.Name,
		},
		{
			Name:  "SCHEMA_REGISTRY_GROUP_ID",
			Value: sr.Name,
		},
		{
			Name:  "SCHEMA_REGISTRY_MASTER_ELIGIBILITY",
			Value: "true",
		},
		{
			Name:  "SCHEMA_REGISTRY_KAFKASTORE_SECURITY_PROTOCOL",
			Value: "SASL_PLAINTEXT",
		},
		{
			Name:  "SCHEMA_REGISTRY_KAFKASTORE_SASL_MECHANISM",
			Value: "PLAIN",
		},
		{
			Name:      "SCHEMA_REGISTRY_KAFKASTORE_SASL_JAAS_CONFIG",
			ValueFrom: sr.Spec.KafkaConfig.Authentication.SaslJaasConfig.Source,
		},
		{
			Name:  "SCHEMA_REGISTRY_SCHEMA_COMPATIBILITY_LEVEL",
			Value: sr.Spec.CompatibilityLevel,
		},
	}

	if sr.Spec.Debug {
		envs = append(envs, corev1.EnvVar{
			Name:  "SCHEMA_REGISTRY_DEBUG",
			Value: "true",
		})
	}

	containers := []corev1.Container{
		{
			Name:            SchemaRegistryPodName,
			Image:           sr.Spec.Image.Repository + ":" + sr.Spec.Image.Version,
			ImagePullPolicy: *sr.Spec.Image.PullPolicy,
			Ports: []corev1.ContainerPort{
				{
					Name:          SchemaRegistryHttpPortName,
					ContainerPort: SchemaRegistryHttpPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			Resources: *sr.Spec.Resources,
			Env:       envs,
		},
	}

	var volumes []corev1.Volume

	if sr.Spec.Metrics.Enabled {
		objectMeta.Annotations = map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/port":   strconv.Itoa(int(sr.Spec.Metrics.Port)),
		}

		volumes = append(volumes, corev1.Volume{
			Name: PrometheusConfigMapNameSuffix,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: sr.Name + "-" + PrometheusConfigMapNameSuffix,
					},
				},
			},
		})

		containers = append(containers, corev1.Container{
			Name:            PrometheusExporterPodName,
			Image:           PrometheusExporterPodImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				strconv.Itoa(int(sr.Spec.Metrics.Port)),
				"/etc/jmx-exporter/" + JmxConfigMapFileName,
			},
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: sr.Spec.Metrics.Port,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      PrometheusConfigMapNameSuffix,
					MountPath: "/etc/jmx-exporter",
				},
			},
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sr.Name,
			Namespace: sr.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &sr.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: r.getSchemaRegistryLabels(sr),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: objectMeta,
				Spec: corev1.PodSpec{
					Containers: containers,
					Volumes:    volumes,
				},
			},
		},
	}
}

func (r *SchemaRegistryReconciler) createSchemaRegistryService(sr *clientv1alpha1.SchemaRegistry) *corev1.Service {
	ports := []corev1.ServicePort{
		{
			Name: SchemaRegistryHttpPortName,
			Port: 8082,
		},
	}

	if sr.Spec.Metrics.Enabled {
		ports = append(ports, corev1.ServicePort{
			Name: "metrics",
			Port: sr.Spec.Metrics.Port,
		})
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sr.Name,
			Namespace: sr.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: r.getSchemaRegistryLabels(sr),
			Ports:    ports,
		},
	}
}

func (r *SchemaRegistryReconciler) createSchemaRegistryIngress(sr *clientv1alpha1.SchemaRegistry) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sr.Name,
			Namespace: sr.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: sr.Spec.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: sr.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: SchemaRegistryHttpsPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *SchemaRegistryReconciler) createSchemaRegistryConfigMap(sr *clientv1alpha1.SchemaRegistry) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sr.Name + "-" + PrometheusConfigMapNameSuffix,
			Namespace: sr.Namespace,
		},
		Data: map[string]string{
			JmxConfigMapFileName: JmxConfigMapContent,
		},
	}
}

func (r *SchemaRegistryReconciler) getSchemaRegistryLabels(sr *clientv1alpha1.SchemaRegistry) map[string]string {
	return map[string]string{
		"app": sr.Name,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.SchemaRegistry{}).
		Complete(r)
}
