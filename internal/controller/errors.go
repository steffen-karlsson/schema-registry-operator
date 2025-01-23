package controller

import (
	"errors"
)

var (
	ErrInstanceLabelNotFound = errors.New("instance label not found")
	ErrInstanceNotFound      = errors.New("schema registry instance not found")
)
