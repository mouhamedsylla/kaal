<div align="center">

<img src="assets/mascot.png" alt="pilot mascot" width="1280" />

# pilot

**Dev Environment as Code. AI-native. Terminal-first.**

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-ready-blueviolet?style=flat)](docs/mcp-server.md)

</div>

---

Most deployment friction isn't technical. It's the gap between what you told your AI agent, what Docker actually built, and what landed on your VPS — silently broken.

**pilot closes that gap.** One file — `pilot.yaml` — is the single source of truth for your entire infrastructure. pilot reads it to run your app locally, your AI agent reads it to generate optimized infra files, and pilot executes the same contract in production. Zero drift, by design.

```
pilot init    →  describe your infra in pilot.yaml
pilot up      →  run it locally  (docker compose)
pilot push    →  build + push    (auto-detects arch, compile-time vars)
pilot deploy  →  SSH, sync, pull, restart  (with lock, hooks, migrations)
```

When something breaks, pilot doesn't just crash. It tells you exactly what failed, why, and what to do — and if it can fix it silently, it does. That's the resilience model.

---

## Table of contents

- [Install](#install)
- [Quick start](#quick-start)
- [pilot.yaml reference](#pilotyaml-reference)
- [Commands](#commands)
- [The resilience model](#the-resilience-model)
- [AI-native via MCP](#ai-native-via-mcp)
- [Implementation status](#implementation-status)
- [Architecture](#architecture)

---

## Install

```bash
# From source
go install github.com/mouhamedsylla/pilot@latest

# macOS / Linux
curl -sSL https://raw.githubusercontent.com/mouhamedsylla/pilot/main/install.sh | sh
```

---

## Quick start

```bash
# New project
mkdir my-api && cd my-api
pilot init

# Existing project — pilot detects your stack automatically
cd my-existing-project
pilot init
```

The wizard asks: project name, services (app / postgres / redis / nginx...), environments, VPS target, registry. It writes `pilot.yaml` and `.mcp.json`. No Dockerfiles, no compose files yet — your AI agent generates those.

```bash
pilot up
# → Missing files: [Dockerfile, docker-compose.dev.yml]
# → Ask Claude: "Generate the missing infrastructure files"
# → Claude calls pilot_context, reads your project, writes the files
# → Re-run:
pilot up
# ✓  api     http://localhost:8080
# ✓  db      postgres://localhost:5432
```

---

## pilot.yaml reference

One file. Expresses intention — pilot infers execution.

```yaml
apiVersion: pilot/v1

project:
  name: my-api
  stack: go                  # go | node | python | rust | java
  language_version: "1.23"

services:
  app:
    type: app
    port: 8080
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
  proxy:
    type: nginx

environments:
  dev:
    runtime: compose
    env_file: .env.dev
    resources:
      cpus: "1"
      memory: 1G             # mirror prod constraints locally

  prod:
    runtime: compose
    target: vps-prod
    env_file: .env.prod
    secrets:
      provider: local
      refs:
        DATABASE_URL: DATABASE_URL
    hooks:
      pre_deploy:
        - command: "echo 'starting deploy'"
          description: "Deploy started notification"
      post_deploy:
        - command: "curl -X POST $WEBHOOK_URL"
          description: "Notify webhook"
    migrations:
      tool: prisma
      command: "npx prisma migrate deploy"
      rollback_command: "npx prisma migrate rollback"
      reversible: true

targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot
    port: 22

registry:
  provider: ghcr             # ghcr | dockerhub | ecr | gcr | acr | custom
  image: ghcr.io/mouhamedsylla/my-api
  build_args:                # compile-time vars auto-injected at build
    - VITE_API_URL
    - NEXT_PUBLIC_ENV
```

**The minimality principle:** a field belongs in `pilot.yaml` only if pilot cannot deduce it from the project, or if you explicitly want to override the default.

What pilot infers automatically:

| What | How |
|------|-----|
| Migration tool | `prisma/schema.prisma`, `alembic.ini`, `flyway.conf`, `go-migrate` dirs |
| Service startup order | `depends_on` in compose files |
| Ports to check | `ports:` declarations in compose files |
| nginx reload on sync | services with image containing `nginx` and bind-mounted `.conf` files |
| Compile-time vars | `VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*` prefixes |
| Healthchecks | `healthcheck:` in compose services |
| Project stack | `go.mod`, `package.json`, `requirements.txt`, `Cargo.toml`, etc. |

---

## Commands

### Development

```bash
pilot up                          # start all services
pilot up api db                   # start specific services
pilot up --build                  # force rebuild
pilot down                        # stop all services
pilot down --volumes              # stop + delete named volumes

pilot status                      # runtime state of all services
pilot logs api --follow           # stream logs
pilot logs api --since 1h --lines 100
```

### Environments

```bash
pilot env use prod                # switch active environment
pilot env current                 # print active environment
pilot env diff dev prod           # compare two envs (vars, ports, services)
```

### Build and deploy

```bash
# Before you ship: verify everything
pilot preflight --target deploy   # generate pilot.lock + full checklist
pilot preflight --target push

# Build and push
pilot push                        # tag: git short SHA
pilot push --tag v1.2.3
pilot push --platform linux/amd64,linux/arm64

# Deploy
pilot plan                        # show execution plan without running
pilot deploy --env prod           # full pipeline: lock check → secrets → sync → hooks → migrations → deploy → healthcheck
pilot deploy --env prod --tag v1.2.3
pilot deploy --dry-run            # show plan without executing

# Rollback
pilot rollback --env prod
pilot rollback --env prod --version v1.1.0

# Sync config without redeploy
pilot sync --env prod
```

### Resilience

```bash
pilot diagnose                    # full system snapshot (Docker, SSH, ports, git, registry)
pilot resume                      # resume suspended operation (TypeC error)
pilot resume --answer 0           # answer by option index
pilot resume --answer "8081"      # answer by exact text
```

### Secrets

```bash
pilot secrets list                # list keys in active env's .env file
pilot secrets get DATABASE_URL
pilot secrets set DATABASE_URL "postgres://..."
pilot secrets inject              # resolve + display all secrets (no values)
pilot secrets inject --show-values
```

### AI agent

```bash
pilot mcp serve                   # start MCP server (auto-started by Claude Code / Cursor)
pilot mcp context                 # full project context → paste into any AI chat
pilot mcp context --summary       # short version
```

### Global flags

```bash
--env / -e <name>                 # override active environment
--json                            # machine-readable JSON output
--config <path>                   # explicit pilot.yaml path
```

---

## The resilience model

pilot never leaves you — or your AI agent — in an ambiguous state.

### Every failure is classified

| Type | Who acts | How |
|------|----------|-----|
| **TypeA** — deterministic, low-risk | pilot, silently | auto-fix, log, continue |
| **TypeB** — deterministic, impactful | pilot, announced | auto-fix, print what it did, supports `--dry-run` |
| **TypeC** — choice required, options known | human or agent | pilot suspends, presents options, waits |
| **TypeD** — choice required, options unknown | human | pilot stops with step-by-step instructions |

### TypeC in practice

```
$ pilot deploy

  ✓  preflight OK
  ✓  migration applied

  ✗  user "deploy" is not in the docker group on 1.2.3.4

     Possible actions:
     → [0] pilot setup --env prod   (automatic)
       [1] ssh deploy@1.2.3.4 'sudo usermod -aG docker deploy'   (manual)

     After taking action, run: pilot resume
```

The suspended context is saved to `.pilot/suspended.json`. `pilot resume` restarts from where it stopped.

For AI agents, the same event arrives as structured JSON:
```json
{
  "status": "awaiting_choice",
  "code": "PILOT-DEPLOY-003",
  "options": ["pilot setup --env prod", "ssh deploy@1.2.3.4 '...'"],
  "recommended": "pilot setup --env prod",
  "resume_with": "pilot resume --answer 0"
}
```

### The deploy pipeline

`pilot deploy` runs a 7-step skeleton — each step activates only if relevant:

```
[1] lock check      validate pilot.lock is not stale
[2] secrets         resolve refs → temp env file
[3] sync            push compose + config files to remote
[4] pre_hooks       run pre-deploy commands via SSH        (if declared)
[5] migrations      apply schema changes                   (if detected)
[6] deploy          docker pull + compose up
[7] post_hooks      run post-deploy commands via SSH       (if declared)
[8] healthcheck     wait for all services healthy
```

On failure from step 4 onward, **LIFO compensation** runs automatically:
- image rollback (always when deploy started)
- migration rollback (if `reversible: true` + `rollback_command` defined)

### pilot.lock

Generated by `pilot preflight --target deploy`. Commit it to your repo.

```yaml
# pilot.lock — generated automatically
schema_version: 1
generated_from:
  - pilot.yaml
  - docker-compose.prod.yml
  - prisma/schema.prisma
project_hash: "sha256:abc..."     # staleness guard

execution_plan:
  nodes_active: [preflight, migrations, deploy, healthcheck]
  migrations:
    tool: prisma
    command: npx prisma migrate deploy
    rollback_command: npx prisma migrate rollback
    reversible: true
    detected_from: prisma/schema.prisma
execution_provider: compose
```

If any source file changes, pilot detects the stale lock and refuses to deploy until you re-run preflight. What runs in production is what your team validated — not what pilot inferred this morning.

### State machine

pilot's state machine has one invariant: every operation ends in `SUCCEEDED` or `GUIDED_FAILURE`. Never indeterminate.

```
IDLE
 ├─► PREFLIGHTING  ──── TypeA/B ──► RECOVERING ──► PREFLIGHTING
 │       └─ TypeC ─► AWAITING_CHOICE ──► PREFLIGHTING
 │
 └─► EXECUTING  ──── TypeA/B ──► RECOVERING ──► EXECUTING
         ├─ TypeC ─► AWAITING_CHOICE ──► EXECUTING
         ├─ TypeD ─► GUIDED_FAILURE  (terminal)
         └─ OK   ─► SUCCEEDED        (terminal)
```

---

## AI-native via MCP

pilot ships a [Model Context Protocol](https://modelcontextprotocol.io) server — JSON-RPC 2.0 over stdio, no network port, no separate process.

`pilot init` adds `.mcp.json` to your project:

```json
{
  "mcpServers": {
    "pilot": {
      "command": "pilot",
      "args": ["mcp", "serve"],
      "cwd": "${workspaceFolder}"
    }
  }
}
```

Claude Code and Cursor start the server automatically when you open the project.

### Tools

| Tool | What it does |
|------|-------------|
| `pilot_context` | Full project context (stack, services, missing files, agent prompt) |
| `pilot_generate_dockerfile` | Write an optimized Dockerfile to disk (multi-stage, non-root, healthcheck, pinned versions) |
| `pilot_generate_compose` | Write a docker-compose file to disk |
| `pilot_preflight` | Run preflight checks, return structured action plan |
| `pilot_up` / `pilot_down` | Start / stop local services |
| `pilot_push` | Build and push image |
| `pilot_deploy` | Full deploy pipeline |
| `pilot_rollback` | Roll back to previous deployment |
| `pilot_sync` | Sync config files to remote |
| `pilot_status` | Service state as JSON |
| `pilot_logs` | Stream service logs |
| `pilot_secrets_inject` | Resolve and display secrets |
| `pilot_setup` | Fix Docker group on VPS |

### Real agent interactions

> *"Tests pass, deploy v2.3 to prod"*

Agent: `pilot_preflight` → follows plan → `pilot_push` → `pilot_deploy` → `pilot_status` → reports back. One conversation, no terminal.

> *"Add nginx reverse proxy to the prod architecture"*

Agent: `pilot_generate_compose` (adds nginx service) → `pilot_sync` (pushes nginx.conf to VPS) → `pilot_deploy`. Done.

> *"Generate infrastructure files for this project"*

Agent: `pilot_context` → reads stack, services, existing files → writes multi-stage Dockerfile + docker-compose with healthchecks, volumes, resource limits — tailored to your project, not a generic template.

---

## Implementation status

| Feature | Status |
|---------|--------|
| `pilot init` — TUI wizard | ✅ |
| `pilot up / down` — local compose | ✅ |
| `pilot push` — build + push (platform detection, VITE_* auto-inject) | ✅ |
| `pilot deploy` — VPS / SSH with full 7-step pipeline | ✅ |
| `pilot sync` — compose + env files + bind-mount configs | ✅ |
| `pilot rollback` — auto tag resolution + LIFO compensation | ✅ |
| `pilot status / logs` — local + remote | ✅ |
| `pilot preflight` — generates pilot.lock with staleness guard | ✅ |
| `pilot plan` — execution plan without running | ✅ |
| `pilot diagnose` — full system snapshot | ✅ |
| `pilot resume` — TypeC suspension/resume | ✅ |
| `pilot env diff` — compare two environments | ✅ |
| `pilot secrets` — list, get, set, inject | ✅ |
| `pilot setup` — Docker group fix via SSH | ✅ |
| MCP server — full tool suite | ✅ |
| Error taxonomy TypeA/B/C/D | ✅ |
| Hooks: pre_deploy, post_deploy | ✅ |
| Migrations: prisma, alembic, goose, flyway (auto-detect) | ✅ |
| Secrets: local .env | ✅ |
| Registry: GHCR, Docker Hub, custom | ✅ |
| Secrets: AWS SM, GCP SM | stub |
| Registry: ECR, GCR, ACR | stub |
| Providers: AWS, GCP, Azure, DigitalOcean | stub |
| Runtime: k3d, lima | stub |

---

## Architecture

The codebase follows hexagonal architecture (ports & adapters). The dependency rule is strict: adapters implement domain interfaces — domain never imports adapters.

```
cmd/ and mcp/
     │
     ▼ (call)
internal/app/          ← use cases (pure business logic, injected ports)
     │
     ▼ (depend on)
internal/domain/       ← interfaces (ports) + error taxonomy + state machine
     ▲
     │ (implement)
internal/adapters/     ← concrete implementations (compose, vps, ghcr, ...)
```

See [docs/architecture.md](docs/architecture.md) for the full technical breakdown.

---

## Project layout

```
my-project/
├── pilot.yaml                  # infra blueprint — commit this
├── pilot.lock                  # validated execution plan — commit this
├── .mcp.json                   # AI agent config — commit this
├── .pilot/                     # runtime state — do NOT commit
│   ├── state.json              # state machine + last operation
│   └── suspended.json          # TypeC pending choice (if any)
├── Dockerfile                  # generated by your AI agent
├── docker-compose.dev.yml      # generated by your AI agent
├── docker-compose.prod.yml     # generated by your AI agent
├── nginx/
│   └── prod.conf               # synced to VPS automatically by pilot sync
├── .env.dev                    # local variables — do NOT commit
└── .env.prod                   # prod variables — do NOT commit
```

---

## Registry credentials

| Registry | Environment variables |
|----------|-----------------------|
| `ghcr` | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom` | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |

---

<div align="center">

MIT — built by [Mouhamed SYLLA](https://github.com/mouhamedsylla)

*One file. From local to production, always in sync.*

</div>
