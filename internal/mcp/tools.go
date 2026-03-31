package mcp

// registerAll wires up all MCP tools to their handlers.
func (s *Server) registerAll() {
	// Context + infra generation — primary AI agent workflow
	s.Register(toolContext, handleContext)
	s.Register(toolGenerateDockerfile, handleGenerateDockerfile)
	s.Register(toolGenerateCompose, handleGenerateCompose)

	// Environment lifecycle
	s.Register(toolInit, handleInit)
	s.Register(toolEnvSwitch, handleEnvSwitch)
	s.Register(toolUp, handleUp)
	s.Register(toolDown, handleDown)

	// Registry + deployment
	s.Register(toolPush, handlePush)
	s.Register(toolDeploy, handleDeploy)
	s.Register(toolRollback, handleRollback)
	s.Register(toolSync, handleSync)

	// Observability
	s.Register(toolStatus, handleStatus)
	s.Register(toolLogs, handleLogs)

	// Config + secrets
	s.Register(toolConfigGet, handleConfigGet)
	s.Register(toolConfigSet, handleConfigSet)
	s.Register(toolSecretsInject, handleSecretsInject)

	// VPS setup
	s.Register(toolSetup, handleSetup)

	// Preflight — call this before push/deploy
	s.Register(toolPreflight, handlePreflight)
}

// ──────────────────── context + infra generation ────────────────────

var toolContext = Tool{
	Name: "pilot_context",
	Description: `Return the complete project context for this pilot project.
Call this FIRST before generating any infrastructure files.
Returns: pilot.yaml, file tree, detected stack, existing Dockerfiles/compose files,
service definitions, missing file list, and a ready-to-use agent_prompt.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment to collect context for (defaults to active env)"},
		},
	},
}

var toolGenerateDockerfile = Tool{
	Name: "pilot_generate_dockerfile",
	Description: `Write a Dockerfile to the project directory.
Call pilot_context first to understand the project stack and requirements.
The agent is responsible for generating the Dockerfile content — pilot writes it to disk.

OPTIMIZATION REQUIREMENTS — every Dockerfile you generate MUST follow these rules:

1. Multi-stage build: use a builder stage (full SDK) + a final runtime stage (minimal image).
2. Minimal base image: use distroless, alpine, or slim variants for the final stage.
   - Go   → builder: golang:<version>-alpine  → final: gcr.io/distroless/static or scratch
   - Node → builder: node:<version>-alpine    → final: node:<version>-alpine (non-root)
   - Python → builder: python:<version>-slim  → final: python:<version>-slim
   - Rust → builder: rust:<version>-alpine    → final: gcr.io/distroless/static or scratch
3. Non-root user: create and switch to a dedicated user in the final stage (e.g. USER nonroot or adduser app).
4. Layer caching: copy dependency files (go.mod/go.sum, package.json, requirements.txt, Cargo.toml)
   and run install BEFORE copying source code to maximize cache reuse.
5. WORKDIR: always set an explicit WORKDIR (e.g. /app).
6. HEALTHCHECK: add a HEALTHCHECK instruction appropriate for the stack (HTTP endpoint or TCP probe).
7. Read-only filesystem: avoid writing to the container filesystem at runtime where possible.
8. No secrets in image: never COPY .env files or secret files into the image.
9. Pinned versions: use pinned base image tags (not :latest).
10. Single responsibility: one process per container; use CMD not ENTRYPOINT+CMD unless an entrypoint script is needed.
11. ARG vs ENV for build-time vars — applies to ALL stacks, not just Vite:
    Any var that is only needed at build time (compile/bundle step) must be declared as ARG only.
    NEVER follow a build-time ARG with ENV VAR=$ARG. Here is why:
    - Docker ARG vars are available as process env to every RUN command in the same stage.
      So "RUN npm run build", "RUN go build", "RUN python setup.py" etc. all see ARG vars directly.
    - If you also write "ENV VAR=$ARG", Docker bakes the value (or "" if no --build-arg was
      passed) permanently into the image layer. Every container started from that image will have
      VAR="" in its process env. That empty string overrides any runtime source (env_file, .env
      files, secrets) because process env has highest priority in every major framework:
        Vite      → process env beats .env.* files
        Next.js   → process env beats .env.* files  (NEXT_PUBLIC_* same issue)
        CRA       → process env beats .env.* files  (REACT_APP_* same issue)
        SvelteKit → process env beats .env.* files  (PUBLIC_* same issue)
        Nuxt      → process env beats .env.* files  (NUXT_PUBLIC_* same issue)
        Angular   → ng build reads process env; empty var baked into bundle
        Go/Rust   → ldflags / build tags read from env; "" gets compiled in
        Python    → os.environ reads process env first
    CORRECT pattern (build-time only — nothing baked into the image):
      ARG NEXT_PUBLIC_API_URL
      ARG VITE_APP_ENV
      ARG REACT_APP_FEATURE_FLAG
      RUN npm run build       # or: RUN go build, RUN cargo build, etc.
    WRONG pattern (bakes "" into the image, silently breaks all envs that skip --build-arg):
      ARG NEXT_PUBLIC_API_URL
      ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL   # ← NEVER DO THIS
    Exception: if a var is genuinely needed at container RUNTIME (not just at build time),
    declare it with ENV and a safe default (e.g. ENV PORT=8080), or inject it via env_file.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"content": {Type: "string", Description: "Full Dockerfile content to write"},
			"path":    {Type: "string", Description: "Destination path (default: Dockerfile)"},
		},
		Required: []string{"content"},
	},
}

