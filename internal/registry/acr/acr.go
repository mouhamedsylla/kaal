package acr

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/registry"
)

// Registry is a stub — Azure Container Registry support is not yet implemented.
type Registry struct {
	image string
	url   string
}

func New(image, url string) *Registry {
	return &Registry{image: image, url: url}
}

func (r *Registry) Login(_ context.Context) error {
	return fmt.Errorf("acr registry: not yet implemented")
}

func (r *Registry) Build(_ context.Context, _ registry.BuildOptions) error {
	return fmt.Errorf("acr registry: not yet implemented")
}

func (r *Registry) Push(_ context.Context, _ string) error {
	return fmt.Errorf("acr registry: not yet implemented")
}

func (r *Registry) Pull(_ context.Context, _ string) error {
	return fmt.Errorf("acr registry: not yet implemented")
}
