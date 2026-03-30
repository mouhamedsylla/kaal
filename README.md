<div align="center">

<img src="assets/mascot.png" alt="kaal mascot" width="1280" />

# kaal

**Your infrastructure, as code. Your AI agent, as teammate.**

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-ready-blueviolet?style=flat)](docs/workflows/ai-agent.md)

</div>

---

Most deployment friction isn't technical. It's the gap between what you described to your AI agent, what Docker actually built, and what landed on your VPS.

**kaal closes that gap.** One file — `kaal.yaml` — describes your entire infrastructure. kaal reads it to run your app locally, your AI agent reads it to generate optimized infra files, and kaal executes it in production. Same contract, three contexts, zero drift.

```
kaal init    →  describe your infra in kaal.yaml (wizard TUI)
kaal up      →  run it locally (docker compose)
kaal push    →  build + push your image (auto-detects arch + compile-time vars)
kaal deploy  →  SSH into your VPS, sync, restart
```

---

## The mental model

```
kaal.yaml
    │
    ├── Human reads it      → understands what the app needs
    ├── AI agent reads it   → generates the right Dockerfile and compose files
    ├── kaal reads it       → runs it locally and deploys it remotely
    └── Same file. Always in sync.
```

This is the core idea. You don't maintain Dockerfiles by hand. You don't write compose files from scratch. You describe your services, environments, and targets — your AI agent handles the implementation details, kaal handles the execution.

---

## Install

```bash
# From source
go install github.com/mouhamedsylla/kaal@latest

# macOS / Linux (coming soon)
curl -sSL https://raw.githubusercontent.com/mouhamedsylla/kaal/main/install.sh | sh
```

---

## Quick start

```bash
# New project
mkdir my-api && cd my-api
kaal init

# Existing project — kaal detects your stack automatically
cd my-existing-project
kaal init
```

The wizard asks: name, services (app / postgres / redis / nginx...), environments, VPS target, registry. It writes `kaal.yaml` and `.mcp.json`. No Dockerfiles, no compose files yet — your AI agent generates those next.

```bash
kaal up
# → Missing: [Dockerfile, docker-compose.dev.yml]
# → Ask Claude: "Generate the missing infrastructure files for this project"
# → Claude calls kaal_context, reads your project, writes the files
# → Re-run:

kaal up
# ✓ Environment "dev" is up
#   api     http://localhost:8080
#   db      postgres://localhost:5432
```

---

## kaal.yaml

One file. Describes everything.

```yaml
apiVersion: kaal/v1

project:
  name: my-api
  stack: go
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
      memory: 1G        # mirror prod constraints locally

  prod:
    runtime: compose
    target: vps-prod
    env_file: .env.prod

targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_kaal

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/my-api
```

<details>
<summary>Supported services</summary>

| Type | Description |
|---|---|
| `app` | Your application |
| `postgres` | PostgreSQL |
| `mysql` | MySQL |
| `mongodb` | MongoDB |
| `redis` | Redis |
| `rabbitmq` | RabbitMQ + management UI |
| `nats` | NATS messaging |
| `nginx` | Nginx reverse proxy |
| `custom` | Any Docker image |

</details>

---

## AI-native via MCP