var toolGenerateCompose = Tool{
	Name: "pilot_generate_compose",
	Description: `Write a docker-compose.<env>.yml to the project directory.
Call pilot_context first to understand the services and environment configuration.
The agent is responsible for generating the compose file content — pilot writes it to disk.
After writing, the agent should tell the user to run 'pilot up'.

OPTIMIZATION REQUIREMENTS — every docker-compose file you generate MUST follow these rules:

1. Named volumes: use named volumes (not bind mounts) for database data and persistent state.
2. Explicit networks: define a custom bridge network; do not rely on the default network.
3. Resource limits: set mem_limit and cpus for every service to prevent runaway containers.
4. Restart policy: use restart: unless-stopped for all long-lived services.
5. Health checks: add healthcheck blocks for every service, especially databases.
   Depend on health: use depends_on with condition: service_healthy so services start in order.
6. env_file injection: ALWAYS add env_file to EVERY app service — MANDATORY for all stacks,
   all environments (dev, staging, prod). Read the env_file from pilot.yaml:
   (environments.<env>.env_file) and inject it:
     env_file:
       - .env.dev   # match the environment: .env.dev / .env.staging / .env.prod
   WHY this is mandatory for every stack and every env:
   Docker images often contain empty process env vars (e.g. from ARG/ENV patterns, base images,
   or previous layers). Process env has the highest priority in every framework and runtime:
   it overrides .env files (Vite, Next.js, CRA, SvelteKit, Nuxt), config files (Rails, Django,
   Laravel), and SDK defaults (AWS SDK, GCP SDK read env first). If a var is "" in process env,
   the framework uses that "" — not the value in the .env file on disk.
   env_file in docker-compose sets the correct values at container startup, overriding whatever
   the image has. This is the single reliable source of truth for runtime config.
   This applies to: Node/Vite/Next.js/CRA/SvelteKit/Nuxt (VITE_*, NEXT_PUBLIC_*, REACT_APP_*,
   PUBLIC_*, NUXT_PUBLIC_*), Go binaries (env-based config), Python (os.environ), Ruby, PHP, Rust.
   Never hardcode secret values in the compose file; always rely on env_file.
7. Framework-specific start commands: match the dev server command to the framework:
   - Vite        → npx vite --host 0.0.0.0 --port <port> --mode <env>
                   (--mode is required: without it Vite reads .env.development, not .env.dev)
   - Next.js dev → node_modules/.bin/next dev -p <port>
                   (reads .env.development or .env.local by default in dev mode)
   - CRA dev     → node_modules/.bin/react-scripts start
   - Angular dev → node_modules/.bin/ng serve --host 0.0.0.0 --port <port>
   - Nuxt dev    → node_modules/.bin/nuxt dev --host 0.0.0.0 --port <port>
   In ALL cases, env_file (rule 6) is still required — framework file-based loading is
   a fallback; process env (set by env_file) is authoritative.
8. Read-only app containers: where feasible add read_only: true + tmpfs for /tmp.
9. No build in prod compose: production compose files should reference pre-built images
   (image: <registry>/<name>:<tag>) not build: context.
10. Dev compose: dev files may use build: context and volume mounts for live reload.
11. Logging: configure logging driver with max-size and max-file to avoid disk exhaustion.
    Example: logging: { driver: json-file, options: { max-size: "10m", max-file: "3" } }
12. Service naming: use the exact service names from pilot.yaml services section.
13. Pinned image tags: never use :latest for external images.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"content": {Type: "string", Description: "Full docker-compose YAML content to write"},
			"env":     {Type: "string", Description: "Environment name (default: active env) — determines filename docker-compose.<env>.yml"},
			"path":    {Type: "string", Description: "Override destination path (optional)"},
		},
		Required: []string{"content"},
	},
}

// ──────────────────────── environment lifecycle ────────────────────────

var toolInit = Tool{
	Name:        "pilot_init",
	Description: "Initialize a new pilot project with scaffold, Dockerfiles, and pilot.yaml",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"name":         {Type: "string", Description: "Project name"},
			"stack":        {Type: "string", Description: "Language stack", Enum: []string{"go", "node", "python", "rust"}},
			"registry":     {Type: "string", Description: "Registry provider", Enum: []string{"ghcr", "dockerhub", "custom"}},
			"envs":         {Type: "string", Description: "Comma-separated list of environments (default: dev,staging,prod)"},
			"orchestrator": {Type: "string", Description: "Orchestrator type", Enum: []string{"compose", "k8s"}},
		},
		Required: []string{"name", "stack"},
	},
}

var toolEnvSwitch = Tool{
	Name:        "pilot_env_switch",
	Description: "Switch the active pilot environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Target environment name (e.g. dev, staging, prod)"},
		},
		Required: []string{"env"},
	},
}

var toolUp = Tool{
	Name:        "pilot_up",
	Description: "Start local services for the active environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment to start (defaults to active env)"},
			"services": {Type: "string", Description: "Comma-separated list of services to start (defaults to all)"},
		},
	},
}

var toolDown = Tool{
	Name:        "pilot_down",
	Description: "Stop and remove local services for the active environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment to stop (defaults to active env)"},
		},
	},
}

var toolPush = Tool{
	Name: "pilot_push",
	Description: `Build the Docker image and push it to the configured registry. Defaults to linux/amd64 for VPS compatibility.

If registry.build_args is declared in pilot.yaml, the values are read from the active
environment's env_file and injected as --build-arg at build time. This is required for
frontend stacks (Vite, Next.js, CRA) where VITE_* / NEXT_PUBLIC_* variables must be
baked into the bundle during 'npm run build'. Pass 'env' to select which env file to read.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment whose env_file is used to resolve build_args (defaults to active env)"},
			"tag":      {Type: "string", Description: "Image tag (defaults to git short SHA)"},
			"no_cache": {Type: "string", Description: "Disable build cache (true/false)"},
			"platform": {Type: "string", Description: "Target platform (default: linux/amd64). Use linux/arm64 for ARM VPS, linux/amd64,linux/arm64 for multi-arch."},
		},
	},
}

