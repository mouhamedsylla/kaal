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
		// Warn if the Dockerfile is missing ARG declarations for these vars —
		// docker passes them but Vite/webpack silently ignores undeclared ARGs.
		warnMissingDockerfileArgs(dockerfile, names)
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

// resolveBuildArgs collects compile-time variables to inject via --build-arg.
//
// Strategy (no kaal.yaml config required for the common case):
//  1. Auto-detect: scan the active env file for vars matching known frontend
//     conventions (VITE_*, NEXT_PUBLIC_*, REACT_APP_*) — covers 90% of cases.
//  2. Override/extend: if registry.build_args is set in kaal.yaml, those names
//     are resolved instead (explicit list overrides auto-detection entirely).
//
// Values come from the env file first, process environment as fallback (CI/CD).
// Missing vars in override mode are a hard error; in auto-detect mode they are silently
// skipped (the env file simply may not have VITE_ vars for non-frontend projects).
func resolveBuildArgs(cfg *config.Config, activeEnv string) (map[string]string, error) {
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
			ui.Warn(fmt.Sprintf("Could not read %s for build args: %v", envFile, err))
		} else {
			fileVars = parsed
		}
	}

	// ── Explicit override (registry.build_args in kaal.yaml) ──────────────
	if len(cfg.Registry.BuildArgs) > 0 {
		result := map[string]string{}
		var missing []string
		for _, name := range cfg.Registry.BuildArgs {
			if v, ok := fileVars[name]; ok {
				result[name] = v
			} else if v := os.Getenv(name); v != "" {
				result[name] = v
			} else {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			src := envFile
			if src == "" {
				src = "environment"
			}
			return nil, fmt.Errorf(
				"build_args declared in kaal.yaml but not found in %s: %s\n"+
					"  Add them to %s or export them before running kaal push",
				src, strings.Join(missing, ", "), src,
			)
		}
		return result, nil
	}

	// ── Auto-detect frontend compile-time vars ─────────────────────────────
	// Only meaningful for node/frontend stacks. Scan the env file for vars
	// matching universal frontend conventions — no kaal.yaml config needed.
	if cfg.Project.Stack != "node" {
		return nil, nil
	}

	compilePrefixes := []string{"VITE_", "NEXT_PUBLIC_", "REACT_APP_"}
	result := map[string]string{}
	for name, val := range fileVars {
		for _, prefix := range compilePrefixes {
			if strings.HasPrefix(name, prefix) {
				result[name] = val
				break
			}
		}
	}
	// Also check process env for the same prefixes (CI pipeline case).
	for _, prefix := range compilePrefixes {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, prefix) {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					if _, alreadySet := result[parts[0]]; !alreadySet {
						result[parts[0]] = parts[1]
					}
				}
			}
		}
	}

	return result, nil
}

// warnMissingDockerfileArgs reads the Dockerfile and warns if any of the given
// build arg names are not declared as ARG instructions. Without an ARG declaration,
// Docker silently drops the --build-arg value and the build tool (Vite, webpack)
// never sees it — a common source of "vars are undefined in prod" bugs.
func warnMissingDockerfileArgs(dockerfile string, argNames []string) {
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		return
	}
	text := string(content)
	var missing []string
	for _, name := range argNames {
		// Match "ARG NAME" or "ARG NAME=default"
		if !strings.Contains(text, "ARG "+name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		fmt.Println()
		ui.Warn(fmt.Sprintf("Dockerfile is missing ARG declarations for: %s", strings.Join(missing, ", ")))
		ui.Dim("  These build args will be passed but silently ignored by Docker.")
		ui.Dim("  Add to your Dockerfile builder stage (before the build command):")
		fmt.Println()
		for _, name := range missing {
			ui.Dim(fmt.Sprintf("    ARG %s", name))
		}
		for _, name := range missing {
			ui.Dim(fmt.Sprintf("    ENV %s=$%s", name, name))
		}
		fmt.Println()
	}
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
