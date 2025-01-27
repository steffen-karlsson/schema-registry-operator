package controller

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
