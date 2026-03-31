// Package push implements the pilot push command logic.
package push

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/gitutil"
	"github.com/mouhamedsylla/pilot/internal/registry"
	pilotRuntime "github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot push behaviour.
type Options struct {
	Env       string   // active environment — used to resolve build_args from the env file
	Tag       string   // explicit tag; empty = git short SHA
	NoCache   bool
	Platforms []string // empty = current platform only
	Force     bool     // skip compile-time var gap check (use when vars are intentionally excluded)
}

// Run executes pilot push: login → build → push.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	if cfg.Registry.Image == "" {
		return fmt.Errorf("registry.image is not set in pilot.yaml\n  Add: registry:\n    provider: ghcr\n    image: ghcr.io/<user>/<project>")
	}
	if isPlaceholderImage(cfg.Registry.Image) {
		return fmt.Errorf(
			"registry.image still contains a placeholder: %q\n"+
				"  Edit pilot.yaml and replace it with your real image name, e.g.:\n"+
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
		return fmt.Errorf("Dockerfile not found at %q\n  Run 'pilot up' or ask your AI agent to generate it first", dockerfile)
	}

	// Resolve build args from the active env file (for VITE_* and similar compile-time vars).
	buildArgs, err := resolveBuildArgs(cfg, pilotenv.Active(opts.Env), opts.Force)
	if err != nil {
		return err
	}

	// If build args need to be injected, auto-patch the Dockerfile in a temp file
	// so the user never has to manually add ARG/ENV lines.
	// The original Dockerfile is never modified.
	if len(buildArgs) > 0 {
		names := make([]string, 0, len(buildArgs))
		for k := range buildArgs {
			names = append(names, k)
		}
		ui.Info(fmt.Sprintf("Injecting build args: %s", strings.Join(names, ", ")))

		patched, tmp, patchErr := patchDockerfileArgs(dockerfile, names)
		if patchErr != nil {
			ui.Warn(fmt.Sprintf("Could not auto-patch Dockerfile: %v — build args may be ignored", patchErr))
		} else if patched {
			defer os.Remove(tmp)
			dockerfile = tmp
			ui.Dim("  ARG/ENV lines auto-injected into builder stage (original Dockerfile unchanged)")
		}
	}

	reg, err := pilotRuntime.NewRegistry(cfg)
	if err != nil {
		return err
	}

	if err := ui.Spinner(fmt.Sprintf("Logging in to %s", cfg.Registry.Provider), func() error {
		return reg.Login(ctx)
	}); err != nil {
		return fmt.Errorf(
			"registry login failed for %s\n\n"+
				"  Check that your credentials are set:\n"+
				"    pilot preflight --target push\n\n"+
				"  Cause: %w",
			cfg.Registry.Provider, err,
		)
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
	ui.Dim(fmt.Sprintf("  pilot deploy --env prod --tag %s", tag))
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
// Strategy (no pilot.yaml config required for the common case):
//  1. Auto-detect: scan the active env file for vars matching known frontend
//     conventions (VITE_*, NEXT_PUBLIC_*, REACT_APP_*) — covers 90% of cases.
//  2. Override/extend: if registry.build_args is set in pilot.yaml, those names
//     are resolved instead (explicit list overrides auto-detection entirely).
//
// Values come from the env file first, process environment as fallback (CI/CD).
// Missing vars in override mode are a hard error; in auto-detect mode they are silently
// skipped (the env file simply may not have VITE_ vars for non-frontend projects).
func resolveBuildArgs(cfg *config.Config, activeEnv string, force bool) (map[string]string, error) {
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

	// ── Explicit override (registry.build_args in pilot.yaml) ──────────────
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
				"build_args declared in pilot.yaml but not found in %s: %s\n"+
					"  Add them to %s or export them before running pilot push",
				src, strings.Join(missing, ", "), src,
			)
		}

		// Detect compile-time vars present in the env file but absent from
		// build_args. These would be silently empty in the built image — block.
		declaredSet := map[string]bool{}
		for _, name := range cfg.Registry.BuildArgs {
			declaredSet[name] = true
		}
		compilePrefixes := []string{"VITE_", "NEXT_PUBLIC_", "REACT_APP_", "PUBLIC_", "NUXT_PUBLIC_", "NG_APP_"}
		var unlisted []string
		for name := range fileVars {
			if declaredSet[name] {
				continue
			}
			for _, prefix := range compilePrefixes {
				if strings.HasPrefix(name, prefix) {
					unlisted = append(unlisted, name)
					break
				}
			}
		}
		if len(unlisted) > 0 && !force {
			sort.Strings(unlisted)
			lines := []string{}
			for _, name := range unlisted {
				lines = append(lines, fmt.Sprintf("    - %s", name))
			}
			return nil, fmt.Errorf(
				"%d compile-time var(s) in %s are NOT in pilot.yaml registry.build_args:\n%s\n\n"+
					"  These vars would be silently EMPTY in the built image.\n\n"+
					"  Fix: add them to pilot.yaml:\n"+
					"    registry:\n"+
					"      build_args:\n"+
					"%s\n\n"+
					"  If these vars are intentionally excluded from the build, run:\n"+
					"    pilot push --force",
				len(unlisted), envFile,
				strings.Join(lines, "\n"),
				strings.Join(lines, "\n"),
			)
		}

		return result, nil
	}

	// ── Auto-detect frontend compile-time vars ─────────────────────────────
	// Only meaningful for node/frontend stacks. Scan the env file for vars
	// matching universal frontend conventions — no pilot.yaml config needed.
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