var toolDeploy = Tool{
	Name:        "pilot_deploy",
	Description: "Deploy the application to a remote target (VPS or cloud)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment to deploy (defaults to active env)"},
			"tag":      {Type: "string", Description: "Image tag to deploy (defaults to git short SHA)"},
			"target":   {Type: "string", Description: "Target name from pilot.yaml (overrides env default)"},
			"strategy": {Type: "string", Description: "Deployment strategy", Enum: []string{"rolling", "blue-green", "canary"}},
			"dry_run":  {Type: "string", Description: "Show what would happen without executing (true/false)"},
		},
	},
}

var toolRollback = Tool{
	Name:        "pilot_rollback",
	Description: "Roll back to a previous deployment version",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":     {Type: "string", Description: "Environment to roll back"},
			"target":  {Type: "string", Description: "Target name"},
			"version": {Type: "string", Description: "Version tag to roll back to (defaults to previous)"},
		},
		Required: []string{"env"},
	},
}

var toolSync = Tool{
	Name:        "pilot_sync",
	Description: "Synchronize local pilot.yaml and compose files to the remote target",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"target": {Type: "string", Description: "Target name from pilot.yaml"},
		},
	},
}

var toolStatus = Tool{
	Name:        "pilot_status",
	Description: "Return the complete project state as JSON (local containers + remote services)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Filter by environment (optional)"},
		},
	},
}

