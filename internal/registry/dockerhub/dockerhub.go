package dockerhub

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/registry"
)

// Registry implements registry.Registry for Docker Hub.
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

func (r *Registry) Build(ctx context.Context, opts registry.BuildOptions) error {
	return registry.BuildImage(ctx, opts)
}

func (r *Registry) Push(ctx context.Context, tag string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (r *Registry) Pull(ctx context.Context, tag string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", tag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
