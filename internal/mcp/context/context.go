// Package context collects the full project context.
// This is used by pilot up (when files are missing) and by the MCP server
// so that AI agents have everything they need to generate infrastructure files.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/scaffold"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
	"gopkg.in/yaml.v3"
)

// ProjectContext is the complete picture of a project at a given moment.
// It is serializable to JSON for the MCP response and printable for humans.
type ProjectContext struct {
	// From pilot.yaml
	KaalYAML string `json:"pilot_yaml"`

	// Detected project info
	Stack             string `json:"stack"`
	LanguageVersion   string `json:"language_version"`
	IsExistingProject bool   `json:"is_existing_project"`

	// File structure
	FileTree string   `json:"file_tree"`
	KeyFiles []string `json:"key_files"` // files relevant to infra generation

	// Existing infra files
	ExistingDockerfiles  []string `json:"existing_dockerfiles"`
	ExistingComposeFiles []string `json:"existing_compose_files"`
	ExistingEnvFiles     []string `json:"existing_env_files"`

	// What's missing
	MissingDockerfile  bool     `json:"missing_dockerfile"`
	MissingCompose     bool     `json:"missing_compose"`      // active env only (legacy)
	MissingComposeEnvs []string `json:"missing_compose_envs"` // all envs missing a compose file
	ActiveEnv          string   `json:"active_env"`

	// The parsed config (for structured access)
	Config *config.Config `json:"config"`
}

// Collect gathers the full project context from the current directory.
func Collect(activeEnv string) (*ProjectContext, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	detected := scaffold.Detect(".")

	ctx := &ProjectContext{
		Stack:             cfg.Project.Stack,
		LanguageVersion:   cfg.Project.LanguageVersion,
		IsExistingProject: detected.IsExisting,
		ActiveEnv:         activeEnv,
		Config:            cfg,
	}

	if ctx.Stack == "" {
		ctx.Stack = detected.Stack
	}
	if ctx.LanguageVersion == "" {
		ctx.LanguageVersion = detected.LanguageVersion
	}

	// Read pilot.yaml as raw string.
	raw, err := os.ReadFile(config.FileName)
	if err != nil {
		return nil, err
	}
	ctx.KaalYAML = string(raw)

	// File tree (max 3 levels deep, skip common noise).
	ctx.FileTree = buildFileTree(".", 0, 3)

	// Scan for relevant files.
	ctx.KeyFiles = scanKeyFiles(".")
	ctx.ExistingDockerfiles = glob(".", "Dockerfile*")
	ctx.ExistingComposeFiles = glob(".", "docker-compose*.yml")
	ctx.ExistingEnvFiles = globEnvFiles(".")

	// Active env compose check (legacy field).
	composeFile := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
	ctx.MissingDockerfile = !fileExists("Dockerfile") && !hasCustomDockerfile(cfg)
	ctx.MissingCompose = !fileExists(composeFile)

	// Check ALL configured environments for missing compose files.
	for envName := range cfg.Environments {
		f := fmt.Sprintf("docker-compose.%s.yml", envName)
		if !fileExists(f) {
			ctx.MissingComposeEnvs = append(ctx.MissingComposeEnvs, envName)
		}
	}
	sort.Strings(ctx.MissingComposeEnvs)

	return ctx, nil
}

// Summary returns a human-readable summary of the context.
func (c *ProjectContext) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Project:  %s\n", c.Config.Project.Name))
	b.WriteString(fmt.Sprintf("Stack:    %s %s\n", c.Stack, c.LanguageVersion))
	b.WriteString(fmt.Sprintf("Env:      %s\n", c.ActiveEnv))

	b.WriteString("\nServices:\n")
	// Use ServiceForEnv to reflect per-env overrides (e.g. managed postgres in prod)
	for name := range c.Config.Services {
		svc, _ := c.Config.ServiceForEnv(name, c.ActiveEnv)
		hosting := svc.Hosting
		if hosting == "" {
			hosting = "container"
		}
		line := fmt.Sprintf("  %-12s type=%-12s hosting=%s", name, svc.Type, hosting)
		if svc.Provider != "" {
			line += fmt.Sprintf(" provider=%s", svc.Provider)
		}
		if svc.Port > 0 {
			line += fmt.Sprintf(" port=%d", svc.Port)
		}
		b.WriteString(line + "\n")
	}

	if len(c.ExistingDockerfiles) > 0 {
		b.WriteString(fmt.Sprintf("\nDockerfiles: %s\n", strings.Join(c.ExistingDockerfiles, ", ")))
	}
	if len(c.ExistingComposeFiles) > 0 {
		b.WriteString(fmt.Sprintf("Compose:     %s\n", strings.Join(c.ExistingComposeFiles, ", ")))
	}

	return b.String()
}

