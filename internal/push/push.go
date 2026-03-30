// Package push implements the kaal push command logic.
package push

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/config"
	kaalenv "github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/internal/gitutil"
	"github.com/mouhamedsylla/kaal/internal/registry"
	kaalRuntime "github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Options controls kaal push behaviour.
type Options struct {
	Env       string   // active environment — used to resolve build_args from the env file
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
		return fmt.Errorf("registry.image is not set in kaal.yaml\n  Add: registry:\n    provider: ghcr\n    image: ghcr.io/<user>/<project>")
	}
	if isPlaceholderImage(cfg.Registry.Image) {
		return fmt.Errorf(
			"registry.image still contains a placeholder: %q\n"+
				"  Edit kaal.yaml and replace it with your real image name, e.g.:\n"+
				"    registry:\n"+
				"      image: ghcr.io/mouhamedsylla/%s",
			cfg.Registry.Image, cfg.Project.Name,
		)
	}

	tag, err := resolveTag(opts.Tag)
	if err != nil {
		return err
	}

	fullTag := cfg.Registry.Image + ":" + tag

	// Default to linux/amd64 — most VPS targets are x86_64.
	// On macOS ARM64 (Apple Silicon), Docker builds arm64 images by default
	// which will crash-loop on amd64 VPS. We always override to linux/amd64
	// unless the caller explicitly passed --platform.
	platforms := opts.Platforms
	if len(platforms) == 0 {
		platforms = []string{"linux/amd64"}
		if runtime.GOARCH == "arm64" {
			ui.Info("Detected macOS ARM64 — building for linux/amd64 (VPS target)")
			ui.Dim("  Pass --platform linux/arm64 if your VPS is ARM-based")
		}
	}

	dockerfile := resolveDockerfile(cfg)
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found at %q\n  Run 'kaal up' or ask your AI agent to generate it first", dockerfile)
	}

	// Resolve build args from the active env file (for VITE_* and similar compile-time vars).
	buildArgs, err := resolveBuildArgs(cfg, kaalenv.Active(opts.Env))
	if err != nil {
		return err
	}
	if len(buildArgs) > 0 {
		names := make([]string, 0, len(buildArgs))
		for k := range buildArgs {
			names = append(names, k)
		}
		ui.Info(fmt.Sprintf("Injecting build args: %s", strings.Join(names, ", ")))
	}

	reg, err := kaalRuntime.NewRegistry(cfg)
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Logging in to %s", cfg.Registry.Provider))
	if err := reg.Login(ctx); err != nil {
		return fmt.Errorf("registry login: %w", err)
	}

	ui.Info(fmt.Sprintf("Building %s [%s]", fullTag, strings.Join(platforms, ",")))
	if err := reg.Build(ctx, registry.BuildOptions{
		Tag:        fullTag,
		Dockerfile: dockerfile,
		Context:    ".",
		Platforms:  platforms,
		BuildArgs:  buildArgs,
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

// isPlaceholderImage returns true if the image name was never customised after init.
func isPlaceholderImage(image string) bool {
	return strings.Contains(image, "YOUR_") ||
		strings.HasPrefix(image, "YOUR_DOCKERHUB_USER/") ||
		strings.HasPrefix(image, "ghcr.io/YOUR_GITHUB_USER/")
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

// resolveBuildArgs reads cfg.Registry.BuildArgs (list of var names) and resolves
// their values from the active environment's env file. Returns only declared vars
// that are present in the file (or in the current process env as fallback).
//
// This is the mechanism for injecting VITE_* and similar compile-time variables:
//   kaal.yaml:  registry.build_args: [VITE_APP_ENV, VITE_API_URL]
//   .env.prod:  VITE_APP_ENV=prod
//   → docker build --build-arg VITE_APP_ENV=prod
func resolveBuildArgs(cfg *config.Config, activeEnv string) (map[string]string, error) {
	if len(cfg.Registry.BuildArgs) == 0 {
		return nil, nil
	}

	// Find the env file for the active environment.
	envFile := ""
	if envCfg, ok := cfg.Environments[activeEnv]; ok {
		envFile = envCfg.EnvFile
	}

	// Parse the env file into a flat map.
	fileVars := map[string]string{}
	if envFile != "" {
		parsed, err := parseEnvFile(envFile)
		if err != nil {
			// Non-fatal: warn but continue — vars may come from process env.
			ui.Warn(fmt.Sprintf("Could not read %s for build args: %v", envFile, err))
		} else {
			fileVars = parsed
		}
	}

	result := map[string]string{}
	var missing []string
	for _, name := range cfg.Registry.BuildArgs {
		if v, ok := fileVars[name]; ok {
			result[name] = v
		} else if v := os.Getenv(name); v != "" {
			// Fallback to process environment (e.g. CI/CD pipeline vars).
			result[name] = v
		} else {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"build_args declared in kaal.yaml but not found in %s or environment: %s\n"+
				"  Add them to %s or export them before running kaal push",
			envFile, strings.Join(missing, ", "), envFile,
		)
	}

	return result, nil
}

// parseEnvFile parses a .env file into a map. Supports KEY=VALUE and KEY="VALUE".
// Ignores blank lines and lines starting with #.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip optional surrounding quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			vars[key] = val
		}
	}
	return vars, scanner.Err()
}
