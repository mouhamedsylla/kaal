package registry

import "context"

// Registry abstracts container image registries: GHCR, Docker Hub, ECR, GCR, ACR, custom.
type Registry interface {
	// Login authenticates with the registry.
	Login(ctx context.Context) error

	// Build builds a Docker image with the given options.
	Build(ctx context.Context, opts BuildOptions) error

	// Push pushes an already-built image to the registry.
	Push(ctx context.Context, tag string) error

	// Pull pulls an image from the registry.
	Pull(ctx context.Context, tag string) error
}

// BuildOptions controls the docker build step.
type BuildOptions struct {
	Tag        string
	Dockerfile string
	Context    string
	Platforms  []string // for multi-arch: linux/amd64, linux/arm64
	BuildArgs  map[string]string
	NoCache    bool
}
