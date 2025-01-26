package hash

import (
	"fmt"
	"hash/fnv"
)

type Hashable interface {
	Hash() (uint32, error)
}

func Hash(s string) (uint32, error) {
	h := fnv.New32a()
	_, err := h.Write([]byte(s))
	if err != nil {
		return 0, fmt.Errorf("failed to hash string: %w", err)
	}

	return h.Sum32(), nil
}
