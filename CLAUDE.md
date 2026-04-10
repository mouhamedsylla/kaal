# pilot — Dev Environment as Code

**Auteur** : Mouhamed SYLLA (@mouhamedsylla)
**Module Go** : `github.com/mouhamedsylla/pilot`
**Go version** : 1.25.3

---

## Résumé du projet

`pilot` est un CLI terminal-first, opiniated et IA-natif qui accompagne le développeur de l'initialisation du projet jusqu'au déploiement en production. Ce qui tourne en local tourne en production sans modification.

---

## Architecture

```
pilot/
├── main.go
├── cmd/                        # Commandes Cobra (1 fichier = 1 commande)
├── internal/
│   ├── config/                 # Parse + valide pilot.yaml
│   ├── scaffold/               # pilot init — génération de projets
│   ├── orchestrator/           # Interface + compose/ + k8s/ (stub)
│   ├── providers/              # Interface + vps/ + aws/ gcp/ azure/ do/ (stubs)
│   ├── registry/               # Interface + ghcr/ dockerhub/ custom/ + stubs
│   ├── secrets/                # Interface + local/ + aws_sm/ gcp_sm/ (stubs)
│   └── mcp/                    # Serveur MCP JSON-RPC 2.0 stdio
└── pkg/
    ├── ui/                     # Spinner, couleurs, JSON output
    └── ssh/                    # Client SSH (golang.org/x/crypto/ssh)
```

### Principe architectural fondamental

Chaque couche extensible est définie par une **interface Go** dans son package racine :
- `internal/orchestrator/orchestrator.go` → `Orchestrator`
- `internal/providers/provider.go` → `Provider`
- `internal/registry/registry.go` → `Registry`
- `internal/secrets/secrets.go` → `SecretManager`

Les `factory.go` dans chaque package instancient la bonne implémentation selon `pilot.yaml`. Les stubs retournent `fmt.Errorf("xxx: not yet implemented")`.

---

## Commandes CLI

| Commande | Description |
|---|---|
| `pilot init [name]` | Scaffold complet d'un projet |
| `pilot env use <env>` | Switch d'environnement actif |
| `pilot up [services...]` | Lance l'environnement local |
| `pilot down` | Arrête l'environnement local |
| `pilot push` | Build + push image vers le registry |
| `pilot deploy` | Déploie sur la cible distante |
| `pilot sync` | Sync config locale → remote |
| `pilot status` | État complet du projet |
| `pilot logs [service]` | Logs d'un service |
| `pilot mcp serve` | Démarre le serveur MCP |

Flags globaux : `--env/-e`, `--json`, `--config`

---

## pilot.yaml — schéma

```yaml
apiVersion: pilot/v1
project:
  name: mon-projet
  stack: go           # go | node | python | rust | java
  language_version: "1.23"
registry:
  provider: ghcr      # ghcr | dockerhub | ecr | gcr | acr | custom
  image: ghcr.io/mouhamedsylla/mon-projet
environments:
  dev:
    compose_file: docker-compose.dev.yml
    env_file: .env.dev
    ports:
      api: 8080
      db: 5432
  prod:
    target: vps-prod
    compose_file: docker-compose.prod.yml
    secrets:
      provider: local
      refs:
        DATABASE_URL: DATABASE_URL
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot
    port: 22
```

---

## Stack technique

| Composant | Choix |
|---|---|
| Langage | Go 1.23 |
| CLI | `github.com/spf13/cobra` + `github.com/spf13/viper` |
| TUI | `github.com/charmbracelet/bubbletea` + `lipgloss` |
| SSH | `golang.org/x/crypto/ssh` |
| YAML | `gopkg.in/yaml.v3` |
| MCP | Implémentation custom JSON-RPC 2.0 stdio |

---

## Conventions de code

- **Pas de `panic`** dans les paths normaux — toujours retourner une `error`
- **Context** : toutes les fonctions I/O acceptent `context.Context` en premier argument
- **Stubs** : les implémentations non encore faites retournent `fmt.Errorf("xxx: not yet implemented")`, jamais de `nil` silencieux
- **Nommage** : les factories s'appellent `New(cfg, ...)` dans leur package
- **Output** : tout output utilisateur passe par `pkg/ui` (pas de `fmt.Println` direct dans `internal/`)
- **JSON output** : respecter le flag `--json` global en utilisant `ui.JSON()` vs affichage table

---

## Providers et registres implémentés

| Composant | Implémenté | Stub |
|---|---|---|
| Orchestrator: compose | ✅ | |
| Orchestrator: k8s | | ✅ |
| Provider: VPS/SSH | ✅ | |
| Provider: AWS | | ✅ |
| Provider: GCP | | ✅ |
| Provider: Azure | | ✅ |
| Provider: DigitalOcean | | ✅ |
| Registry: GHCR | ✅ | |
| Registry: Docker Hub | ✅ | |
| Registry: Custom | ✅ | |
| Registry: ECR | | ✅ |
| Registry: GCR | | ✅ |
| Registry: ACR | | ✅ |
| Secrets: local (.env) | ✅ | |
| Secrets: AWS SM | | ✅ |
| Secrets: GCP SM | | ✅ |

---

## MCP Server

Transport : stdio (JSON-RPC 2.0). Pas de port réseau, pas de processus séparé.

Config client (`.mcp.json` à la racine) :
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

Tools exposés : `pilot_init`, `pilot_env_switch`, `pilot_up`, `pilot_down`, `pilot_push`, `pilot_deploy`, `pilot_rollback`, `pilot_sync`, `pilot_status`, `pilot_logs`, `pilot_config_get`, `pilot_config_set`, `pilot_secrets_inject`

---

## Variables d'environnement

| Variable | Usage |
|---|---|
| `GITHUB_TOKEN` | Auth GHCR push/pull |
| `GITHUB_ACTOR` | Username GHCR |
| `DOCKER_USERNAME` / `DOCKER_PASSWORD` | Auth Docker Hub |
| `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` | Auth registry custom |

---

## Build & run local

```bash
go build -o pilot .
./pilot --help
./pilot init my-app
./pilot up
./pilot deploy --env prod
```
