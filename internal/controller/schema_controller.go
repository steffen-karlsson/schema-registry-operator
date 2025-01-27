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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
	k8s_manager "github.com/steffen-karlsson/schema-registry-operator/pkg/k8s"
	"github.com/steffen-karlsson/schema-registry-operator/pkg/srclient"
)

const (
	SchemaVersionLatest   = "latest"
	SchemaDeployedSuccess = "Schema deployed successfully"
)

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	k8s_manager.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the Schema object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SchemaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// The purpose is checking if the Custom Resource for the Kind Schema
	// is applied on the cluster if not we return nil to stop the reconciliation
	schema := &clientv1alpha1.Schema{}
	err := r.Get(ctx, req.NamespacedName, schema)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			logger.Info("schema resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}

		// If the error is not NotFound then it means that there was an error while trying to get the resource
		// In this way, we will requeue the request
		logger.Error(err, "failed to get schema")
		return ctrl.Result{}, err
	}

	// The purpose is to check if the schema content has changed
	updated, err := Updated(schema.ObjectMeta, schema)
	if err != nil {
		logger.Error(err, "failed to check if schema content has changed")
		return ctrl.Result{}, err
	}

	if !updated && schema.Status.Ready {
		// No need to update the schema if the content hash is the same
		schema.UpdateStatus(true, SchemaDeployedSuccess)

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if schema.Spec.Subject == "" {
		schema.Spec.Subject = schema.Name
	}

	// The purpose is to get the SchemaRegistry instance
	schemaRegistry, retry, err := FetchSchemaRegistryInstance(ctx, r, schema.ObjectMeta, schema)
	if err != nil {
		logger.Error(err, "failed to get schema registry instance")

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		if retry {
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		return ctrl.Result{}, err
	}

	srSchemaObject, err := r.deploySchema(ctx, schema, schemaRegistry, logger)
	if err != nil {
		logger.Error(err, "failed to deploy schema to schema registry", "schema", schema)

		if errors.Is(err, ErrIncompatibleSchema) || errors.Is(err, ErrInvalidSchemaOrType) {
			schema.Status.SchemaRegistryError = errors.Unwrap(err).Error()
		}

		schema.UpdateStatus(false, "Failed to deploy schema to Schema Registry: "+schemaRegistry.Name)

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	version := int(*srSchemaObject.Version)

	if err = r.deploySchemaVersion(ctx, schema, srSchemaObject, schemaRegistry.Name, logger); err != nil {
		logger.Error(err, "failed to create new SchemaVersion CRD", "schema", schema)

		schema.UpdateStatus(false, "Failed to create new SchemaVersion CRD with version: "+strconv.Itoa(version))

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// Get the new SchemaVersion CRD and update status to active
	newSchemaVersion := &clientv1alpha1.SchemaVersion{}
	if err = r.Get(ctx, types.NamespacedName{
		Namespace: schema.Namespace,
		Name:      schema.GetSubject() + "-v" + strconv.Itoa(version),
	}, newSchemaVersion); err != nil {
		logger.Error(err, "failed to get new schema version")
		return ctrl.Result{}, err
	}

	newSchemaVersion.Status.Active = true
	newSchemaVersion.Status.Ready = true

	if err = r.Status().Update(ctx, newSchemaVersion); err != nil {
		logger.Error(err, "failed to update new schema version status")
		return ctrl.Result{}, err
	}

	// Update status of previous SchemaVersion CRD to inactive
	oldSchemaVersion := &clientv1alpha1.SchemaVersion{}
	if schema.Status.LatestVersion != 0 {
		if err = r.Get(ctx, types.NamespacedName{
			Namespace: schema.Namespace,
			Name:      schema.GetSubject() + "-v" + strconv.Itoa(schema.Status.LatestVersion),
		}, oldSchemaVersion); err != nil {
			logger.Error(err, "failed to get previous active schema version")
			return ctrl.Result{}, err
		}

		oldSchemaVersion.Status.Active = false
		oldSchemaVersion.Status.Ready = true

		if err = r.Status().Update(ctx, oldSchemaVersion); err != nil {
			logger.Error(err, "failed to update previous active schema version status")
			return ctrl.Result{}, err
		}
	}

	newContentHash, err := schema.Hash()
	if err != nil {
		logger.Error(err, "failed to hash schema content")
		return ctrl.Result{}, err
	}

	schema.ObjectMeta.Labels[SchemaRegistryContentHash] = strconv.Itoa(int(newContentHash))

	if err = r.Update(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema content hash")
		return ctrl.Result{}, err
	}

	schema.UpdateStatus(true, SchemaDeployedSuccess)
	schema.Status.LatestVersion = version

	if err = r.Status().Update(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *SchemaReconciler) deploySchemaVersion(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	srSchemaObject *srclient.Schema,
	schemaRegistryName string,
	logger logr.Logger,
) error {
	schemaVersion := r.createSchemaVersion(schema, srSchemaObject, schemaRegistryName)
	if err := ctrl.SetControllerReference(schema, &schemaVersion, r.Scheme); err != nil {
		logger.Error(err, "failed to set controller reference", "schemaversion", schemaVersion)
		return err
	}

	if err := r.Upsert(ctx, &schemaVersion, false); err != nil {
		logger.Error(err, "failed to create schemaversion", "schemaversion", schemaVersion)
		return err
	}

	return nil
}

func (r *SchemaReconciler) deploySchema(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (*srclient.Schema, error) {
	server := fmt.Sprintf("http://%s:%d", schemaRegistry.Name, schemaRegistry.Spec.Port)
	srClient, err := srclient.NewClientWithResponses(server)
	if err != nil {
		logger.Error(err, "failed to create schema registry client")
		return nil, err
	}

	registerResp, err := srClient.Register1WithResponse(ctx, schema.GetSubject(), &srclient.Register1Params{
		Normalize: &schema.Spec.Normalize,
	}, srclient.Register1JSONRequestBody{
		Schema:     &schema.Spec.Content,
		SchemaType: &schema.Spec.Type,
	})

	if err != nil {
		logger.Error(err, "failed to register schema")
		return nil, err
	}

	switch registerResp.HTTPResponse.StatusCode {
	case http.StatusUnprocessableEntity:
		return nil, NewInvalidSchemaOrTypeError(*registerResp.ApplicationvndSchemaregistryV1JSON422.Message)
	case http.StatusConflict:
		return nil, NewIncompatibleSchemaError(*registerResp.ApplicationvndSchemaregistryV1JSON409.Message)
	}

	getResp, err := srClient.GetSchemaByVersion1WithResponse(ctx, schema.GetSubject(), SchemaVersionLatest, nil)
	if err != nil || getResp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unknown error, failed to get schema: %w", err)
	}

	return getResp.ApplicationvndSchemaregistryV1JSON200, nil
}

func (r *SchemaReconciler) createSchemaVersion(
	schema *clientv1alpha1.Schema,
	srSchemaObject *srclient.Schema,
	schemaRegistryName string,
) clientv1alpha1.SchemaVersion {
	version := int(*srSchemaObject.Version)
	return clientv1alpha1.SchemaVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      schema.GetSubject() + "-v" + strconv.Itoa(version),
			Namespace: schema.Namespace,
			Annotations: map[string]string{
				PreviousActiveSchemaVersionAnnotationName: strconv.Itoa(schema.Status.LatestVersion),
			},
			Labels: map[string]string{
				SchemaRegistryLabelName: schemaRegistryName,
			},
		},
		Spec: clientv1alpha1.SchemaVersionSpec{
			Subject:                schema.GetSubject(),
			Version:                version,
			Content:                schema.Spec.Content,
			SchemaRegistrySchemaId: int(*srSchemaObject.Id),
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.Schema{}).
		Complete(r)
}
