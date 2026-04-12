# Internals : Serveur MCP

## Protocole

pilot implémente le **Model Context Protocol (MCP)** en JSON-RPC 2.0 sur stdin/stdout.

- **Pas de port réseau** : communication par pipe standard
- **Pas de processus séparé** : `pilot mcp serve` *est* le serveur
- **Intégration IDE** : Claude Code et Cursor gèrent le cycle de vie du processus

---

## Démarrage

```bash
pilot mcp serve
```

Le processus lit les requêtes JSON-RPC depuis stdin, écrit les réponses sur stdout.

**Configuration client (`.mcp.json`) :**

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

L'IDE démarre `pilot mcp serve` dans le dossier du projet. `cwd` est crucial : pilot lit `pilot.yaml` depuis le répertoire courant.

---

## Structure des messages

### Requête (stdin)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "pilot_context",
    "arguments": {
      "env": "dev"
    }
  }
}
```

### Réponse (stdout)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"pilot_yaml\": \"...\", \"stack\": \"go\", ...}"
      }
    ]
  }
}
```

### Réponse d'erreur

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{"type": "text", "text": "pilot_context: pilot.yaml not found"}],
    "isError": true
  }
}
```

Note : les erreurs d'outil retournent un `result` avec `isError: true`, pas un champ `error` JSON-RPC : conformément à la spec MCP.

---

## Méthodes supportées

| Méthode | Description |
|---------|-------------|
| `initialize` | Handshake initial : retourne les capabilities du serveur |
| `tools/list` | Liste tous les outils disponibles avec leur schéma |
| `tools/call` | Appelle un outil avec les paramètres donnés |

---

## Outils implémentés

| Outil | Handler | Statut |
|-------|---------|--------|
| `pilot_context` | `handlers.HandleContext` | ✅ |
| `pilot_generate_dockerfile` | `handlers.HandleGenerateDockerfile` | ✅ |
| `pilot_generate_compose` | `handlers.HandleGenerateCompose` | ✅ |
| `pilot_init` | `handlers.HandleInit` | ✅ |
| `pilot_env_switch` | `handlers.HandleEnvSwitch` | ✅ |
| `pilot_up` | `handlers.HandleUp` | ✅ |
| `pilot_down` | `handlers.HandleDown` | ✅ |
| `pilot_push` | `handlers.HandlePush` | ✅ |
| `pilot_deploy` | `handlers.HandleDeploy` | ✅ |
| `pilot_rollback` | `handlers.HandleRollback` | ✅ |
| `pilot_sync` | `handlers.HandleSync` | ✅ |
| `pilot_status` | `handlers.HandleStatus` | ✅ |
| `pilot_logs` | `handlers.HandleLogs` | ✅ |
| `pilot_secrets_inject` | `handlers.HandleSecretsInject` | ✅ |
| `pilot_setup` | `handlers.HandleSetup` | ✅ |
| `pilot_preflight` | `handlers.HandlePreflight` | ✅ |

---

## Description des outils principaux

### `pilot_context`

Retourne le contexte complet du projet : stack, services, fichiers existants/manquants, prompt agent, env_file paths, targets. **Premier outil à appeler avant toute génération.**

Paramètres : `env` (optionnel)

### `pilot_generate_dockerfile`

Écrit un Dockerfile optimisé sur disque. L'agent génère le contenu selon des règles strictes encodées dans la description de l'outil : multi-stage, image minimale (distroless/alpine), utilisateur non-root, healthcheck, pas d'`ARG` suivis d'`ENV` (piège silencieux), COPY sélectif, tags épinglés.

Paramètres : `content` (requis), `path` (optionnel, défaut : `Dockerfile`)

### `pilot_generate_compose`

Écrit un `docker-compose.<env>.yml` sur disque. Règles encodées : volumes nommés, réseaux explicites, limites de ressources, healthchecks, `depends_on` avec `condition: service_healthy`, `env_file` obligatoire sur tous les services applicatifs, commandes de dev server adaptées au framework (`--mode <env>` pour Vite).

Paramètres : `content` (requis), `env` (optionnel), `path` (optionnel)

### `pilot_preflight`

Lance la checklist pré-déploiement. Retourne un plan d'action structuré avec `all_ok`, `checks[]`, `next_steps[]`. **L'agent suit les `next_steps` dans l'ordre** : traite d'abord les `[HUMAN]` (demande confirmation), puis les `[AGENT]` (appelle l'outil indiqué). Génère `pilot.lock` si tout passe avec `--target deploy`.

Paramètres : `target` (`up`, `push` ou `deploy`), `env` (optionnel)

### `pilot_push`

Build + push de l'image. Injecte automatiquement les vars `VITE_*`/`NEXT_PUBLIC_*`/`REACT_APP_*` depuis l'env file. Patche le Dockerfile si des `ARG` manquent. Détecte macOS ARM64 → build `linux/amd64`.

Paramètres : `tag` (optionnel), `env` (optionnel), `no_cache` (optionnel), `platform` (optionnel)

### `pilot_deploy`

Exécute le pipeline de déploiement complet en 8 étapes : lock check → secrets → sync → pre_hooks → migrations → deploy → post_hooks → healthcheck. En cas d'échec à partir de l'étape 4, compensation LIFO automatique.

Paramètres : `env` (optionnel), `tag` (optionnel), `dry_run` (optionnel)

### `pilot_setup`

Ajoute le user deploy au groupe docker sur le VPS via SSH. À appeler quand `pilot_preflight` retourne une erreur `vps_docker_group` ou quand `pilot_deploy` retourne une erreur TypeC `PILOT-DEPLOY-003`.

Paramètres : `env` (optionnel)

---

## Architecture du package

```
internal/mcp/
├── server.go          # Boucle I/O + dispatch JSON-RPC
├── tools.go           # Définitions Tool + registerAll()
├── handlers.go        # Wiring des vars HandlerFunc
└── handlers/
    ├── context.go     # HandleContext, HandleGenerateDockerfile, HandleGenerateCompose
    ├── lifecycle.go   # HandleUp, HandleDown, HandlePush, HandleDeploy, HandleRollback, HandleSync
    ├── preflight.go   # HandlePreflight
    ├── query.go       # HandleStatus, HandleLogs, HandleSecretsInject
    ├── setup.go       # HandleSetup
    ├── init.go        # HandleInit
    └── capture.go     # Helpers partagés (capture stdout, JSON response)
