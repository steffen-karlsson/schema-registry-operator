package controller

import (
	"errors"
	"fmt"
)

var (
	ErrInstanceLabelNotFound            = errors.New("instance label not found")
	ErrInstanceNotFound                 = errors.New("schema registry instance not found")
	ErrIncompatibleSchema               = errors.New("incompatible schema")
	ErrInvalidSchemaOrType              = errors.New("invalid schema or schema type")
	ErrInvalidSchemaVersionModification = errors.New("no previous active schema version found, SchemaVersion has been modified manually")
)

func NewIncompatibleSchemaError(message string) error {
	return fmt.Errorf("%w: %s", ErrIncompatibleSchema, message)
}

func NewInvalidSchemaOrTypeError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidSchemaOrType, message)
}
