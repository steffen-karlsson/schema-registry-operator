package v1alpha1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/steffen-karlsson/schema-registry-operator/pkg/srclient"
)

// NewInstance creates a new instance of the SchemaRegistry CRD
func (s *SchemaRegistry) NewInstance(
	ctx context.Context,
	reader client.Reader,
	meta metav1.ObjectMeta,
	updatable Updatable,
) error {
	instance, ok := meta.Labels[SchemaRegistryLabelName]
	if !ok {
		updatable.UpdateStatus(false, "Instance label: "+SchemaRegistryLabelName+" not found")
		return ErrInstanceLabelNotFound
	}

	err := reader.Get(ctx, types.NamespacedName{Name: instance, Namespace: meta.Namespace}, s)
	switch {
	case apierrors.IsNotFound(err):
		updatable.UpdateStatus(false, "Schema Registry instance not found")
		return ErrInstanceNotFound
	case err != nil:
		return err
	}

	return nil
}

// DeploySchema deploys a schema to the schema registry
func (s *SchemaRegistry) DeploySchema(
	ctx context.Context,
	schema *Schema,
	logger logr.Logger,
) (*srclient.Schema, error) {
	srClient, err := s.newClient()
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

// DeleteSchema deletes a schema from the schema registry
func (s *SchemaRegistry) DeleteSchema(
	ctx context.Context,
	schema *Schema,
	logger logr.Logger,
) error {
	logger.Info("Deleting schema in schema registry", "Name", schema.Name, "Namespace", schema.Namespace)
	srClient, err := s.newClient()
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

// ChangeCompatibilityLevel changes the compatibility level of a schema in the schema registry
func (s *SchemaRegistry) ChangeCompatibilityLevel(
	ctx context.Context,
	schema *Schema,
	logger logr.Logger,
) error {
	srClient, err := s.newClient()
	if err != nil {
		logger.Error(err, "failed to create schema registry client")
		return err
	}
	resp, err := srClient.UpdateSubjectLevelConfig1WithResponse(ctx, schema.GetSubject(), srclient.UpdateSubjectLevelConfig1JSONRequestBody{
		Compatibility: ptr.To(srclient.ConfigUpdateRequestCompatibility(schema.Spec.CompatibilityLevel)),
	})

	if err != nil || resp.HTTPResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update compatibility level: %w", err)
	}

	return nil
}

func (s *SchemaRegistry) newClient() (*srclient.ClientWithResponses, error) {
	server := fmt.Sprintf("http://%s:%d", s.Name, s.Spec.Port)
	return srclient.NewClientWithResponses(server)
}
