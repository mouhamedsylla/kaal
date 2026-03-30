# Concepts & Philosophy

## The problem kaal solves

When you build and ship an application, three separate worlds need to agree:

- **You** — you know what your app needs (a database, a cache, specific memory limits, secret variables)
- **Your AI agent** — it writes Dockerfiles and compose files, but only if it understands your exact project structure
- **Your production environment** — it runs what was built, with what was configured, in the right place

Today these three worlds drift apart constantly. You tell the AI "generate a Dockerfile for my Go app" and get a generic template. You copy-paste environment variables and forget one. You build on macOS ARM64 and deploy to an AMD64 VPS. You add a new service locally and forget to sync the config remotely.

**kaal's answer:** a single `kaal.yaml` that all three parties read. You write it once, everyone stays in sync.

---

## The core model: kaal.yaml as shared contract

```
kaal.yaml
    │
    ├── You write it        → describes what your app needs
    │                         (services, environments, targets, registry)
    │
    ├── AI agent reads it   → understands your exact infra
    │                         generates the right Dockerfile and compose files
    │                         knows which env vars are compile-time vs runtime
    │                         knows where to deploy and with what constraints
    │
    └── kaal executes it    → runs it locally (docker compose)
                              deploys it remotely (SSH + docker compose)
                              handles the boring ops automatically
```

This is not just a config file. It is the **contract** between you, your tools, and your production environment. Everything else — Dockerfiles, compose files, CI scripts — is derived from it.

---

## Principles

### 1. Describe intent, not implementation

You declare **what** your app needs, not **how** to build it:

```yaml
services:
  app:
    type: app
    port: 8080
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
```

kaal (or your AI agent) generates the implementation. If you add a service, you add one line. The agent regenerates the compose file. No manual YAML editing for volumes, networks, healthchecks.

### 2. Local = Production

Local environments simulate production constraints:

```yaml
environments:
  dev:
    runtime: compose
    resources:
      cpus: "1"
      memory: 1G    # same limits as prod

  prod:
    target: vps-prod
    resources:
      cpus: "1"
      memory: 1G    # identical
```

If it works within these constraints locally, it will work in production.

### 3. AI-native, not AI-optional

kaal is designed to work *with* AI agents, not alongside them as an afterthought.

When `kaal up` finds missing files, it doesn't just fail — it constructs a structured prompt with your full project context and tells the agent exactly what to generate and with what constraints (multi-stage build, non-root user, health checks, VITE_* handling, env_file injection…).

When `kaal preflight` runs before a deploy, it returns a structured JSON action plan the agent follows step by step: human actions first, then agent actions, then the deploy goal.

When `kaal_context` is called via MCP, the agent receives everything: stack, services, existing files, missing files, env_file paths, unconfigured targets — a complete picture, not a summary.

**The agent doesn't guess. kaal tells it exactly what exists, what's missing, and what to do.**

### 4. Zero-friction local → prod

```bash
kaal up              # local
kaal push            # build
kaal deploy --env prod   # prod
```

Three commands. Same commands from your terminal, from CI, from your AI agent via MCP.

### 5. Automated ops, not documented ops

Most tools document what you need to do manually. kaal does it for you:

| Problem | Manual approach | kaal |
|---|---|---|
| Built ARM image on Mac, VPS is AMD64 | Add `--platform linux/amd64` to every build | Auto-detects Apple Silicon, builds linux/amd64 by default |
| Vite vars not showing in prod | Manually add `ARG`/`ENV` to Dockerfile | Auto-injects `VITE_*` vars, patches Dockerfile transparently |
| nginx config missing on VPS | `scp nginx/prod.conf deploy@host:~/kaal/nginx/` | `kaal sync` scans bind-mounts in compose file and copies them |
| `.env.prod` not on VPS | Manual `scp` | `kaal sync` copies all `env_file` declared in kaal.yaml |
| Deploy user not in docker group | SSH + `sudo usermod -aG docker deploy` | `kaal setup --env prod` |
| Wrong working dir on VPS | Debug compose errors | kaal always uses `~/kaal/docker-compose.<env>.yml` |

---

## The preflight system

Before pushing or deploying, `kaal preflight` runs a structured checklist and returns an action plan:

```
kaal preflight --target deploy

✓ kaal_yaml
✓ registry_image
✓ dockerfile
✓ docker_daemon
✓ registry_creds
✓ compose_file
✓ target_host
✓ ssh_key
✓ vps_connectivity
✓ vps_docker_group
✓ vps_env_file
✓ All checks passed — ready to deploy
```

When something is wrong, the report tells exactly who needs to act:
- `[HUMAN]` — you must do this (provide credentials, add SSH key, open firewall port)
- `[AGENT]` — the AI agent can call a kaal tool to fix this automatically

The agent calls `kaal_preflight` first, follows `next_steps[]` in order, and only asks you when human action is genuinely required — not for things it can handle itself.

---

## The AI-agent deploy workflow

In a project with Claude Code and `.mcp.json`:

```
You:    "Les tests passent, déploie en prod"

Agent:  kaal_preflight → all_ok: false
          [HUMAN] registry_creds: export GITHUB_TOKEN=...
        → waits for you

You:    (set the token)

Agent:  kaal_preflight → all_ok: true
        kaal_push → image built and pushed
        kaal_deploy → synced + deployed
        kaal_status → reports service health

You:    ✓ Done. No terminal opened.
```

The agent knows your infrastructure through `kaal.yaml`. It knows what's deployed through `kaal_status`. It knows what's broken through `kaal_preflight`. It knows how to fix things through `kaal_setup`, `kaal_sync`, and the generate tools. You stay in the chat.

---

## What kaal is not

- **Not a wrapper around Docker** — kaal supports compose, k3d (local Kubernetes), Lima (lightweight VMs), and cloud providers. Docker compose is the default implementation.
- **Not a CI tool** — kaal provides the primitives (`push`, `deploy`, `rollback`). GitHub Actions or GitLab CI sequence them.
- **Not a template engine** — Dockerfiles and compose files are generated by your AI agent reading your actual project, not by static templates.
- **Not Terraform** — kaal manages your application and its direct dependencies. Infrastructure provisioning (VMs, VPCs, managed databases) is Terraform or Pulumi's job.
- **Not opinionated about your stack** — Go, Node, Python, Rust, Java. VPS, AWS, GCP. GHCR, Docker Hub, custom registry. kaal abstracts the differences through provider interfaces.

---

## Project lifecycle

```
kaal init my-app
    └─► kaal.yaml + .mcp.json created
        Wizard: services, environments, VPS host, registry

─── first run ───

kaal up
    └─► Missing files detected
        Agent receives full context via kaal_context (MCP)
        or kaal context (paste into any AI chat)
        Agent generates: Dockerfile + docker-compose.dev.yml
        Files written to project root

kaal up
    └─► docker compose up
        Services running locally

─── development cycle ───

kaal push
    └─► VITE_* vars auto-detected from .env.dev
        linux/amd64 image built
        Image pushed to registry

kaal deploy --env prod
    └─► kaal sync: compose + env + nginx/prod.conf + ...
        docker pull on VPS
        docker compose up -d
        ✓ Deployed

─── something breaks ───

kaal rollback --env prod
    └─► Reads prev-tag from VPS state
        Restarts with previous image

─── infrastructure change ───

Edit kaal.yaml (add a service, change a port)
    └─► Agent calls kaal_context
        Regenerates docker-compose files
        kaal sync + kaal deploy
        VPS updated
```
