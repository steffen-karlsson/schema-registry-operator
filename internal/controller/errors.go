package controller

import (
	"errors"
	"fmt"
)

var (
	ErrInstanceLabelNotFound = errors.New("instance label not found")
	ErrInstanceNotFound      = errors.New("schema registry instance not found")
	ErrIncompatibleSchema    = errors.New("incompatible schema")
	ErrInvalidSchemaOrType   = errors.New("invalid schema or schema type")
	ErrInvalidSchemaVersion  = errors.New("invalid schema version")
	ErrSchemaVersionNotFound = errors.New("schema version not found")
	ErrSchemaSubjectNotFound = errors.New("schema subject not found")
)

func NewIncompatibleSchemaError(message string) error {
	return fmt.Errorf("%w: %s", ErrIncompatibleSchema, message)
}

func NewInvalidSchemaOrTypeError(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalidSchemaOrType, message)
}
