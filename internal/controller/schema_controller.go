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
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	logger.Info("Reconciling Schema: ", "Name", req.Name, "Namespace", req.Namespace)

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

	// The purpose is to get the SchemaRegistry instance
	schemaRegistry, retry, err := FetchSchemaRegistryInstance(ctx, r, schema.ObjectMeta, schema)
	if err != nil {
		logger.Info("failed to get schema registry instance")

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		if retry {
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		return ctrl.Result{}, err
	}

	// The purpose is to check if the schema is scheduled for deletion
	schemaMarkedTobeDeleted := schema.GetDeletionTimestamp() != nil
	if schemaMarkedTobeDeleted {
		if err = r.delete(ctx, schema, schemaRegistry, logger); err != nil {
			logger.Error(err, "failed to delete schema")
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}

		controllerutil.RemoveFinalizer(schema, SchemaFinalizer)
		if err = r.Update(ctx, schema); err != nil {
			logger.Error(err, "failed to remove finalizer from schema")
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
	}

	if schema.Spec.Subject == "" {
		schema.Spec.Subject = schema.Name
	}

	// Check if the <schema.Spec.Subject>-<schema.Spec.Type> is unique in the specific Schema Registry instance
	unique, err := r.isSubjectUnique(ctx, schema, schemaRegistry, logger)
	if err != nil {
		logger.Error(err, "failed to check if subject is unique")
		return ctrl.Result{}, err
	}

	if !unique {
		logger.Info("subject is not unique")
		message := fmt.Sprintf("Subject %s is not unique in Schema Registry %s",
			schema.GetSubject(), schemaRegistry.Name)
		schema.UpdateStatus(false, message)

		if err = r.Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	// All one of events, needs to be checked, otherwise crd Update, will trigger the next reconcile
	// 1) Add finalizer to the schema
	// 2) Assign subject to the schema
	if !controllerutil.ContainsFinalizer(schema, SchemaFinalizer) {
		controllerutil.AddFinalizer(schema, SchemaFinalizer)

		if err = r.Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema")
			return ctrl.Result{}, err
		}
	}

	return r.upsert(ctx, schema, schemaRegistry, logger)
}

func (r *SchemaReconciler) delete(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) error {
	logger.Info("Deleting schema in schema registry", "Name", schema.Name, "Namespace", schema.Namespace)
	srClient, err := schemaRegistry.NewInstance()
	if err != nil {
		logger.Error(err, "failed to create schema registry client")
		return err
	}

	_, err = srClient.DeleteSubject1WithResponse(ctx, schema.GetSubject(), nil)
	if err != nil {
		logger.Error(err, "failed to delete schema")
		return fmt.Errorf("failed to delete schema: %w", err)
	}

	permanentDelete := true
	_, err = srClient.DeleteSubject1WithResponse(ctx, schema.GetSubject(), &srclient.DeleteSubject1Params{
		Permanent: &permanentDelete,
	})
	if err != nil {
		logger.Error(err, "failed to permanently delete schema")
	}

	return nil
}

func (r *SchemaReconciler) upsert(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info("Upserting schema in schema registry", "Name", schema.Name, "Namespace", schema.Namespace)
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

	schema.UpdateStatus(true, SchemaDeployedSuccess)
	schema.Status.LatestVersion = int(*srSchemaObject.Version)

	if err = r.Status().Update(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema status")
		return ctrl.Result{}, err
	}

	secondsTillNextReconcile := time.Duration(schema.Spec.SchemaRegistryConfig.SyncInterval) * time.Second
	return ctrl.Result{RequeueAfter: secondsTillNextReconcile}, nil
}

func (r *SchemaReconciler) isSubjectUnique(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (bool, error) {
	potentialMatchingSchemas := &clientv1alpha1.SchemaList{}
	if err := r.List(ctx, potentialMatchingSchemas,
		client.InNamespace(schema.Namespace),
		client.MatchingLabels{SchemaRegistryLabelName: schemaRegistry.Name}); err != nil {

		logger.Error(err, "failed to list schemas")
		return false, err
	}

	for _, potentialMatchingSchema := range potentialMatchingSchemas.Items {
		if potentialMatchingSchema.GetSubject() == schema.GetSubject() {
			return false, nil
		}
	}

	return true, nil
}

func (r *SchemaReconciler) deploySchema(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (*srclient.Schema, error) {
	srClient, err := schemaRegistry.NewInstance()
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

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.Schema{}).
		Complete(r)
}
