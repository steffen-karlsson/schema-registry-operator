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
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
	k8s_manager "github.com/steffen-karlsson/schema-registry-operator/pkg/k8s"
)

const (
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
	schemaRegistry := &clientv1alpha1.SchemaRegistry{}
	err = schemaRegistry.NewInstance(ctx, r, schema.ObjectMeta, schema)
	switch {
	case errors.Is(err, clientv1alpha1.ErrInstanceLabelNotFound) || errors.Is(err, clientv1alpha1.ErrInstanceNotFound):
		logger.Info("schema registry instance not found")

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Minute}, nil
	case err != nil:
		logger.Error(err, "failed to get schema registry instance")
		return ctrl.Result{}, err
	}

	schemaMarkedToBeDeleted := schema.GetDeletionTimestamp() != nil
	if schemaMarkedToBeDeleted {
		return r.DeleteReconciler(ctx, schema, schemaRegistry, logger)
	}

	isNewSchemaObject := !controllerutil.ContainsFinalizer(schema, SchemaFinalizer)
	if isNewSchemaObject {
		return r.CreateReconciler(ctx, schema, schemaRegistry, logger)
	}

	return r.UpdateReconciler(ctx, schema, schemaRegistry, logger)

}

func (r *SchemaReconciler) DeleteReconciler(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info("Deleting Schema: ", "Name", schema.Name, "Namespace", schema.Namespace)
	if err := schemaRegistry.DeleteSchema(ctx, schema, logger); err != nil {
		logger.Error(err, "failed to delete schema")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	controllerutil.RemoveFinalizer(schema, SchemaFinalizer)
	if err := r.Update(ctx, schema); err != nil {
		logger.Error(err, "failed to remove finalizer from schema")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	return ctrl.Result{}, nil
}

// CreateReconciler creates a new schema in the schema registry
func (r *SchemaReconciler) CreateReconciler(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info("Creating Schema: ", "Name", schema.Name, "Namespace", schema.Namespace)
	controllerutil.AddFinalizer(schema, SchemaFinalizer)

	if schema.Spec.Subject == "" {
		schema.Spec.Subject = schema.Name
	}

	unique, err := schema.IsSubjectUnique(ctx, r, schemaRegistry.Name)
	if err != nil {
		logger.Error(err, "failed to check if subject is unique")
		return ctrl.Result{}, err
	}

	if !unique {
		logger.Info("subject is not unique")

		message := fmt.Sprintf("Subject %s is not unique in Schema Registry %s",
			schema.GetSubject(), schemaRegistry.Name)
		schema.UpdateStatus(false, message)

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if err := r.Update(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	return ctrl.Result{}, nil
}

// UpdateReconciler updates the schema in the schema registry
func (r *SchemaReconciler) UpdateReconciler(
	ctx context.Context,
	schema *clientv1alpha1.Schema,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) (ctrl.Result, error) {
	logger.Info("Updating Schema: ", "Name", schema.Name, "Namespace", schema.Namespace)
	srSchemaObject, err := schemaRegistry.DeploySchema(ctx, schema, logger)
	if err != nil {
		logger.Error(err, "failed to deploy schema to schema registry", "schema", schema)

		if errors.Is(err, clientv1alpha1.ErrIncompatibleSchema) || errors.Is(err, clientv1alpha1.ErrInvalidSchemaOrType) {
			schema.Status.SchemaRegistryError = errors.Unwrap(err).Error()
		}

		schema.UpdateStatus(false, "Failed to deploy schema to Schema Registry: "+schemaRegistry.Name)

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	err = schemaRegistry.ChangeCompatibilityLevel(ctx, schema, logger)
	if err != nil {
		logger.Error(err, "failed to change compatibility level")
		schema.UpdateStatus(false, "Failed to change compatibility level in Schema Registry: "+schemaRegistry.Name)

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

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.Schema{}).
		Complete(r)
}