kaal ships a [Model Context Protocol](https://modelcontextprotocol.io) server. `kaal init` adds `.mcp.json` to your project automatically:

```json
{
  "mcpServers": {
    "kaal": {
      "command": "kaal",
      "args": ["mcp", "serve"],
      "cwd": "${workspaceFolder}"
    }
  }
}
```

Claude Code and Cursor start the server automatically. Your agent gets direct access to your infrastructure context and can act on it — no copy-paste, no leaving the editor.

### What the agent can do

| Tool | What it does |
|---|---|
| `kaal_context` | Full project context — stack, services, missing files, agent prompt |
| `kaal_generate_dockerfile` | Write an optimized Dockerfile to disk |
| `kaal_generate_compose` | Write a docker-compose file to disk |
| `kaal_preflight` | Pre-deploy checklist — returns a structured action plan |
| `kaal_push` | Build and push the image |
| `kaal_deploy` | Deploy to the configured target |
| `kaal_rollback` | Roll back to the previous deployment |
| `kaal_setup` | Fix Docker group permissions on the VPS |
| `kaal_sync` | Push config files to remote |
| `kaal_up` / `kaal_down` | Start / stop local services |
| `kaal_status` | Full project state as JSON |
| `kaal_logs` | Service logs |

### Real interactions

> *"Les tests passent, déploie la v2.3 en prod"*

The agent calls `kaal_preflight` → follows the action plan → `kaal_push` → `kaal_deploy` → `kaal_status` → reports back. You never leave the chat.

> *"Ajoute un reverse proxy nginx à l'architecture prod"*

The agent updates `docker-compose.prod.yml` via `kaal_generate_compose` → calls `kaal_sync` to push the new nginx config to the VPS → calls `kaal_deploy`. Done.

> *"Génère les fichiers d'infra pour ce projet"*

The agent calls `kaal_context`, reads your stack and services, generates a production-optimized multi-stage Dockerfile and docker-compose with healthchecks, named volumes, resource limits — adapted to your specific project, not a generic template.

---

## The deploy workflow

```bash
# Check everything before you ship
kaal preflight --target deploy
# ✓ kaal_yaml            project: my-api
# ✓ registry_image       ghcr.io/mouhamedsylla/my-api
# ✓ dockerfile           Dockerfile
# ✓ docker_daemon        reachable
# ✓ registry_creds       GITHUB_ACTOR=mouhamedsylla ✓
# ✓ compose_file         docker-compose.prod.yml
# ✓ target_host          1.2.3.4 (vps-prod)
# ✓ ssh_key              ~/.ssh/id_kaal
# ✓ vps_connectivity     connected to deploy@1.2.3.4
# ✓ vps_docker_group     deploy can run docker commands
# ✓ vps_env_file         .env.prod synced at ~/kaal/.env.prod
# ✓ All checks passed — ready to deploy

kaal push             # build linux/amd64 image + push
kaal deploy --env prod
# → Syncing files to remote (compose + env + nginx/prod.conf + ...)
# → Pulling image and restarting services
# ✓ Deployed my-api:abc1234 → vps-prod (1.2.3.4)
```

### What kaal handles so you don't have to

**Platform detection** — On Apple Silicon, kaal builds `linux/amd64` by default. Your image runs on the VPS without crashing.

**Compile-time env vars** — For Vite / Next.js / React apps, `VITE_*` and `NEXT_PUBLIC_*` variables must be baked into the bundle at build time. kaal auto-detects them from your `.env.prod` and injects them as `--build-arg`. If the Dockerfile is missing `ARG` declarations, kaal patches it transparently in a temp file — the original is never modified.

**Config file sync** — `kaal sync` scans your compose files for bind-mounts (e.g. `./nginx/prod.conf:/etc/nginx/...`) and copies those config files to `~/kaal/` on the VPS preserving the directory structure. No more Docker creating directories where files should be.

**Env file sync** — `kaal sync` copies the `env_file` declared for each environment in `kaal.yaml`. You never manually `scp` a `.env.prod` again.

**Docker group setup** — If the deploy user isn't in the docker group, `kaal setup --env prod` (or `kaal_setup` via MCP) fixes it over SSH with `sudo usermod -aG docker`.

---

## Commands

### Local development

```bash
kaal up                      # start all services
kaal up api db               # start specific services
kaal up --build              # force rebuild
kaal down                    # stop services
kaal down --volumes          # stop + delete data volumes
kaal status                  # check what's running
kaal logs api --follow       # stream logs
```

### Environment management

```bash
kaal env use prod            # switch active environment
kaal env current             # print active environment
```

### Build & deploy

```bash
kaal preflight               # pre-deploy checklist (auto-detects env)
kaal preflight --target push
kaal preflight --target deploy --env prod

kaal push                    # build + push (tag: git SHA)
kaal push --tag v1.2.3       # explicit tag
kaal push --env prod         # reads .env.prod for VITE_* build args

kaal sync --env prod         # push config files to VPS
kaal deploy --env prod
kaal deploy --env prod --tag v1.2.3
kaal rollback --env prod
kaal rollback --env prod --version v1.1.0

kaal setup --env prod        # fix Docker group permissions on VPS
```

### AI context

```bash
kaal context                 # full agent prompt → paste into any AI chat
kaal context --summary       # short summary
```

### Registry credentials

| Registry | Variables |
|---|---|
| `ghcr` | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom` | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |

---

## Project layout

```
my-project/
├── kaal.yaml                  # infra blueprint — commit this
├── .mcp.json                  # AI agent config — commit this
├── Dockerfile                 # generated by your AI agent
├── docker-compose.dev.yml     # generated by your AI agent
├── docker-compose.prod.yml    # generated by your AI agent
├── nginx/
│   └── prod.conf              # synced to VPS automatically by kaal sync
├── .env.dev                   # local variables — do NOT commit
└── .env.prod                  # prod variables — do NOT commit
```

---

## What's implemented

| Feature | Status |
|---|---|
| `kaal init` — TUI wizard (services, envs, VPS host, registry) | ✅ |
| `kaal up / down` — local docker compose | ✅ |
| `kaal push` — build + push (platform detection, VITE_* auto-inject) | ✅ |
| `kaal deploy` — VPS / SSH | ✅ |
| `kaal sync` — compose + env files + bind-mount config files | ✅ |
| `kaal rollback` — auto tag resolution | ✅ |
| `kaal status / logs` — local + remote | ✅ |
| `kaal preflight` — structured pre-deploy checklist | ✅ |
| `kaal setup` — Docker group fix via SSH | ✅ |
| `kaal context` — AI agent prompt | ✅ |
| MCP server — full tool suite (context, generate, deploy, preflight…) | ✅ |
| Secrets: local .env | ✅ |
| Registry: GHCR, Docker Hub, custom | ✅ |
| `k3d` runtime — local Kubernetes | 🔲 |
| `lima` runtime — lightweight VMs | 🔲 |
| AWS / GCP / Azure / DigitalOcean providers | 🔲 |
| Secrets: AWS SM, GCP SM | 🔲 |
| Auto-rollback on healthcheck failure | 🔲 |

---

## Docs

- [Concepts & philosophy](docs/concepts.md)
- [kaal.yaml reference](docs/kaal-yaml.md)
- [Architecture](docs/architecture.md)
- [Local dev workflow](docs/workflows/local-dev.md)
- [AI agent workflow](docs/workflows/ai-agent.md)
- [VPS deploy workflow](docs/workflows/deploy-vps.md)
- [CI/CD workflow](docs/workflows/ci-cd.md)

---

<div align="center">

MIT — built by [Mouhamed SYLLA](https://github.com/mouhamedsylla)

*One file. Local and production, always in sync.*

</div>
