// Package imgbuild wraps docker build / buildx build for all registry adapters.
package imgbuild

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Build runs docker build (or docker buildx build for multi-platform).
//
// Single platform (e.g. linux/amd64):
//
//	docker build --platform linux/amd64 -t <tag> .
//
// Multi-platform (e.g. linux/amd64,linux/arm64):
//
//	docker buildx build --platform ... --push -t <tag> .
//	(buildx pushes directly — the subsequent Push() call is a no-op)
func Build(ctx context.Context, opts domain.BuildOptions) error {
	var args []string

	switch len(opts.Platforms) {
	case 0:
		args = []string{"build", "-t", opts.Tag}
	case 1:
		args = []string{"build", "--platform", opts.Platforms[0], "-t", opts.Tag}
	default:
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
