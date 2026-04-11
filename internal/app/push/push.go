// Package push implements the pilot push use case.
//
// PushUseCase orchestrates: login → build → push.
// Complex build-arg logic (env file parsing, Dockerfile patching) is kept here
// as it is pure business logic with no network I/O.
// All actual container operations are delegated to the injected RegistryProvider.
package push

import (
	"bufio"
	"context"
	"fmt"
	goruntime "runtime"
	"os"
	"sort"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/gitutil"
)

// Input is the data required to push an image.
type Input struct {
	Env       string
	Tag       string   // explicit tag; empty = git short SHA
	NoCache   bool
	Platforms []string // empty = ["linux/amd64"]
	Force     bool     // skip compile-time var gap check
	Config    *config.Config
}

// Output is the result of a successful push.
type Output struct {
	Tag   string // resolved tag that was pushed
	Image string // full image:tag reference
	// ARMDetected is true when the build was cross-compiled (macOS ARM64 → linux/amd64).
	ARMDetected bool
}

// PushUseCase builds and pushes a Docker image.
type PushUseCase struct {
	provider domain.RegistryProvider
}

// New constructs a PushUseCase.
func New(provider domain.RegistryProvider) *PushUseCase {
	return &PushUseCase{provider: provider}
}

// Execute runs the push pipeline and returns the pushed tag.
func (uc *PushUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	cfg := in.Config

	if cfg.Registry.Image == "" {
		return Output{}, fmt.Errorf(
			"registry.image is not set in pilot.yaml\n" +
				"  Add: registry:\n    provider: ghcr\n    image: ghcr.io/<user>/<project>",
		)
	}
	if isPlaceholderImage(cfg.Registry.Image) {
		return Output{}, fmt.Errorf(
			"registry.image still contains a placeholder: %q\n"+
				"  Edit pilot.yaml and replace it with your real image name, e.g.:\n"+
				"    registry:\n      image: ghcr.io/mouhamedsylla/%s",
			cfg.Registry.Image, cfg.Project.Name,
		)
	}

	tag, err := resolveTag(in.Tag)
	if err != nil {
		return Output{}, err
	}

	fullTag := cfg.Registry.Image + ":" + tag

	platforms := in.Platforms
	armDetected := false
	if len(platforms) == 0 {
		platforms = []string{"linux/amd64"}
		if goruntime.GOARCH == "arm64" {
			armDetected = true
		}
	}

	dockerfile := resolveDockerfile(cfg)
	if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
		return Output{}, fmt.Errorf(
			"Dockerfile not found at %q\n  Run 'pilot up' or ask your AI agent to generate it first",
			dockerfile,
		)
	}

	buildArgs, err := resolveBuildArgs(cfg, in.Env, in.Force)
	if err != nil {
		return Output{}, err
	}

	if len(buildArgs) > 0 {
		names := argNames(buildArgs)
		patched, tmp, patchErr := patchDockerfileArgs(dockerfile, names)
		if patchErr == nil && patched {
			defer os.Remove(tmp)
			dockerfile = tmp
		}
	}

	if err := uc.provider.Login(ctx); err != nil {
		return Output{}, fmt.Errorf(
			"registry login failed for %s\n\n"+
				"  Check that your credentials are set:\n"+
				"    pilot preflight --target push\n\n"+
				"  Cause: %w",
			cfg.Registry.Provider, err,
		)
	}

	if err := uc.provider.Build(ctx, domain.BuildOptions{
		Tag:        fullTag,
		Dockerfile: dockerfile,
		Context:    ".",
		Platforms:  platforms,
		BuildArgs:  buildArgs,
		NoCache:    in.NoCache,
	}); err != nil {
		return Output{}, fmt.Errorf("docker build: %w", err)
	}

	if err := uc.provider.Push(ctx, fullTag); err != nil {
		return Output{}, fmt.Errorf("docker push: %w", err)
	}

	return Output{Tag: tag, Image: fullTag, ARMDetected: armDetected}, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveTag(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return gitutil.ShortSHA()
}

func isPlaceholderImage(image string) bool {
	return strings.Contains(image, "YOUR_") ||
		strings.HasPrefix(image, "YOUR_DOCKERHUB_USER/") ||
		strings.HasPrefix(image, "ghcr.io/YOUR_GITHUB_USER/")
}

func resolveDockerfile(cfg *config.Config) string {
	for _, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp && svc.Dockerfile != "" {
			return svc.Dockerfile
		}
	}
	return "Dockerfile"
}

func argNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// resolveBuildArgs collects compile-time variables to inject via --build-arg.
func resolveBuildArgs(cfg *config.Config, activeEnv string, force bool) (map[string]string, error) {
	envFile := ""
	if envCfg, ok := cfg.Environments[activeEnv]; ok {
		envFile = envCfg.EnvFile
	}

	fileVars := map[string]string{}
	if envFile != "" {
		parsed, err := parseEnvFile(envFile)
		if err == nil {
			fileVars = parsed
		}
	}

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

		compilePrefixes := []string{"VITE_", "NEXT_PUBLIC_", "REACT_APP_", "PUBLIC_", "NUXT_PUBLIC_", "NG_APP_"}
		declaredSet := map[string]bool{}
		for _, name := range cfg.Registry.BuildArgs {
			declaredSet[name] = true
		}
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
			lines := make([]string, len(unlisted))
			for i, name := range unlisted {
				lines[i] = "    - " + name
			}
			return nil, fmt.Errorf(
				"%d compile-time var(s) in %s are NOT in pilot.yaml registry.build_args:\n%s\n\n"+
					"  These vars would be silently EMPTY in the built image.\n\n"+
					"  Fix: add them to pilot.yaml:\n"+
					"    registry:\n      build_args:\n%s\n\n"+
					"  If these vars are intentionally excluded, run:\n    pilot push --force",
				len(unlisted), envFile,
				strings.Join(lines, "\n"),
				strings.Join(lines, "\n"),
			)
		}
		return result, nil
	}

	// Auto-detect frontend compile-time vars (node stack only).
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
	for _, prefix := range compilePrefixes {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, prefix) {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) == 2 {
					if _, ok := result[parts[0]]; !ok {
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
func patchDockerfileArgs(dockerfile string, argNames []string) (bool, string, error) {
	content, err := os.ReadFile(dockerfile)
	if err != nil {
		return false, "", err
	}
	text := string(content)

	var missing []string
	for _, name := range argNames {
		if !strings.Contains(text, "ARG "+name) {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return false, "", nil
	}

	var block strings.Builder
	block.WriteString("# pilot: auto-injected build args for compile-time env vars\n")
	for _, name := range missing {
		block.WriteString(fmt.Sprintf("ARG %s\n", name))
	}
	for _, name := range missing {
		block.WriteString(fmt.Sprintf("ENV %s=$%s\n", name, name))
	}
	injection := block.String()

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

	injectionLines := strings.Split(strings.TrimRight(injection, "\n"), "\n")
	patched := make([]string, 0, len(lines)+len(injectionLines))
	patched = append(patched, lines[:insertAt]...)
	patched = append(patched, injectionLines...)
	patched = append(patched, lines[insertAt:]...)

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

// parseEnvFile parses a .env file into a map.
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
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			vars[key] = val
		}
	}
	return vars, scanner.Err()
}
