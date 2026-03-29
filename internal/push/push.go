// Package push implements the kaal push command logic.
package push

import (
	"context"
	"fmt"
	"os"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/gitutil"
	"github.com/mouhamedsylla/kaal/internal/registry"
	"github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Options controls kaal push behaviour.
type Options struct {
	Tag       string   // explicit tag; empty = git short SHA
	NoCache   bool
	Platforms []string // empty = current platform only
}

// Run executes kaal push: login → build → push.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	if cfg.Registry.Image == "" {
		return fmt.Errorf("registry.image is not set in kaal.yaml\n  Add: registry:\\n    provider: ghcr\\n    image: ghcr.io/<user>/<project>")
	}

	tag, err := resolveTag(opts.Tag)
	if err != nil {
		return err
	}

	fullTag := cfg.Registry.Image + ":" + tag

	dockerfile := resolveDockerfile(cfg)
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found at %q\n  Run 'kaal up' or ask your AI agent to generate it first", dockerfile)
	}

	reg, err := runtime.NewRegistry(cfg)
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Logging in to %s", cfg.Registry.Provider))
	if err := reg.Login(ctx); err != nil {
		return fmt.Errorf("registry login: %w", err)
	}

	ui.Info(fmt.Sprintf("Building %s", fullTag))
	if err := reg.Build(ctx, registry.BuildOptions{
		Tag:        fullTag,
		Dockerfile: dockerfile,
		Context:    ".",
		Platforms:  opts.Platforms,
		NoCache:    opts.NoCache,
	}); err != nil {
		return fmt.Errorf("docker build: %w", err)
	}

	ui.Info(fmt.Sprintf("Pushing %s", fullTag))
	if err := reg.Push(ctx, fullTag); err != nil {
		return fmt.Errorf("docker push: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Pushed %s", fullTag))
	fmt.Println()
	ui.Dim(fmt.Sprintf("  kaal deploy --env prod --tag %s", tag))
	fmt.Println()

	return nil
}

// resolveTag returns the explicit tag or the git short SHA of HEAD.
func resolveTag(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return gitutil.ShortSHA()
}

// resolveDockerfile returns the Dockerfile path to use for the build.
// It prefers a custom dockerfile declared on the app service over the default.
func resolveDockerfile(cfg *config.Config) string {
	for _, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp && svc.Dockerfile != "" {
			return svc.Dockerfile
		}
	}
	return "Dockerfile"
}