```

### `HandlerFunc`

```go
type HandlerFunc func(ctx context.Context, params map[string]any) (any, error)
```

Chaque handler reçoit les arguments de l'outil et retourne n'importe quelle valeur sérialisable en JSON, ou une erreur.

### Ajouter un handler

1. Créer ou éditer `internal/mcp/handlers/<feature>.go`
2. Implémenter la fonction avec la signature `HandlerFunc`
3. Déclarer le `Tool` dans `internal/mcp/tools.go`
4. Dans `handlers.go`, câbler le handler :
   ```go
   var handleMyTool HandlerFunc = handlers.HandleMyTool
   ```
5. Dans `tools.go`, enregistrer dans `registerAll()` :
   ```go
   s.Register(toolMyTool, handleMyTool)
   ```

---

## Workflow typique d'un agent AI

### Génération d'infrastructure

```
[tools/call: pilot_context]
→ retourne stack, services, fichiers manquants, prompt complet

[... l'agent génère le Dockerfile et le compose ...]

[tools/call: pilot_generate_dockerfile {"content": "FROM ..."}]
→ pilot écrit Dockerfile sur disque

[tools/call: pilot_generate_compose {"content": "services: ...", "env": "dev"}]
→ pilot écrit docker-compose.dev.yml sur disque

[tools/call: pilot_up]
→ pilot démarre les services
```

### Déploiement complet

```
[tools/call: pilot_preflight {"target": "deploy"}]
→ all_ok: false
→ [HUMAN] registry_creds: export GITHUB_TOKEN=...
→ (agent demande à l'humain, attend confirmation)

[tools/call: pilot_preflight {"target": "deploy"}]
→ all_ok: true : pilot.lock generated

[tools/call: pilot_push {"env": "prod", "tag": "abc1234"}]
→ build linux/amd64, injecte VITE_*, push

[tools/call: pilot_deploy {"env": "prod", "tag": "abc1234"}]
→ pipeline 8 étapes : lock → secrets → sync → hooks → migrations → deploy → hooks → healthcheck

[tools/call: pilot_status {"env": "prod"}]
→ {"services": [{"name": "api", "state": "running", "health": "healthy"}]}
```
