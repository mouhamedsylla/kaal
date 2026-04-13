# pilot

**Dev Environment as Code. AI-native. Terminal-first.**

---

Most deployment friction isn't technical. It's the gap between what you told your AI agent, what Docker actually built, and what landed on your VPS — silently broken.

**pilot closes that gap.** One file — `pilot.yaml` — is the single source of truth for your entire infrastructure. pilot reads it to run your app locally, your AI agent reads it to generate optimized infra files, and pilot executes the same contract in production. Zero drift, by design.

---

## Install

**macOS / Linux** — one-liner via the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/mouhamedsylla/pilot/main/install.sh | sh
```

**Go toolchain** — if you already have Go installed:

```bash
go install github.com/mouhamedsylla/pilot@latest
```

---

## Quick start

```bash
pilot init          # scaffold pilot.yaml + .mcp.json
pilot up            # run locally (docker compose)
pilot push          # build + push image
pilot deploy        # SSH → sync → migrate → deploy → healthcheck
```

Claude Code and Cursor pick up `.mcp.json` automatically — your agent can run any of these commands without leaving the chat.

---

## bob — the pilot agent

**bob** is pilot's dedicated AI agent. Instead of a chat window, it's a terminal-first REPL that talks directly to pilot via MCP.

```bash
pip install bob
bob                 # start the REPL
```

```
  ╭────────────────────────────────────────────────────────╮
  │  ╭─╮╭─╮                                               │
  │  ╰─╯╰─╯  bob  v0.1                                    │
  │  █ ▘▝ █  claude-3-5-sonnet  ·  my-api  ·  prod        │
  │                                                        │
  │  Décris ce que tu veux faire  ·  Ctrl+D  ·  Ctrl+C    │
  ╰────────────────────────────────────────────────────────╯
```

bob knows your `pilot.yaml`, generates missing infra files, manages your env variables without leaking secrets, and deploys — all from a single prompt. Supports Anthropic, OpenAI, DeepSeek and Ollama models.

→ [github.com/mouhamedsylla/bob](https://github.com/mouhamedsylla/bob)

---

## pilot.yaml

One file expresses intention — pilot infers execution.

```yaml
apiVersion: pilot/v1

project:
  name: my-api
  stack: go

services:
  app:   { type: app,      port: 8080 }
  db:    { type: postgres, version: "16" }
  cache: { type: redis }

environments:
  dev:  { runtime: compose, env_file: .env.dev }
  prod: { runtime: compose, target: vps-prod, env_file: .env.prod }

targets:
  vps-prod: { type: vps, host: 1.2.3.4, user: deploy, key: ~/.ssh/id_pilot }

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/my-api
```

→ Full schema and options in [docs/pilot-yaml.md](docs/pilot-yaml.md)

---

## Documentation

| Topic | Link |
|-------|------|
| Commands reference | [docs/commands.md](docs/commands.md) |
| pilot.yaml schema | [docs/pilot-yaml.md](docs/pilot-yaml.md) |
| Resilience model (TypeA/B/C/D) | [docs/resilience.md](docs/resilience.md) |
| MCP server & tools | [docs/mcp-server.md](docs/mcp-server.md) |
| Architecture | [docs/architecture.md](docs/architecture.md) |

---

<div align="center">

MIT — built by [Mouhamed SYLLA](https://github.com/mouhamedsylla)

*One file. From local to production, always in sync.*

</div>
