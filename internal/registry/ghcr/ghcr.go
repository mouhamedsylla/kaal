package ghcr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/registry"
)

// Registry implements registry.Registry for GitHub Container Registry.
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

func (r *Registry) Build(ctx context.Context, opts registry.BuildOptions) error {
	args := []string{"build", "-t", opts.Tag}
	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}
	if opts.NoCache {
		args = append(args, "--no-cache")
	}
	for k, v := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}
	if len(opts.Platforms) > 0 {
		args = append(args, "--platform", strings.Join(opts.Platforms, ","))
	}
	ctxPath := opts.Context
	if ctxPath == "" {
		ctxPath = "."
	}
	args = append(args, ctxPath)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