// AgentPrompt returns a ready-to-use prompt for an AI agent to generate missing files.
func (c *ProjectContext) AgentPrompt() string {
	var b strings.Builder

	b.WriteString("Here is the full context of this pilot project.\n\n")

	b.WriteString("## pilot.yaml\n\n```yaml\n")
	b.WriteString(c.KaalYAML)
	b.WriteString("```\n\n")

	b.WriteString("## Project structure\n\n```\n")
	b.WriteString(c.FileTree)
	b.WriteString("```\n\n")

	if len(c.KeyFiles) > 0 {
		b.WriteString("## Key files detected\n\n")
		for _, f := range c.KeyFiles {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
		b.WriteString("\n")
	}

	if len(c.ExistingDockerfiles) > 0 {
		b.WriteString("## Existing Dockerfiles\n\n")
		for _, f := range c.ExistingDockerfiles {
			content, err := os.ReadFile(f)
			if err == nil {
				b.WriteString(fmt.Sprintf("### %s\n\n```dockerfile\n%s```\n\n", f, string(content)))
			}
		}
	}

	// Existing compose files — critical for understanding what's already configured.
	if len(c.ExistingComposeFiles) > 0 {
		b.WriteString("## Existing compose files\n\n")
		for _, f := range c.ExistingComposeFiles {
			content, err := os.ReadFile(f)
			if err == nil {
				b.WriteString(fmt.Sprintf("### %s\n\n```yaml\n%s```\n\n", f, string(content)))
			}
		}
	}

	// Dependency files — critical for writing accurate Dockerfiles.
	// The agent must read these to know what packages to install and how.
	depFiles := []string{
		"pyproject.toml", "requirements.txt", "requirements-base.txt",
		"go.mod", "package.json", "Cargo.toml", "Gemfile",
	}
	for _, f := range depFiles {
		content, err := os.ReadFile(f)
		if err == nil {
			lang := depFileLang(f)
			b.WriteString(fmt.Sprintf("## %s\n\n```%s\n%s```\n\n", f, lang, string(content)))
		}
	}

	// entrypoint.sh — if it exists, show it so the agent uses it in the Dockerfile.
	if content, err := os.ReadFile("entrypoint.sh"); err == nil {
		b.WriteString("## entrypoint.sh\n\n```sh\n")
		b.WriteString(string(content))
		b.WriteString("```\n\n")
		b.WriteString("**IMPORTANT**: this project has an entrypoint.sh. The Dockerfile MUST include:\n")
		b.WriteString("```dockerfile\nCOPY entrypoint.sh .\nRUN chmod +x entrypoint.sh\nENTRYPOINT [\"./entrypoint.sh\"]\n```\n\n")
	}

	b.WriteString("## Stack\n\n")
	b.WriteString(fmt.Sprintf("- Language: %s %s\n", c.Stack, c.LanguageVersion))
	b.WriteString(fmt.Sprintf("- Active environment: %s\n", c.ActiveEnv))
	b.WriteString("\n")

	b.WriteString("## Services defined in pilot.yaml\n\n")
	data, _ := yaml.Marshal(c.Config.Services)
	b.WriteString("```yaml\n")
	b.WriteString(string(data))
	b.WriteString("```\n\n")

	// ── Managed services — critical section ───────────────────────────────────
	// This must appear BEFORE the "What is needed" section so the agent
	// knows which services to skip when generating compose files.
	c.writeManagedServicesSection(&b)

	// ── What is needed ────────────────────────────────────────────────────────
	needsWork := c.MissingDockerfile || len(c.MissingComposeEnvs) > 0
	if needsWork {
		b.WriteString("## What is needed\n\n")
	}

	if c.MissingDockerfile {
		c.writeDockerfileSection(&b)
	}

	if len(c.MissingComposeEnvs) > 0 {
		c.writeComposeSection(&b)
	}

	// ── Unconfigured targets warning ──────────────────────────────────────────
	var unconfiguredTargets []string
	for name, t := range c.Config.Targets {
		if t.Host == "" {
			unconfiguredTargets = append(unconfiguredTargets, name)
		}
	}
	sort.Strings(unconfiguredTargets)
	if len(unconfiguredTargets) > 0 {
		b.WriteString("\n## ⚠ Unconfigured deploy targets\n\n")
		b.WriteString("The following targets have no `host` set in pilot.yaml.\n")
		b.WriteString("`pilot deploy` will fail until these are filled in:\n\n")
		for _, name := range unconfiguredTargets {
			b.WriteString(fmt.Sprintf("- **%s** — set `targets.%s.host` to the VPS IP or hostname\n", name, name))
		}
		b.WriteString("\nAsk the user for the VPS IP, then update pilot.yaml or run `pilot setup --env <env>`.\n")
	}

	return b.String()
}

// ── Managed services section ──────────────────────────────────────────────────

// writeManagedServicesSection emits the managed services table when at least
// one service has hosting: managed. The agent MUST NOT generate compose blocks
// for these — they connect via environment variables only.
func (c *ProjectContext) writeManagedServicesSection(b *strings.Builder) {
	managed := c.managedServices()
	if len(managed) == 0 {
		return
	}

	b.WriteString("## Managed external services\n\n")
	b.WriteString("**CRITICAL** — the following services are hosted externally.\n")
	b.WriteString("Do **NOT** generate a compose service block, volume, or healthcheck for them.\n")
	b.WriteString("They connect via environment variables only.\n\n")

	b.WriteString("| Service name | Type | Provider | Expected env vars |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, entry := range managed {
		envVarStr := strings.Join(entry.envVars, ", ")
		if envVarStr == "" {
			envVarStr = "—"
		}
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | `%s` |\n",
			entry.name, entry.svcType, entry.provider, envVarStr))
	}

	b.WriteString("\n**Rules for managed services:**\n\n")
	b.WriteString("1. Never add them to any `docker-compose.*.yml`.\n")
	b.WriteString("2. The env vars listed above must be present in the `env_file` for each environment.\n")
	b.WriteString("3. App services MUST declare `env_file` in the compose so these vars are injected at runtime.\n")
	b.WriteString("4. Do not assume the service is on localhost — the connection URL comes from the env var.\n\n")
}

