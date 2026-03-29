package registry

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BuildImage runs docker build (or docker buildx build for multi-platform).
// All registry implementations delegate their Build method here.
func BuildImage(ctx context.Context, opts BuildOptions) error {
	var args []string

	if len(opts.Platforms) > 0 {
		// Multi-platform requires buildx
		args = []string{"buildx", "build", "--push", "-t", opts.Tag}
		args = append(args, "--platform", strings.Join(opts.Platforms, ","))
	} else {
		args = []string{"build", "-t", opts.Tag}
	}

	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}
	if opts.NoCache {
		args = append(args, "--no-cache")
	}
	for k, v := range opts.BuildArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
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
