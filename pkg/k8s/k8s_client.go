package k8s_manager

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	client.Client
}

// Upsert creates or updates the given object in the cluster
func (r *Client) Upsert(ctx context.Context, obj client.Object, exists bool) error {
	if exists {
		return r.Update(ctx, obj)
	}

	return r.Create(ctx, obj)
}

func NewClient(client client.Client) *Client {
	return &Client{Client: client}
}
