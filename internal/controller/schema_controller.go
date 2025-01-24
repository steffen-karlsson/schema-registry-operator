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
	"github.com/steffen-karlsson/schema-registry-operator/pkg/hash"
	k8s_manager "github.com/steffen-karlsson/schema-registry-operator/pkg/k8s"
)

// SchemaReconciler reconciles a Schema object
type SchemaReconciler struct {
	k8s_manager.Client
	Scheme *runtime.Scheme
}

const (
	SchemaRegistryLabelName = "client.sroperator.io/instance"
	SchemaVersionLatest     = "latest"
)

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
	newContentHash, err := hash.Hash(schema.Spec.Content)
	if err != nil {
		logger.Error(err, "failed to hash schema content")
		return ctrl.Result{}, err
	}

	contentHash, exists := schema.ObjectMeta.Labels[SchemaRegistryContentHash]
	if exists && contentHash == strconv.Itoa(int(newContentHash)) {
		// No need to update the schema if the content hash is the same
		if err = r.updateStatusSuccessfully(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// The purpose is to get the SchemaRegistry instance
	schemaRegistry, err := r.fetchSchemaRegistryInstance(ctx, schema)

	switch {
	case errors.Is(err, ErrInstanceNotFound):
		schema.Status.Message = "Schema Registry instance not found"
		schema.Status.Ready = false

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	case errors.Is(err, ErrInstanceLabelNotFound):
		schema.Status.Message = "Instance label: " + SchemaRegistryLabelName + " not found"
		schema.Status.Ready = false

		if err = r.Status().Update(ctx, schema); err != nil {
			logger.Error(err, "failed to update schema status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	case err != nil:
		logger.Error(err, "failed to get schema registry instance")
		return ctrl.Result{}, err
	}

	version, err := r.deploySchema(ctx, &schemaRegistry, schema, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	schema.ObjectMeta.Labels[SchemaRegistryContentHash] = strconv.Itoa(int(newContentHash))

	if err = r.Update(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema content hash")
		return ctrl.Result{}, err
	}

	schema.Status.LatestVersion = version

	if err = r.updateStatusSuccessfully(ctx, schema); err != nil {
		logger.Error(err, "failed to update schema status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

func (r *SchemaReconciler) updateStatusSuccessfully(ctx context.Context, schema *clientv1alpha1.Schema) error {
	schema.Status.Ready = true
	schema.Status.Message = "Schema deployed successfully"

	return r.Status().Update(ctx, schema)
}

func (r *SchemaReconciler) fetchSchemaRegistryInstance(ctx context.Context, schema *clientv1alpha1.Schema) (clientv1alpha1.SchemaRegistry, error) {
	instance, ok := schema.ObjectMeta.Labels[SchemaRegistryLabelName]
	if !ok {
		return clientv1alpha1.SchemaRegistry{}, ErrInstanceLabelNotFound
	}

	schemaRegistry := &clientv1alpha1.SchemaRegistry{}
	err := r.Get(ctx, types.NamespacedName{Name: instance, Namespace: schema.Namespace}, schemaRegistry)
	if err != nil && apierrors.IsNotFound(err) {
		return clientv1alpha1.SchemaRegistry{}, ErrInstanceNotFound
	}

	return *schemaRegistry, err
}

func (r *SchemaReconciler) deploySchema(
	ctx context.Context,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	schema *clientv1alpha1.Schema,
	logger logr.Logger,
) (int, error) {
	version := max(1, schema.Status.LatestVersion) + 1

	schemaVersion := r.createSchemaVersion(schema, version)
	if err := ctrl.SetControllerReference(schema, &schemaVersion, r.Scheme); err != nil {
		logger.Error(err, "failed to set controller reference", "schemaversion", schemaVersion)
		return 0, err
	}

	if err := r.Upsert(ctx, &schemaVersion, false); err != nil {
		logger.Error(err, "failed to create schemaversion", "schemaversion", schemaVersion)
		return 0, err
	}

	return version, nil
}

func (r *SchemaReconciler) createSchemaVersion(schema *clientv1alpha1.Schema, version int) clientv1alpha1.SchemaVersion {
	return clientv1alpha1.SchemaVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      schema.Name + "-v" + strconv.Itoa(int(version)),
			Namespace: schema.Namespace,
		},
		Spec: clientv1alpha1.SchemaVersionSpec{
			Subject: schema.GetSubject(),
			Version: version,
			Content: schema.Spec.Content,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.Schema{}).
		Complete(r)
}
