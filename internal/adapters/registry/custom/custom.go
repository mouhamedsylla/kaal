package custom

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/imgbuild"
)

// Registry implements domain.RegistryProvider for a self-hosted private registry.
type Registry struct {
	image       string
	registryURL string
}

func New(image, registryURL string) *Registry {
	return &Registry{image: image, registryURL: registryURL}
}

func (r *Registry) Login(ctx context.Context) error {
	user := os.Getenv("REGISTRY_USERNAME")
	pass := os.Getenv("REGISTRY_PASSWORD")
	if user == "" || pass == "" {
		return fmt.Errorf("REGISTRY_USERNAME and REGISTRY_PASSWORD must be set for custom registry")
	}
	cmd := exec.CommandContext(ctx, "docker", "login", r.registryURL, "-u", user, "-p", pass)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("registry login failed: %w\n%s", err, out)
	}
	return nil
}

func (r *Registry) Build(ctx context.Context, opts domain.BuildOptions) error {
	return imgbuild.Build(ctx, opts)
}

func (r *Registry) Push(ctx context.Context, tag string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
