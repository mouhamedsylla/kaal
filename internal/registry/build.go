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
//
// Single platform (e.g. linux/amd64):
//
//	docker build --platform linux/amd64 -t <tag> .
//
// Multi-platform (e.g. linux/amd64,linux/arm64):
//
//	docker buildx build --platform ... --push -t <tag> .
//	(buildx pushes directly — the subsequent Push() call is a no-op)
func BuildImage(ctx context.Context, opts BuildOptions) error {
	var args []string

	switch len(opts.Platforms) {
	case 0:
		// No platform specified — plain build for current machine arch.
		args = []string{"build", "-t", opts.Tag}

	case 1:
		// Single target platform: use plain docker build with --platform.
		// Works with Docker Desktop on macOS ARM64 to produce linux/amd64 images.
		args = []string{"build", "--platform", opts.Platforms[0], "-t", opts.Tag}

	default:
		// Multi-platform: requires buildx and pushes directly to the registry.
		args = []string{"buildx", "build",
			"--platform", strings.Join(opts.Platforms, ","),
			"--push",
			"-t", opts.Tag,
		}
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