// managedServiceEntry holds resolved info about one managed service.
type managedServiceEntry struct {
	name     string
	svcType  string
	provider string
	envVars  []string
}

// managedServices returns all services with hosting: managed, with their resolved
// provider label and expected env vars from the catalog.
func (c *ProjectContext) managedServices() []managedServiceEntry {
	var entries []managedServiceEntry

	// Sort by service name for deterministic output.
	names := make([]string, 0, len(c.Config.Services))
	for n := range c.Config.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		// Use ServiceForEnv to pick up per-env overrides (e.g. managed in prod, container in dev)
		svc, _ := c.Config.ServiceForEnv(name, c.ActiveEnv)
		if svc.Hosting != config.HostingManaged {
			continue
		}

		// Resolve provider label and env vars from the catalog.
		providerLabel := svc.Provider
		var envVars []string

		if pDef, ok := catalog.GetProvider(svc.Type, svc.Provider); ok {
			if pDef.Label != "" {
				providerLabel = pDef.Label
			}
			envVars = pDef.EnvVars
		}

		// Fallback: use catalog env vars for the service type if provider unknown.
		if len(envVars) == 0 {
			envVars = catalog.EnvVarsFor(svc.Type, svc.Provider)
		}

		entries = append(entries, managedServiceEntry{
			name:     name,
			svcType:  svc.Type,
			provider: providerLabel,
			envVars:  envVars,
		})
	}
	return entries
}

// ── Dockerfile section ────────────────────────────────────────────────────────

func (c *ProjectContext) writeDockerfileSection(b *strings.Builder) {
	b.WriteString("### Dockerfile\n\n")
	b.WriteString("A Dockerfile is missing. Generate a **production-optimized** Dockerfile:\n\n")
	b.WriteString("- **Multi-stage build**: builder stage (full SDK) → final stage (minimal image)\n")
	b.WriteString("- **Minimal base image**: distroless/alpine/slim for the final stage\n")
	b.WriteString("  - Go     → `golang:<ver>-alpine` builder / `gcr.io/distroless/static` final\n")
	b.WriteString("  - Node   → `node:<ver>-alpine` builder / `node:<ver>-alpine` final (non-root)\n")
	b.WriteString("  - Python → `python:<ver>-slim` for both stages\n")
	b.WriteString("  - Rust   → `rust:<ver>-alpine` builder / `gcr.io/distroless/static` final\n")
	b.WriteString("- **Non-root user**: create a dedicated user in the final stage\n")
	b.WriteString("- **Layer caching**: copy dependency files first, then install, then copy source\n")
	b.WriteString("- **Explicit WORKDIR**: e.g. `WORKDIR /app`\n")
	b.WriteString("- **HEALTHCHECK**: HTTP or TCP probe appropriate for the stack\n")
	b.WriteString("- **No secrets**: never COPY .env files into the image\n")
	b.WriteString("- **Pinned tags**: no `:latest` base images\n\n")

	// ARG/ENV rules — this is the fixed version of the previous bug.
	c.writeBuildArgRules(b)

	b.WriteString("Call `pilot_generate_dockerfile` with the generated content.\n\n")
}

