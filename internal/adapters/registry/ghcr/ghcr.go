package ghcr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/adapters/registry/imgbuild"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Registry implements domain.RegistryProvider for GitHub Container Registry.
type Registry struct {
	image string
}

func New(image string) *Registry {
	return &Registry{image: image}
}

func (r *Registry) Login(ctx context.Context) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}
	user := os.Getenv("GITHUB_ACTOR")
	if user == "" {
		return fmt.Errorf("GITHUB_ACTOR environment variable is not set")
	}
	cmd := exec.CommandContext(ctx, "docker", "login", "ghcr.io", "-u", user, "--password-stdin")
	cmd.Stdin = strings.NewReader(token)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ghcr login failed: %w\n%s", err, out)
	}
	return nil
}

func (r *Registry) Build(ctx context.Context, opts domain.BuildOptions) error {
	return imgbuild.Build(ctx, opts)
}

func (r *Registry) Push(ctx context.Context, tag string) error {
	return imgbuild.Push(ctx, tag)
}
