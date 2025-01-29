package v1alpha1

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsSubjectUnique checks if the subject of the schema is unique in the schema registry instance
func (s *Schema) IsSubjectUnique(
	ctx context.Context,
	r client.Reader,
	schemaRegistryInstanceName string,
) (bool, error) {
	potentialMatchingSchemas := &SchemaList{}
	if err := r.List(ctx, potentialMatchingSchemas,
		client.InNamespace(s.Namespace),
		client.MatchingLabels{SchemaRegistryLabelName: schemaRegistryInstanceName}); err != nil {

		return false, err
	}

	for _, potentialMatchingSchema := range potentialMatchingSchemas.Items {
		if potentialMatchingSchema.GetSubject() == s.GetSubject() {
			return false, nil
		}
	}

	return true, nil
}