// writeBuildArgRules emits the correct ARG/ENV instructions for build-time vars.
//
// BUG FIXED: the previous version told the agent to follow ARG with ENV VAR=$ARG.
// That bakes an empty string into the image when --build-arg is not passed,
// which silently breaks all runtime env injection (env_file, secrets).
// The correct pattern is ARG only — the RUN command sees the ARG value directly.
func (c *ProjectContext) writeBuildArgRules(b *strings.Builder) {
	b.WriteString("**ARG vs ENV — build-time variables:**\n\n")
	b.WriteString("Any variable needed only at build time (compile/bundle step) MUST be declared as `ARG` **only**.\n")
	b.WriteString("**NEVER** follow a build-time `ARG` with `ENV VAR=$ARG`. Reason:\n\n")
	b.WriteString("- Docker makes `ARG` values available as process env to every `RUN` command in the same stage.\n")
	b.WriteString("  So `RUN npm run build`, `RUN go build`, `RUN cargo build` all see `ARG` values directly.\n")
	b.WriteString("- If you also write `ENV VAR=$ARG`, Docker bakes the value (or `\"\"` when no `--build-arg` was\n")
	b.WriteString("  passed) permanently into the image layer. That empty string then overrides every runtime\n")
	b.WriteString("  source (env_file, .env files, secrets) because process env has the highest priority in\n")
	b.WriteString("  every major framework (Vite, Next.js, CRA, SvelteKit, Django, Go, Rust, Python…).\n\n")
	b.WriteString("✅ **CORRECT** — build-time only, nothing baked into the image:\n")
	b.WriteString("```dockerfile\n")
	b.WriteString("ARG NEXT_PUBLIC_API_URL\n")
	b.WriteString("ARG VITE_APP_ENV\n")
	b.WriteString("RUN npm run build   # ARG values are visible here as process env\n")
	b.WriteString("```\n\n")
	b.WriteString("❌ **WRONG** — bakes `\"\"` into the image, silently breaks all envs:\n")
	b.WriteString("```dockerfile\n")
	b.WriteString("ARG NEXT_PUBLIC_API_URL\n")
	b.WriteString("ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL   # ← NEVER DO THIS\n")
	b.WriteString("```\n\n")
	b.WriteString("Exception: if a var is needed at container **runtime** (not build time),\n")
	b.WriteString("declare it with `ENV` and a safe default, e.g. `ENV PORT=8080`.\n\n")

	// If build_args are declared in pilot.yaml, list them explicitly.
	if len(c.Config.Registry.BuildArgs) > 0 {
		b.WriteString("**Build args declared in pilot.yaml** (declare as `ARG` only — no `ENV`):\n\n")
		b.WriteString("```dockerfile\n")
		for _, arg := range c.Config.Registry.BuildArgs {
			b.WriteString(fmt.Sprintf("ARG %s\n", arg))
		}
		b.WriteString("RUN npm run build   # or your build command — ARGs are visible here\n")
		b.WriteString("```\n\n")
	}
}

// ── Compose section ───────────────────────────────────────────────────────────

