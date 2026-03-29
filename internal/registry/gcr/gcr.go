package gcr

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/registry"
)

// Registry is a stub — GCP Artifact Registry support is not yet implemented.
type Registry struct {
	image string
}

func New(image string) *Registry {
	return &Registry{image: image}
}

func (r *Registry) Login(_ context.Context) error {
	return fmt.Errorf("gcr registry: not yet implemented")
}

func (r *Registry) Build(_ context.Context, _ registry.BuildOptions) error {
	return fmt.Errorf("gcr registry: not yet implemented")
}

func (r *Registry) Push(_ context.Context, _ string) error {
	return fmt.Errorf("gcr registry: not yet implemented")
}

func (r *Registry) Pull(_ context.Context, _ string) error {
	return fmt.Errorf("gcr registry: not yet implemented")
}
