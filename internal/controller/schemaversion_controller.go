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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
	k8s_manager "github.com/steffen-karlsson/schema-registry-operator/pkg/k8s"
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
	if err := r.Get(ctx, req.NamespacedName, schemaVersion); err != nil {
		logger.Error(err, "unable to fetch SchemaVersion")
		return ctrl.Result{}, err
	}

	// Update current state to active, as the SchemaVersion CRD cannot be mutated, we know it is a create operation
	schemaVersion.Status.Active = true
	schemaVersion.Status.Ready = true

	if err := r.Status().Update(ctx, schemaVersion); err != nil {
		logger.Error(err, "unable to update SchemaVersion status")
		return ctrl.Result{}, err
	}

	// Update the previous active SchemaVersion to inactive
	previousVersion, exists := schemaVersion.Annotations[PreviousActiveSchemaVersionAnnotationName]
	if !exists {
		logger.Error(ErrInvalidSchemaVersionModification, "unable to find previous active schema version")
		return ctrl.Result{}, nil
	}

	previousVersionInt, err := strconv.Atoi(previousVersion)
	if err != nil {
		logger.Error(err, "unable to convert previous active schema version to int")
		return ctrl.Result{}, err
	}

	if previousVersionInt == 0 {
		return ctrl.Result{}, nil
	}

	oldSchemaNamespacedName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      schemaVersion.Name + "-v" + strconv.Itoa(previousVersionInt),
	}

	oldSchemaVersion := &clientv1alpha1.SchemaVersion{}
	if err = r.Get(ctx, oldSchemaNamespacedName, schemaVersion); err != nil {
		logger.Error(err, "unable to fetch previous active SchemaVersion")
		return ctrl.Result{}, err
	}

	oldSchemaVersion.Status.Active = false
	oldSchemaVersion.Status.Ready = true

	if err = r.Status().Update(ctx, schemaVersion); err != nil {
		logger.Error(err, "unable to update previous active SchemaVersion status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SchemaVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1alpha1.SchemaVersion{}).
		Complete(r)
}