// patchDockerfileArgs creates a temporary copy of the Dockerfile with ARG + ENV
// declarations injected into the builder stage for any argNames not already declared.
// Returns (patched bool, tempPath string, err).
//
// Injection point: right before the first build command (RUN npm/yarn/pnpm run build
// or RUN npm run build). Falls back to inserting after the first FROM line.
// The original Dockerfile is never modified.
func patchDockerfileArgs(dockerfile string, argNames []string) (bool, string, error) {
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		return false, "", err
	}
	text := string(content)

	// Find which names are actually missing.
	var missing []string
	for _, name := range argNames {
		if !strings.Contains(text, "ARG "+name) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return false, "", nil // nothing to do
	}

	// Build the ARG + ENV block to inject.
	var block strings.Builder
	block.WriteString("# pilot: auto-injected build args for compile-time env vars\n")
	for _, name := range missing {
		block.WriteString(fmt.Sprintf("ARG %s\n", name))
	}
	for _, name := range missing {
		block.WriteString(fmt.Sprintf("ENV %s=$%s\n", name, name))
	}
	injection := block.String()

	// Find the best injection point: line before the frontend build command.
	buildPatterns := []string{
		"RUN npm run build",
		"RUN yarn build",
		"RUN pnpm run build",
		"RUN npm ci && npm run build",
		"RUN yarn install && yarn build",
	}
	lines := strings.Split(text, "\n")
	insertAt := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pat := range buildPatterns {
			if strings.HasPrefix(trimmed, pat) || trimmed == pat {
				insertAt = i
				break
			}
		}
		if insertAt >= 0 {
			break
		}
	}

	// Fallback: insert after the first FROM line.
	if insertAt < 0 {
		for i, line := range lines {
			if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "FROM ") {
				insertAt = i + 1
				break
			}
		}
	}

	if insertAt < 0 {
		return false, "", fmt.Errorf("could not find a suitable injection point in %s", dockerfile)
	}

	// Splice the injection into the lines slice.
	injectionLines := strings.Split(strings.TrimRight(injection, "\n"), "\n")
	patched := make([]string, 0, len(lines)+len(injectionLines))
	patched = append(patched, lines[:insertAt]...)
	patched = append(patched, injectionLines...)
	patched = append(patched, lines[insertAt:]...)

	// Write to a temp file alongside the original.
	tmp, err := os.CreateTemp(".", ".pilot-dockerfile-*.tmp")
	if err != nil {
		return false, "", err
	}
	if _, err := tmp.WriteString(strings.Join(patched, "\n")); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return false, "", err
	}
	tmp.Close()
	return true, tmp.Name(), nil
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
