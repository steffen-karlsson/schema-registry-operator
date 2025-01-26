package controller

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/steffen-karlsson/schema-registry-operator/pkg/hash"
)

// NotUpdated checks if the content hash of the object is the same as the one in the metadata
func NotUpdated(meta metav1.ObjectMeta, s hash.Hashable) (bool, error) {
	contentHash, exists := meta.Labels[SchemaRegistryContentHash]
	newHash, err := s.Hash()
	if err != nil {
		return false, err
	}
	return exists && contentHash == strconv.Itoa(int(newHash)), nil
}