func (c *ProjectContext) writeComposeSection(b *strings.Builder) {
	// If multiple envs are missing, ask the agent to generate all of them.
	if len(c.MissingComposeEnvs) > 1 {
		b.WriteString("### docker-compose files — all environments\n\n")
		b.WriteString("The following compose files are **all missing**. Generate each one by calling\n")
		b.WriteString("`pilot_generate_compose` once per environment:\n\n")
		for _, env := range c.MissingComposeEnvs {
			b.WriteString(fmt.Sprintf("- `docker-compose.%s.yml`  (env: `%s`)\n", env, env))
		}
		b.WriteString("\n")
	} else {
		env := c.MissingComposeEnvs[0]
		b.WriteString(fmt.Sprintf("### docker-compose.%s.yml\n\n", env))
	}

	b.WriteString("Generate a **production-optimized** docker-compose file for each missing env.\n\n")

	b.WriteString("**Rules:**\n\n")
	b.WriteString("- **Named volumes** for all persistent data (databases, uploads)\n")
	b.WriteString("- **Custom bridge network** — do not rely on the default network\n")
	b.WriteString("- **Resource limits** (`mem_limit`, `cpus`) on every service\n")
	b.WriteString("- **Restart policy**: `restart: unless-stopped` for all long-lived services\n")
	b.WriteString("- **Health checks + ordered startup**: `healthcheck` blocks + `depends_on: condition: service_healthy`\n")

	// Inject env_file hints for all configured environments.
	c.writeEnvFileRules(b)

	// Stack-specific commands.
	if c.Stack == "node" {
		b.WriteString("- **Node/Vite `--mode` flag**: the start command MUST include `--mode <env>`\n")
		b.WriteString("  e.g. `npx vite --host 0.0.0.0 --port 8080 --mode dev`\n")
		b.WriteString("  Without `--mode`, Vite defaults to `.env.development` and ignores `.env.<env>`.\n")
	}

	b.WriteString("- **Dev compose**: `build: context` + source mounts for live reload are acceptable\n")
	b.WriteString("- **Non-dev compose**: pre-built images only — `image: <registry>/<name>:<tag>` — no `build:` blocks\n")
	b.WriteString("- **Logging limits**: `logging: driver: json-file` with `max-size: 10m, max-file: 3`\n")
	b.WriteString("- **Pinned image tags**: never use `:latest` for external images\n")
	b.WriteString("- **Service names**: use the exact names from the pilot.yaml `services:` section\n\n")

	// Remind agent about managed services.
	if managed := c.managedServices(); len(managed) > 0 {
		names := make([]string, 0, len(managed))
		for _, m := range managed {
			names = append(names, "`"+m.name+"`")
		}
		b.WriteString(fmt.Sprintf("**Reminder**: %s %s managed externally — skip them in the compose file entirely.\n",
			strings.Join(names, ", "),
			oneOrMany(len(names), "is", "are"),
		))
		b.WriteString("See the **Managed external services** section above.\n\n")
	}

	b.WriteString("Call `pilot_generate_compose` with the content and the `env` parameter set to the environment name.\n\n")
}

// writeEnvFileRules emits the env_file injection rules for all configured envs.
func (c *ProjectContext) writeEnvFileRules(b *strings.Builder) {
	b.WriteString("- **env_file (MANDATORY)**: add `env_file` to every app service — required for ALL stacks and ALL envs.\n")
	b.WriteString("  Match the env_file to the environment:\n")
	for _, envName := range sortedKeys(c.Config.Environments) {
		envCfg := c.Config.Environments[envName]
		if envCfg.EnvFile != "" {
			b.WriteString(fmt.Sprintf("    - `%s` → `env_file: [%s]`\n", envName, envCfg.EnvFile))
		}
	}
	b.WriteString("  Never hardcode secret values in the compose file.\n")
}

// ── helpers ───────────────────────────────────────────────────────────────────

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".cache": true, "dist": true, "build": true, "__pycache__": true,
	".pilot-current-env": true,
}

func buildFileTree(dir string, depth, maxDepth int) string {
	if depth > maxDepth {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var lines []string
	prefix := strings.Repeat("  ", depth)

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && depth == 0 && name != ".env.example" {
			continue
		}
		if skipDirs[name] {
			continue
		}
		if e.IsDir() {
			lines = append(lines, prefix+name+"/")
			sub := buildFileTree(filepath.Join(dir, name), depth+1, maxDepth)
			if sub != "" {
				lines = append(lines, sub)
			}
		} else {
			lines = append(lines, prefix+name)
		}
	}
	return strings.Join(lines, "\n")
}

func scanKeyFiles(dir string) []string {
	candidates := []string{
		"go.mod", "go.sum", "package.json", "package-lock.json",
		"Cargo.toml", "requirements.txt", "pyproject.toml",
		"pom.xml", "build.gradle", "Makefile",
		"prisma/schema.prisma", "drizzle.config.ts", "drizzle.config.js",
	}
	var found []string
	for _, f := range candidates {
		if fileExists(filepath.Join(dir, f)) {
			found = append(found, f)
		}
	}
	sort.Strings(found)
	return found
}

func glob(dir, pattern string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	var results []string
	for _, m := range matches {
		results = append(results, filepath.Base(m))
	}
	return results
}

func globEnvFiles(dir string) []string {
	var files []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".env") {
			files = append(files, name)
		}
	}
	return files
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasCustomDockerfile(cfg *config.Config) bool {
	for _, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp && svc.Dockerfile != "" {
			return fileExists(svc.Dockerfile)
		}
	}
	return false
}

func sortedKeys(m map[string]config.Environment) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func oneOrMany(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// depFileLang returns the markdown language tag for a dependency file.
func depFileLang(filename string) string {
	switch filename {
	case "pyproject.toml", "Cargo.toml":
		return "toml"
	case "go.mod":
		return "go"
	case "package.json":
		return "json"
	case "Gemfile":
		return "ruby"
	default:
		return ""
	}
}
