package controller

import (
	"context"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clientv1alpha1 "github.com/steffen-karlsson/schema-registry-operator/api/v1alpha1"
	"github.com/steffen-karlsson/schema-registry-operator/pkg/hash"
)

// Updated checks if the content hash has changed, or it is a new object
func Updated(meta metav1.ObjectMeta, s hash.Hashable) (bool, error) {
	contentHash, exists := meta.Labels[SchemaRegistryContentHash]
	newHash, err := s.Hash()
	if err != nil {
		return false, err
	}
	return !exists || contentHash != strconv.Itoa(int(newHash)), nil
}

func FetchSchemaRegistryInstance(
	ctx context.Context,
	reader client.Reader,
	meta metav1.ObjectMeta,
	updatable clientv1alpha1.Updatable,
) (*clientv1alpha1.SchemaRegistry, bool, error) {
	instance, ok := meta.Labels[SchemaRegistryLabelName]
	if !ok {
		updatable.UpdateStatus(false, "Instance label: "+SchemaRegistryLabelName+" not found")
		return nil, true, ErrInstanceLabelNotFound
	}

	schemaRegistry := &clientv1alpha1.SchemaRegistry{}
	err := reader.Get(ctx, types.NamespacedName{Name: instance, Namespace: meta.Namespace}, schemaRegistry)
	switch {
	case apierrors.IsNotFound(err):
		updatable.UpdateStatus(false, "Schema Registry instance not found")
		return nil, false, ErrInstanceNotFound
	case err != nil:
		return nil, false, err
	}

	return schemaRegistry, false, err
}