var toolLogs = Tool{
	Name:        "pilot_logs",
	Description: "Return logs for a service (local or remote based on active env)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"service": {Type: "string", Description: "Service name"},
			"lines":   {Type: "string", Description: "Number of lines to return (default 100)"},
			"since":   {Type: "string", Description: "Return logs since this duration (e.g. 5m, 1h)"},
		},
	},
}

var toolConfigGet = Tool{
	Name:        "pilot_config_get",
	Description: "Read a value from pilot.yaml using dot-notation key",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"key": {Type: "string", Description: "Dot-notation key (e.g. project.name, registry.provider)"},
		},
		Required: []string{"key"},
	},
}

var toolConfigSet = Tool{
	Name:        "pilot_config_set",
	Description: "Set a value in pilot.yaml using dot-notation key",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"key":   {Type: "string", Description: "Dot-notation key"},
			"value": {Type: "string", Description: "New value"},
		},
		Required: []string{"key", "value"},
	},
}

var toolPreflight = Tool{
	Name: "pilot_preflight",
	Description: `Run pre-flight checks for the deployment pipeline and return an ordered action plan.

CALL THIS FIRST before pilot_push or pilot_deploy. It returns a structured report with:
- all_ok: true/false — whether all checks pass
- checks[]: each check with status (ok/error) and fix instructions
- next_steps[]: ordered action plan tagged [HUMAN] or [AGENT]

For each [HUMAN] step: tell the user exactly what to do and wait for confirmation.
For each [AGENT] step: call the indicated tool directly.
Repeat until all_ok is true, then proceed with pilot_push / pilot_deploy.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"target": {Type: "string", Description: "Operation to check for: up | push | deploy (default: deploy)", Enum: []string{"up", "push", "deploy"}},
			"env":    {Type: "string", Description: "Environment (defaults to active env)"},
		},
	},
}

var toolSetup = Tool{
	Name: "pilot_setup",
	Description: `Run one-time VPS setup tasks required before the first deploy.
Connects via SSH and adds the deploy user to the docker group.
Call this when pilot_deploy fails with a docker permission error.
Requires password-less sudo on the VPS (standard on Hetzner, DigitalOcean, OVH with cloud-init).`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment whose target VPS to configure (defaults to active env)"},
		},
	},
}

var toolSecretsInject = Tool{
	Name:        "pilot_secrets_inject",
	Description: "Inject secrets from the configured secret manager into the target environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment (dev, staging, prod)"},
			"provider": {Type: "string", Description: "Secret provider override (local, aws_sm, gcp_sm)"},
		},
		Required: []string{"env"},
	},
}
