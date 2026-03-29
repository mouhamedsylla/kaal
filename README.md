<div align="center">

<img src="assets/mascot.png" alt="kaal mascot" width="1280" />

# kaal

**Dev Environment as Code**

Describe your infrastructure once. Run it locally. Ship it anywhere.

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat)](LICENSE)
[![MCP](https://img.shields.io/badge/MCP-ready-blueviolet?style=flat)](docs/workflows/ai-agent.md)

</div>

---

You're building a project. It'll run on a VPS with PostgreSQL, Redis, and specific memory limits. But locally you develop without constraints — different ports, no resource limits, `.env` files scattered everywhere. When you deploy, things break.

**kaal's answer:** write `kaal.yaml` once, describing what your infra needs. kaal simulates it locally, deploys it remotely, and makes sure both sides behave the same.

```
kaal init      →  describe your infra in kaal.yaml
kaal up        →  simulate it locally (docker compose, k3d, or VMs)
kaal push      →  build + push your image
kaal deploy    →  SSH into your VPS, pull, restart
```

---

## Install

```bash
# From source
go install github.com/mouhamedsylla/kaal@latest

# macOS / Linux (coming soon)
curl -sSL https://raw.githubusercontent.com/mouhamedsylla/kaal/main/install.sh | sh

# Homebrew (coming soon)
brew install mouhamedsylla/tap/kaal
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

The wizard asks four questions: project name, services (multi-select), environments, deployment target. Then writes `kaal.yaml`. Nothing else — no Dockerfiles, no compose files yet. Those come from your AI agent.

```bash
# Ask your AI agent to generate the infra files
kaal up
# → Missing: [Dockerfile, docker-compose.dev.yml]
# → "Ask Claude: Generate the missing infrastructure files for this project"

# Or pull the context yourself and paste it into any AI chat
kaal context

# Once files are in place
kaal up
# ✓ Environment "dev" is up
#   api     http://localhost:8080
```

---

## kaal.yaml

The single source of truth. Describes **what** your app needs, **how** each environment runs it, and **where** it deploys.

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

environments:
  dev:
    runtime: compose
    env_file: .env.dev
    resources:
      cpus: "2"
      memory: 4G        # mirror prod constraints locally

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

| Type | Description | Default version |
|---|---|---|
| `app` | Your application | — |
| `postgres` | PostgreSQL | 16 |
| `mysql` | MySQL | 8 |
| `mongodb` | MongoDB | 7 |
| `redis` | Redis | 7 |
| `rabbitmq` | RabbitMQ + management UI | 3 |
| `nats` | NATS messaging | latest |
| `nginx` | Nginx reverse proxy | alpine |
| `custom` | Any Docker image | — |

</details>

<details>
<summary>Supported runtimes</summary>

| Runtime | Description |
|---|---|
| `compose` | Docker Compose — default, works everywhere |
| `k3d` | Local k3s cluster — simulate Kubernetes locally *(coming soon)* |
| `lima` | Lightweight VM — faithful VPS simulation on macOS *(coming soon)* |

</details>

---

## Commands

### Local development

```bash
kaal up                    # start all services
kaal up api db             # start specific services
kaal up --build            # force rebuild
kaal down                  # stop services
kaal down --volumes        # stop + delete data volumes
kaal status                # check what's running
kaal logs api --follow     # stream logs
```

### AI agent integration

```bash
kaal context               # full project context → paste into any AI chat
kaal context --summary     # short summary
```

kaal is designed to work *with* AI agents. When `kaal up` finds missing infra files, it surfaces the full project context and guides you to ask your agent to generate them. With Claude Code or Cursor, this is automatic via MCP.

### Environment management

```bash
kaal env use prod          # switch active environment
kaal env current           # print active environment
```

### Build & deploy

```bash
kaal push                  # build + push (tag: git SHA)
kaal push --tag v1.2.3     # explicit tag
kaal push --platform linux/amd64,linux/arm64   # multi-arch

kaal deploy                # deploy active env to its target
kaal deploy --env prod --tag v1.2.3
kaal deploy --dry-run      # preview without executing

kaal sync                  # push kaal.yaml + compose files to remote
kaal rollback              # revert to previous deployment
kaal rollback --version v1.1.0
```

**Typical CI/CD flow:**

```bash
kaal push --tag $SHA
kaal deploy --env staging --tag $SHA
# run your tests...
kaal deploy --env prod --tag $SHA
```

### Registry credentials

| Registry | Variables |
|---|---|
| `ghcr` | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom` | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |

---

## AI-native via MCP

kaal runs a [Model Context Protocol](https://modelcontextprotocol.io) server. Add `.mcp.json` to your project:

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

Claude Code and Cursor will automatically start the server and get access to these tools:

| Tool | What it does |
|---|---|
| `kaal_context` | Full project context — call this first |
| `kaal_generate_dockerfile` | Write a Dockerfile to disk |
| `kaal_generate_compose` | Write a docker-compose file to disk |
| `kaal_up` / `kaal_down` | Start / stop local services |
| `kaal_push` | Build and push image |
| `kaal_deploy` | Deploy to remote target |
| `kaal_rollback` | Roll back deployment |
| `kaal_status` | Get full project state as JSON |
| `kaal_logs` | Get service logs |

**Example interaction:**

> "Les tests passent, déploie la v2.3 en prod"

Claude calls `kaal_push` → `kaal_deploy` → `kaal_status` → reports back with the result. You never leave the chat.

---

## Project layout

```
my-project/
├── kaal.yaml                  # infra blueprint — commit this
├── .mcp.json                  # AI agent config — commit this
├── Dockerfile                 # generated by your AI agent
├── docker-compose.dev.yml     # generated by your AI agent
├── .env.dev                   # local variables — do NOT commit
└── .env.prod                  # prod variables — do NOT commit
```

---

## What's implemented

| Feature | Status |
|---|---|
| `kaal init` — TUI wizard | ✅ |
| `kaal up / down` — local compose | ✅ |
| `kaal push` — build + push image | ✅ |
| `kaal deploy` — VPS/SSH | ✅ |
| `kaal rollback` — auto tag resolution | ✅ |
| `kaal sync` — push config to remote | ✅ |
| `kaal status / logs` — local + remote | ✅ |
| `kaal context` — AI agent prompt | ✅ |
| MCP server — context + generate tools | ✅ |
| MCP server — full handler wiring | 🔲 |
| `k3d` runtime — local Kubernetes | 🔲 |
| `lima` runtime — lightweight VMs | 🔲 |
| AWS / GCP / Azure / DigitalOcean | 🔲 |
| Secrets: AWS SM, GCP SM | 🔲 |
| Auto-rollback on healthcheck failure | 🔲 |

---

## Docs

Full documentation in [`docs/`](docs/README.md):

- [Concepts & philosophy](docs/concepts.md)
- [kaal.yaml reference](docs/kaal-yaml.md)
- [Architecture](docs/architecture.md)
- [Local dev workflow](docs/workflows/local-dev.md)
- [CI/CD workflow](docs/workflows/ci-cd.md)
- [AI agent workflow](docs/workflows/ai-agent.md)
- [VPS deploy workflow](docs/workflows/deploy-vps.md)

---

<div align="center">

MIT — built by [Mouhamed SYLLA](https://github.com/mouhamedsylla)

*Ship with confidence. Local = production.*

</div>
