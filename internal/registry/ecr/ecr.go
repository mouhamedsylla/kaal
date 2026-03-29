package ecr

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/registry"
)

// Registry is a stub — AWS ECR support is not yet implemented.
type Registry struct {
	image string
	url   string
}

func New(image, url string) *Registry {
	return &Registry{image: image, url: url}
}

func (r *Registry) Login(_ context.Context) error {
	return fmt.Errorf("ecr registry: not yet implemented")
}

func (r *Registry) Build(_ context.Context, _ registry.BuildOptions) error {
	return fmt.Errorf("ecr registry: not yet implemented")
}

func (r *Registry) Push(_ context.Context, _ string) error {
	return fmt.Errorf("ecr registry: not yet implemented")
}

func (r *Registry) Pull(_ context.Context, _ string) error {
	return fmt.Errorf("ecr registry: not yet implemented")
}
