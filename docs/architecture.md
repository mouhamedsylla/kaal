# pilot : Architecture

> This document is the authoritative technical reference for pilot's internals.
> If you are contributing, this is where to start. If you are debugging something deep,
> this is where to look. It covers every layer, every contract, and every invariant.

---

## Table of contents

1. [Design philosophy](#design-philosophy)
2. [Directory layout](#directory-layout)
3. [Dependency rule](#dependency-rule)
4. [Domain layer](#domain-layer)
   - [Ports (interfaces)](#ports-interfaces)
   - [Error taxonomy](#error-taxonomy)
   - [State machine](#state-machine)
   - [pilot.lock](#pilotlock)
   - [Execution plan](#execution-plan)
5. [Adapters layer](#adapters-layer)
6. [App layer (use cases)](#app-layer-use-cases)
7. [Runtime wiring](#runtime-wiring)
8. [The deploy pipeline](#the-deploy-pipeline)
9. [TypeC suspension / resume cycle](#typec-suspension--resume-cycle)
10. [MCP server](#mcp-server)
11. [Code conventions](#code-conventions)

---

## Design philosophy

pilot uses **hexagonal architecture** (also called ports & adapters). The idea is simple:
business logic lives in the center, I/O details live at the edges, and the center
never depends on the edges.

In practice this means:

- **`domain/`** defines what the application needs from the outside world (interfaces,
  types, error taxonomy, state machine). It imports nothing from the rest of the codebase.
- **`adapters/`** implements those interfaces (Docker Compose, SSH/VPS, GHCR, ...).
  They import `domain/` but `domain/` never imports them.
- **`app/`** contains the use cases (deploy, preflight, push, ...). They depend on
  `domain/` interfaces : never on concrete adapters. This makes every use case unit-testable
  with plain Go mocks, no Docker, no SSH required.
- **`cmd/`** and **`mcp/handlers/`** wire everything together via `app/runtime`.

The reward: adding a new cloud provider is `adapters/azure2/` + two lines in `runtime.go`.
The deploy use case never changes.

---

## Directory layout

```
pilot/
├── main.go                         # Entry point: calls cmd.Execute()
├── cmd/                            # Cobra commands : one file per command
│   ├── root.go                     # Global flags, error rendering (TypeC/TypeD)
│   ├── deploy.go                   # pilot deploy
│   ├── preflight.go                # pilot preflight
│   ├── push.go                     # pilot push
│   ├── up.go / down.go             # pilot up / down
│   ├── sync.go                     # pilot sync
│   ├── rollback.go                 # pilot rollback
│   ├── plan.go                     # pilot plan
│   ├── diagnose.go                 # pilot diagnose
│   ├── resume.go                   # pilot resume
│   ├── status.go                   # pilot status
│   ├── logs.go                     # pilot logs
│   ├── env.go                      # pilot env use/current/diff
│   ├── secrets.go                  # pilot secrets list/get/set/inject
│   └── init.go                     # pilot init (TUI wizard)
│
├── internal/
│   ├── domain/                     # The core : imports nothing internal
│   │   ├── ports.go                # All domain interfaces (4 ports)
│   │   ├── errors/                 # TypeA/B/C/D error taxonomy
│   │   ├── state/                  # State machine + .pilot/state.json
│   │   ├── lock/                   # pilot.lock read/write + staleness check
│   │   └── plan/                   # Step names (StepPreflight, StepDeploy, ...)
│   │
│   ├── adapters/                   # Port implementations : import domain/, nothing else
│   │   ├── compose/                # ExecutionProvider → Docker Compose
│   │   ├── k8s/                    # ExecutionProvider → Kubernetes (stub)
│   │   ├── vps/                    # DeployProvider + HookRunner + MigrationRunner → SSH
│   │   ├── aws/                    # DeployProvider → AWS (stub)
│   │   ├── azure/                  # DeployProvider → Azure (stub)
│   │   ├── gcp/                    # DeployProvider → GCP (stub)
│   │   ├── do/                     # DeployProvider → DigitalOcean (stub)
│   │   ├── registry/
│   │   │   ├── imgbuild/           # Shared docker build/buildx logic
│   │   │   ├── ghcr/               # RegistryProvider → GitHub Container Registry
│   │   │   ├── dockerhub/          # RegistryProvider → Docker Hub
│   │   │   ├── custom/             # RegistryProvider → any registry with basic auth
│   │   │   ├── ecr/                # RegistryProvider → AWS ECR (stub)
│   │   │   ├── gcr/                # RegistryProvider → Google GCR (stub)
│   │   │   └── acr/                # RegistryProvider → Azure ACR (stub)
│   │   └── secrets/
│   │       ├── local/              # SecretManager → .env files
│   │       ├── aws_sm/             # SecretManager → AWS Secrets Manager (stub)
│   │       └── gcp_sm/             # SecretManager → GCP Secret Manager (stub)
│   │
│   ├── app/                        # Use cases : depend on domain/ interfaces only
│   │   ├── runtime/                # Factory: reads pilot.yaml, builds concrete ports
│   │   ├── deploy/                 # DeployUseCase : 7-step pipeline
│   │   ├── preflight/              # PreflightUseCase + lock generation
│   │   ├── push/                   # PushUseCase
│   │   ├── up/                     # UpUseCase
│   │   ├── sync/                   # SyncUseCase
│   │   ├── rollback/               # RollbackUseCase
│   │   ├── status/                 # StatusUseCase
│   │   ├── logs/                   # LogsUseCase
│   │   ├── resume/                 # TypeC resume cycle
│   │   ├── envdiff/                # env diff use case
│   │   ├── diagnose/               # System snapshot use case
│   │   └── planview/               # Render execution plan from pilot.lock
│   │
│   ├── config/                     # Parse + validate pilot.yaml
│   ├── env/                        # Active environment (.pilot-current-env)
│   ├── scaffold/                   # pilot init TUI wizard + project generation
│   ├── gitutil/                    # Git helpers (branch, status, log)
│   ├── version/                    # Build-time version injection
│   └── mcp/                        # MCP JSON-RPC 2.0 stdio server
│       ├── server.go               # Read loop + dispatch
│       ├── tools.go                # Tool schema definitions
│       ├── handlers.go             # Wiring
│       └── handlers/               # One handler per tool
│
└── pkg/                            # Exported utilities (no internal imports)
    ├── ui/                         # Terminal output (colors, spinner, JSON)
    └── ssh/                        # SSH client (golang.org/x/crypto/ssh)
```

---

## Dependency rule

The dependency graph has one invariant: **arrows always point inward**.

```
cmd/ ──────────────────────────────────────────────┐
mcp/handlers/ ────────────────────────────────────►│
                                                    │
                                            app/runtime/
                                                    │
                              ┌─────────────────────┤
                              ▼                     ▼
                         app/deploy/           adapters/vps/
                         app/preflight/        adapters/compose/
                         app/push/             adapters/ghcr/
                         ...                   ...
                              │                     │
                              └──────────┬──────────┘
                                         ▼
                                     domain/
                                  ports.go
                                  errors/
                                  state/
                                  lock/
                                  plan/
```

`domain/` is the innermost ring. It knows nothing about Docker, SSH, or files.
Everything else knows about `domain/` and implements (or uses) its interfaces.

The only package allowed to import both adapters and use cases is `app/runtime`.
`cmd/` and `mcp/handlers/` import `app/runtime` : never adapters directly.

---

## Domain layer

### Ports (interfaces)

All four contracts live in `internal/domain/ports.go`.

```go
// ExecutionProvider : local runtime (Docker Compose, Podman, k3d, ...)
type ExecutionProvider interface {
    Up(ctx context.Context, env string, services []string) error
    Down(ctx context.Context, env string) error
    Status(ctx context.Context, env string) ([]ServiceStatus, error)
    Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}

// DeployProvider : remote target (VPS, AWS, ...)
type DeployProvider interface {
    Sync(ctx context.Context, env string) error
    Deploy(ctx context.Context, env string, opts DeployOptions) error
    Rollback(ctx context.Context, env string, toTag string) (restoredTag string, err error)
    Status(ctx context.Context, env string) ([]ServiceStatus, error)
    Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}

// RegistryProvider : image build + push
type RegistryProvider interface {
    Login(ctx context.Context) error
    Build(ctx context.Context, opts BuildOptions) error
    Push(ctx context.Context, tag string) error
}

// SecretManager : secret resolution
type SecretManager interface {
    Inject(ctx context.Context, env string, refs map[string]string) (map[string]string, error)
}

// HookRunner : pre/post-deploy hooks (remote SSH for VPS)
type HookRunner interface {
    RunHooks(ctx context.Context, commands []string) error
}

// MigrationRunner : schema changes with optional rollback
type MigrationRunner interface {
    RunMigrations(ctx context.Context, cfg MigrationConfig) error
    RollbackMigrations(ctx context.Context, cfg MigrationConfig) error
}
```

Note that `HookRunner` and `MigrationRunner` are not mandatory ports: the deploy use
case injects them as optional `nil`-able fields. Steps that require them are silently
skipped when the runner is nil. For VPS targets, `app/runtime` provides adapters that
bridge the concrete `*vps.Provider` to these interfaces.

### Error taxonomy

`internal/domain/errors/` defines the four-type taxonomy. Every pilot failure is one
of these four types : the type determines **who acts** and **how**, not just what broke.

| Type | Situation | Actor | Mechanism |
|------|-----------|-------|-----------|
| **TypeA** | Deterministic, low-risk (e.g. missing `.pilot/` dir) | pilot, silently | auto-fix, log, continue |
| **TypeB** | Deterministic, impactful (e.g. stale lock auto-regenerated) | pilot, announced | auto-fix, print what it did, `--dry-run` safe |
| **TypeC** | Choice required, options known (e.g. user not in docker group) | human or agent | pilot suspends, presents numbered options, waits |
| **TypeD** | Choice required, options unknown (e.g. SSH key rejected) | human only | pilot stops with step-by-step instructions |

The `PilotError` struct:

```go
type PilotError struct {
    Type         ErrorType  // TypeA | TypeB | TypeC | TypeD
    Code         string     // e.g. "PILOT-SSH-001"
    Message      string
    Exit         int        // os.Exit code (use ExitXxx constants)
    Cause        error      // wrapped cause for errors.Is / errors.As chain

    // TypeC only
    Options     []string   // numbered choices
    Recommended string     // recommended option

    // TypeD only
    Instructions string    // exact steps for the human
}
```

Error codes follow the pattern `PILOT-<DOMAIN>-<NNN>`:

| Code | Type | Trigger |
|------|------|---------|
| `PILOT-CFG-001` | D | `pilot.yaml` not found |
| `PILOT-CFG-002` | D | `pilot.yaml` invalid YAML |
| `PILOT-CFG-003` | D | `pilot.yaml` validation failure |
| `PILOT-SSH-001` | D | SSH connection refused / timeout |
| `PILOT-DEPLOY-002` | D | `docker pull` failure on remote |
| `PILOT-DEPLOY-003` | C | user not in `docker` group on VPS |
| `PILOT-DEPLOY-004` | D | `docker compose up` failure on remote |
| `PILOT-DEPLOY-005` | D | image rollback failure |
| `PILOT-DEPLOY-006` | D | migration rollback failure |

Exit codes (stable across releases, safe in CI):

```go
const (
    ExitOK       = 0
    ExitGeneral  = 1
    ExitConfig   = 2
    ExitDeploy   = 3
    ExitSecrets  = 4
    ExitSSH      = 5
    ExitRegistry = 6
    ExitNotFound = 7
)
```

`cmd/root.go` uses `errors.As` to detect `*PilotError` and renders TypeC and TypeD
distinctively. TypeC additionally calls `resume.SaveSuspension()` to write
`.pilot/suspended.json` so `pilot resume` can restart the operation.

### State machine

`internal/domain/state/` implements the state machine and persists it to `.pilot/state.json`.

```
StateIdle
  │
  ▼  (EventStart)
StatePreflighting ─── EventTypeAB ──► StateRecovering
  │                                       │
  │   EventTypeC ──► StateAwaitingChoice ◄┤ EventTypeC
  │                       │               │
  │   EventOK             │ EventResume   │ EventResume
  ▼                       └──────────────►┘
StateExecuting ── EventTypeAB ──► StateRecovering
  │
  ├─ EventTypeC ──► StateAwaitingChoice
  │
  ├─ EventTypeD ──► StateGuidedFailure  (terminal)
  │
  └─ EventOK    ──► StateSucceeded      (terminal)
```

**Invariant:** every operation ends in `StateSucceeded` or `StateGuidedFailure`.
pilot never leaves a project in an ambiguous state.

`state.State` is the full JSON structure persisted at `.pilot/state.json`:

```go
type State struct {
    SchemaVersion         int
    ActiveEnv             string
    MachineState          MachineState
    LastOperation         *OperationRecord
    LastSuccessPerCommand map[string]time.Time
    PendingChoice         *PendingChoice    // non-nil when TypeC suspended
    Deployed              map[string]DeployRecord  // keyed by env
    KnownContainers       map[string][]string
}
```

`state.json` is written atomically (write temp → rename). `.pilot/` is in `.gitignore`.

### pilot.lock

`internal/domain/lock/` manages `pilot.lock` : the validated, committed snapshot of
what the next deploy will do.

**Lifecycle:**

```
pilot preflight --target deploy
    │
    ├─ reads: pilot.yaml, docker-compose.prod.yml, prisma/schema.prisma, ...
    ├─ computes SHA-256 of those files (GeneratedFrom)
    └─ writes: pilot.lock  ← commit this

pilot deploy
    │
    ├─ reads pilot.lock
    ├─ re-computes SHA-256 of GeneratedFrom files
    ├─ compares → stale? → STOP, run preflight again
    └─ executes the plan
```

`pilot.lock` in YAML:

```yaml
# pilot.lock : generated automatically, commit this file.
schema_version: 1
generated_at: 2026-04-11T14:00:00Z
generated_from:
  - pilot.yaml
  - docker-compose.prod.yml
  - prisma/schema.prisma
project_hash: "sha256:abc123..."

execution_plan:
  nodes_active: [preflight, migrations, deploy, healthcheck]
  migrations:
    tool: prisma
    command: npx prisma migrate deploy
    rollback_command: npx prisma migrate rollback
    reversible: true
    detected_from: prisma/schema.prisma
  service_order: [db, cache, api, proxy]
execution_provider: compose
```

`lock.IsStale(currentHash)` compares `ProjectHash` to a fresh SHA-256. If any source
file changed since preflight was run, the lock is stale and deploy refuses to continue.

### Execution plan

`internal/domain/plan/` defines the step names as typed constants:

```go
const (
    StepPreflight    StepName = "preflight"
    StepPreHooks     StepName = "pre_hooks"
    StepMigrations   StepName = "migrations"
    StepDeploy       StepName = "deploy"
    StepPostHooks    StepName = "post_hooks"
    StepHealthcheck  StepName = "healthcheck"
)
```

The `pilot.lock` stores which steps are active as `nodes_active`. The deploy use case
and `planview` use case both iterate over this set against the fixed skeleton to build
their step list.

---

## Adapters layer

All adapters live in `internal/adapters/`. Each implements one or more domain interfaces.
Stubs return `fmt.Errorf("xxx: not yet implemented")` : never silent `nil`.

### Execution providers

| Adapter | Interface | Status |
|---------|-----------|--------|
| `adapters/compose` | `ExecutionProvider` | implemented |
| `adapters/k8s` | `ExecutionProvider` | stub |

`compose.New(cfg, env)` reads `docker-compose.<env>.yml` and delegates all commands
to the `docker compose` CLI via `os/exec`. `Status()` parses `docker compose ps --json`.

### Deploy providers

| Adapter | Interface | Status | Notes |
|---------|-----------|--------|-------|
| `adapters/vps` | `DeployProvider` + `HookRunner` + `MigrationRunner` | implemented | SSH via `golang.org/x/crypto/ssh` |
| `adapters/aws` | `DeployProvider` | stub | |
| `adapters/gcp` | `DeployProvider` | stub | |
| `adapters/azure` | `DeployProvider` | stub | |
| `adapters/do` | `DeployProvider` | stub | |

`adapters/vps` is where the TypeA/B/C/D taxonomy is most visible in practice:

- SSH connection failure → `NewTypeD("PILOT-SSH-001", ...)` with SSH key/port instructions
- `docker pull` failure → `NewTypeD("PILOT-DEPLOY-002", ...)`
- User not in docker group → `NewTypeC("PILOT-DEPLOY-003", ...)` with two options:
  `["pilot setup --env X", "ssh ... usermod -aG docker deploy"]`
- `docker compose up` failure → `NewTypeD("PILOT-DEPLOY-004", ...)`

`vps.Provider` exposes three additional methods beyond `DeployProvider`:
`RunHooks(ctx, commands)`, `RunMigrations(ctx, tool, command)`,
`RollbackMigrations(ctx, tool, rollbackCmd)`. These are bridged to domain interfaces
by adapters in `app/runtime` (see [Runtime wiring](#runtime-wiring)).

### Registry providers

| Adapter | Interface | Status |
|---------|-----------|--------|
| `adapters/registry/ghcr` | `RegistryProvider` | implemented |
| `adapters/registry/dockerhub` | `RegistryProvider` | implemented |
| `adapters/registry/custom` | `RegistryProvider` | implemented |
| `adapters/registry/ecr` | `RegistryProvider` | stub |
| `adapters/registry/gcr` | `RegistryProvider` | stub |
| `adapters/registry/acr` | `RegistryProvider` | stub |

All registry adapters delegate `Build()` to `adapters/registry/imgbuild`:

- single platform → `docker build -t <tag> [--build-arg ...]`
- multi-platform → `docker buildx build --platform <...> --push`

### Secret managers

| Adapter | Interface | Status |
|---------|-----------|--------|
| `adapters/secrets/local` | `SecretManager` | implemented |
| `adapters/secrets/aws_sm` | `SecretManager` | stub |
| `adapters/secrets/gcp_sm` | `SecretManager` | stub |

`local.New()` resolves secrets from `.env` files on disk.
`Inject(ctx, env, refs)` maps logical names to actual values.

---

## App layer (use cases)

Each use case in `internal/app/` follows the same pattern:

```go
type UseCase struct {
    cfg  *config.Config
    port domain.SomePort  // injected
}

func New(cfg *config.Config, port domain.SomePort) *UseCase { ... }

func (uc *UseCase) Execute(ctx context.Context, in Input) (Output, error) { ... }
```

Ports are injected at construction time : unit tests pass fakes, `cmd/` passes
real adapters from `app/runtime`.

| Use case | Package | Key injected ports |
|----------|---------|-------------------|
| Deploy | `app/deploy` | `DeployProvider`, `SecretManager`, `HookRunner`, `MigrationRunner` |
| Preflight | `app/preflight` | `DeployProvider`, `ExecutionProvider` |
| Push | `app/push` | `RegistryProvider` |
| Up / Down | `app/up` | `ExecutionProvider` |
| Sync | `app/sync` | `DeployProvider` |
| Rollback | `app/rollback` | `DeployProvider` |
| Status | `app/status` | `DeployProvider`, `ExecutionProvider` |
| Logs | `app/logs` | `DeployProvider`, `ExecutionProvider` |
| Resume | `app/resume` | none (reads `.pilot/suspended.json`) |
| Env diff | `app/envdiff` | none (reads config + env files) |
| Diagnose | `app/diagnose` | none (probes system directly) |
| Plan view | `app/planview` | none (reads `pilot.lock`) |

---

## Runtime wiring

`internal/app/runtime/runtime.go` is the only file allowed to import both adapters
and use cases. It acts as a factory layer: given a `*config.Config`, it instantiates
the right concrete adapter and returns a typed domain interface.

```go
// Local runtime (compose, k8s)
func NewExecutionProvider(cfg *config.Config, env string) (domain.ExecutionProvider, error)

// Remote target (vps, aws, ...)
func NewDeployProvider(cfg *config.Config, targetName string) (domain.DeployProvider, error)

// Registry (ghcr, dockerhub, ...)
func NewRegistryProvider(cfg *config.Config) (domain.RegistryProvider, error)

// Secrets (local, aws_sm, ...)
func NewSecretManager(provider string) (domain.SecretManager, error)

// Pre/post-deploy hooks (VPS only for now)
func NewHookRunner(cfg *config.Config, targetName string) (domain.HookRunner, error)

// Database migrations (VPS only for now)
func NewMigrationRunner(cfg *config.Config, targetName string) (domain.MigrationRunner, error)
```

### The hook/migration bridge

`vps.Provider` has raw `RunHooks(ctx, []string)` and `RunMigrations(ctx, tool, cmd string)`
methods : not the exact signatures the domain interfaces require. `runtime.go` bridges
them with local duck-typed adapters:

```go
type hookRunnerAdapter struct{ inner hookRunnerIface }

func (a *hookRunnerAdapter) RunHooks(ctx context.Context, commands []string) error {
    return a.inner.RunHooks(ctx, commands)
}

type migrationRunnerAdapter struct{ inner migrationRunnerIface }

func (a *migrationRunnerAdapter) RunMigrations(ctx context.Context, cfg domain.MigrationConfig) error {
    return a.inner.RunMigrations(ctx, cfg.Tool, cfg.Command)
}
```

If the provider does not implement the duck-typed interface (e.g. AWS stub),
`NewHookRunner` / `NewMigrationRunner` returns `nil`. The deploy use case
skips those steps silently.

---

## The deploy pipeline

`app/deploy.DeployUseCase.Execute()` runs a 7-step skeleton. Each step only executes
if the corresponding node is active in `pilot.lock`.

```
Step 1  preflight     validate pilot.lock is not stale
Step 2  secrets       resolve refs → write temp env file
Step 3  sync          push compose + config files to remote (DeployProvider.Sync)
Step 4  pre_hooks     run pre-deploy commands on remote (HookRunner.RunHooks)
Step 5  migrations    apply schema changes (MigrationRunner.RunMigrations)
Step 6  deploy        docker pull + compose up (DeployProvider.Deploy)
Step 7  post_hooks    run post-deploy commands on remote (HookRunner.RunHooks)
Step 8  healthcheck   wait for all services healthy (DeployProvider.Status)
```

### LIFO compensation

Steps are marked as `completed` **before** they execute. This ensures that if a step
panics or returns mid-way, the compensation stack still includes it.

On any failure from step 4 onward, compensation runs in LIFO order:

```
failure in step 6 (deploy)
  → compensate: step 6 (image rollback : always)
  → compensate: step 5 (migration rollback : if reversible: true)
```

```
failure in step 5 (migrations)
  → compensate: step 5 (migration rollback : if reversible: true)
  (deploy hasn't started yet : no image to roll back)
```

The compensation loop in `deploy.go`:

```go
// Mark as started BEFORE the call, so compensation fires even on partial execution.
completed = append(completed, plan.StepMigrations)
if err := uc.cfg.Migrations.RunMigrations(ctx, migCfg); err != nil {
    return compensateAndFail(ctx, completed, ...)
}
```

### Dry-run mode

`--dry-run` prints the full plan (from `pilot.lock`) and exits before any mutation.
The plan is identical to what `pilot plan` renders.

---

## TypeC suspension / resume cycle

When a TypeC error occurs in the deploy use case:

```
cmd/deploy.go
  └─► uc.Execute()
        └─► vps.Deploy() → NewTypeC("PILOT-DEPLOY-003", ...)
              │
              └─► returned to cmd/deploy.go as *PilotError
                    │
                    ├─► errors.As(err, &pe) → TypeC detected
                    ├─► resume.SaveSuspension({ErrorCode, Command, Args, Options, Recommended})
                    │      └─► writes .pilot/suspended.json
                    └─► root.go printTypeC() renders the options to terminal
```

```
pilot resume [--answer 0]
  └─► resume.UseCase.Resolve()
        ├─► resume.LoadSuspension()   → reads .pilot/suspended.json
        ├─► matches answer to op.Options[idx] or exact text
        └─► dispatches: runDeployResume(op)
              └─► reconstructs flags from op.Args
              └─► deployCmd.RunE()    → re-runs the full deploy
```

For AI agents, the TypeC output is also available as structured JSON when `--json` is set:

```json
{
  "status": "awaiting_choice",
  "code": "PILOT-DEPLOY-003",
  "message": "user 'deploy' is not in the docker group on 1.2.3.4",
  "options": ["pilot setup --env prod", "ssh deploy@1.2.3.4 'sudo usermod -aG docker deploy'"],
  "recommended": "pilot setup --env prod",
  "resume_with": "pilot resume --answer 0"
}
```

---

## MCP server

`internal/mcp/` implements a JSON-RPC 2.0 server over stdio. No network port,
no separate process. Claude Code and Cursor start it automatically via `.mcp.json`.

```
stdin  ──► server.go (Read loop)
              │
              ▼
           Dispatch by method name
              │
              ├─ tools/list  ──► tools.go (schema definitions)
              └─ tools/call  ──► handlers/ (one file per tool group)
                                    │
                                    ├─ context.go   → mcp/context.Collect()
                                    ├─ lifecycle.go → app/deploy, app/up, app/down, ...
                                    ├─ preflight.go → app/preflight
                                    ├─ query.go     → app/status, app/logs
                                    ├─ setup.go     → app/runtime (VPS setup)
                                    └─ stub.go      → "not yet implemented"
stdout ◄── JSON response
```

All handlers are synchronous and blocking. Each tool call maps to exactly one use case
`Execute()` call. Error responses follow JSON-RPC 2.0 with pilot's error taxonomy
embedded in `data.pilot_error`.

### Tool registry

| Tool | Handler | Use case |
|------|---------|----------|
| `pilot_context` | `context.go` | `mcp/context.Collect()` |
| `pilot_generate_dockerfile` | `context.go` | writes file to disk |
| `pilot_generate_compose` | `context.go` | writes file to disk |
| `pilot_preflight` | `preflight.go` | `app/preflight` |
| `pilot_up` | `lifecycle.go` | `app/up` |
| `pilot_down` | `lifecycle.go` | `app/up` |
| `pilot_push` | `lifecycle.go` | `app/push` |
| `pilot_deploy` | `lifecycle.go` | `app/deploy` |
| `pilot_rollback` | `lifecycle.go` | `app/rollback` |
| `pilot_sync` | `lifecycle.go` | `app/sync` |
| `pilot_status` | `query.go` | `app/status` |
| `pilot_logs` | `query.go` | `app/logs` |
| `pilot_secrets_inject` | `query.go` | `app/runtime.NewSecretManager()` |
| `pilot_setup` | `setup.go` | VPS docker group fix |

---

## Code conventions

| Convention | Rule |
|------------|------|
| No `panic` | Always return an `error`. `panic` only in `init()` for programmer errors. |
| Context | All I/O functions accept `context.Context` as first argument. |
| Stubs | Return `fmt.Errorf("xxx: not yet implemented")`, never `nil`. |
| Factories | Named `New(cfg, ...)` in their own package. |
| Terminal output | Everything goes through `pkg/ui` : no `fmt.Println` in `internal/`. |
| `--json` flag | Use `ui.JSON(v)` instead of `ui.Success/Error/...` when JSON mode is active. |
| Error sites | Return `domain/errors.PilotError` with the right type, code, and exit code. |
| Error propagation | Use `WithCause(err)` to wrap the original cause : never lose it. |
| File writes | Atomic: write to `.tmp`, then `os.Rename` to final path. |
| Git | Feature branches `feat/xxx`, docs `docs/xxx`, merge `--no-ff` into `main`. |
| Tests | Use cases are tested with injected mocks : no Docker, no SSH in unit tests. |

### Adding a new provider

1. Create `internal/adapters/<name>/<name>.go`
2. Implement `domain.DeployProvider` (6 methods)
3. Add a case to `runtime.newRawProvider()` for the new `target.Type`
4. Update the implementation status table in `README.md`

That's it. The deploy use case, preflight, rollback, sync, status, and logs all work
immediately : they operate on the interface, not the concrete type.

### Adding a new MCP tool

1. Add the `Tool` definition to `internal/mcp/tools.go`
2. Add the handler in `internal/mcp/handlers/<group>.go`
3. Register the handler in `internal/mcp/handlers.go`

The server loop in `server.go` dispatches by method name automatically.
