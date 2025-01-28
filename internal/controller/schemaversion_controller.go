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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
	k8s_manager "github.com/steffen-karlsson/schema-registry-operator/pkg/k8s"
	"github.com/steffen-karlsson/schema-registry-operator/pkg/srclient"
)

const (
	SchemaVersionWasSoftDeleted = 40406
)

// SchemaVersionReconciler reconciles a SchemaVersion object
type SchemaVersionReconciler struct {
	k8s_manager.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaversions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaversions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=client.sroperator.io,resources=schemaversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the SchemaVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SchemaVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the SchemaVersion instance
	schemaVersion := &clientv1alpha1.SchemaVersion{}
	err := r.Get(ctx, req.NamespacedName, schemaVersion)
	switch {
	case apierrors.IsNotFound(err):
		logger.Info("SchemaVersion resource not found. Ignoring since object must be deleted")

		// The purpose is to get the SchemaRegistry instance
		// Retry parameter is not used, as the instance label is set by operator automatically on creation
		schemaRegistry, _, err := FetchSchemaRegistryInstance(ctx, r, schemaVersion.ObjectMeta, schemaVersion)
		if err != nil {
			logger.Error(err, "failed to get schema registry instance")

			if err = r.Status().Update(ctx, schemaRegistry); err != nil {
				logger.Error(err, "failed to update schema status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		// Delete the schema version from the SchemaRegistry
		if err = r.deleteSchemaVersion(ctx, schemaVersion, schemaRegistry, logger); err != nil {
			logger.Error(err, "failed to delete schema",
				"subject", schemaVersion.Spec.Subject,
				"version", schemaVersion.Spec.Version)

			if err = r.Status().Update(ctx, schemaVersion); err != nil {
				logger.Error(err, "failed to update schema version status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
	case err != nil:
		logger.Error(err, "failed to get SchemaVersion", "name", req.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SchemaVersionReconciler) deleteSchemaVersion(
	ctx context.Context,
	schemaVersion *clientv1alpha1.SchemaVersion,
	schemaRegistry *clientv1alpha1.SchemaRegistry,
	logger logr.Logger,
) error {
	srClient, err := schemaRegistry.NewInstance()
	if err != nil {
		logger.Error(err, "failed to create schema registry client")
		return err
	}

	// Delete the schema from the SchemaRegistry
	subject := schemaVersion.Spec.Subject
	version := strconv.Itoa(schemaVersion.Spec.Version)

	// For SR we first need to soft delete schema version
	softDeleteResp, err := srClient.DeleteSchemaVersion1WithResponse(ctx, subject, version, &srclient.DeleteSchemaVersion1Params{
		Permanent: ptr.To(false),
	})

	if apierrors.IsNotFound(err) || apierrors.IsInvalid(err) {
		srStatusCode := *softDeleteResp.ApplicationvndSchemaregistryV1JSON404.ErrorCode
		// Check if the schema version is only soft deleted in SR
		if srStatusCode != SchemaVersionWasSoftDeleted {
			// Success schema version is not in SR anyway and has been permanent deleted
			return nil
		}
	}

	// If the schema version is found, we can delete it permanently
	_, err = srClient.DeleteSchemaVersion1WithResponse(ctx, subject, version, &srclient.DeleteSchemaVersion1Params{
		Permanent: ptr.To(true),
	})

	if apierrors.IsNotFound(err) || apierrors.IsInvalid(err) {
		return nil
	}

	return err
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.SchemaVersion{}).
		Complete(r)
}
