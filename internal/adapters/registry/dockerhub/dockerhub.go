package dockerhub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/adapters/registry/imgbuild"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Registry implements domain.RegistryProvider for Docker Hub.
type Registry struct {
	image string
}

func New(image string) *Registry {
	return &Registry{image: image}
}

func (r *Registry) Login(ctx context.Context) error {
	user := os.Getenv("DOCKER_USERNAME")
	pass := os.Getenv("DOCKER_PASSWORD")
	if user == "" || pass == "" {
		return fmt.Errorf("DOCKER_USERNAME and DOCKER_PASSWORD must be set")
	}
	cmd := exec.CommandContext(ctx, "docker", "login", "-u", user, "--password-stdin")
	cmd.Stdin = strings.NewReader(pass)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dockerhub login failed: %w\n%s", err, out)
	}
	return nil
}

func (r *Registry) Build(ctx context.Context, opts domain.BuildOptions) error {
	return imgbuild.Build(ctx, opts)
}

func (r *Registry) Push(ctx context.Context, tag string) error {
	return imgbuild.Push(ctx, tag)
}
