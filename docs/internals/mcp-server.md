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
| `pilot_push` | `handlers.HandlePush` | ✅ |
| `pilot_deploy` | `handlers.HandleDeploy` | ✅ |
| `pilot_rollback` | `handlers.HandleRollback` | ✅ |
| `pilot_sync` | `handlers.HandleSync` | ✅ |
| `pilot_up` | `handlers.HandleUp` | ✅ |
| `pilot_down` | `handlers.HandleDown` | ✅ |
| `pilot_status` | `handlers.HandleStatus` | ✅ |
| `pilot_logs` | `handlers.HandleLogs` | ✅ |
| `pilot_preflight` | `handlers.HandlePreflight` | ✅ |
| `pilot_setup` | `handlers.HandleSetup` | ✅ |
| `pilot_init` | stub | 🔲 |
| `pilot_env_switch` | stub | 🔲 |
| `pilot_config_get` | stub | 🔲 |
| `pilot_config_set` | stub | 🔲 |

---

## Description des outils principaux

### `pilot_context`

Retourne le contexte complet du projet : stack, services, fichiers existants/manquants, prompt agent, env_file paths, targets non configurés. **Premier outil à appeler avant toute génération.**

Paramètres : `env` (optionnel)

### `pilot_generate_dockerfile`

Écrit un Dockerfile optimisé sur disque. L'agent doit générer un Dockerfile multi-stage, non-root, avec healthcheck, adapté au stack détecté dans `pilot_context`.

Paramètres : `content` (requis), `path` (optionnel, défaut : `Dockerfile`)

### `pilot_generate_compose`

Écrit un `docker-compose.<env>.yml` sur disque. L'agent doit inclure : `env_file` depuis `pilot.yaml`, healthchecks, limites de ressources, commande `--mode <env>` pour Vite/Next.

Paramètres : `content` (requis), `env` (optionnel), `path` (optionnel)

### `pilot_preflight`

Lance la checklist pré-déploiement. Retourne un plan d'action structuré avec `all_ok`, `checks[]`, `next_steps[]`. L'agent suit les `next_steps` dans l'ordre : actions `[HUMAN]` d'abord, puis `[AGENT]`.

Paramètres : `target` (`push` ou `deploy`), `env` (optionnel)

### `pilot_push`

Build + push de l'image. Injecte automatiquement les vars `VITE_*`/`NEXT_PUBLIC_*`/`REACT_APP_*` depuis l'env file. Patch le Dockerfile si des `ARG` manquent.

Paramètres : `tag` (optionnel), `env` (optionnel), `no_cache` (optionnel), `platform` (optionnel)

### `pilot_deploy`

Déploie sur la cible configurée. Sync automatique des fichiers (compose, env, bind-mounts) vers `~/pilot/` sur le VPS avant chaque déploiement.

Paramètres : `env` (optionnel), `tag` (optionnel)

### `pilot_setup`

Ajoute le user deploy au groupe docker sur le VPS via SSH (`sudo usermod -aG docker <user>`). À appeler quand `pilot_preflight` retourne `vps_docker_group: false`.

Paramètres : `env` (optionnel)

### `pilot_sync`

Copie les fichiers de config vers `~/pilot/` sur le VPS sans redéployer : compose files, env files, et tous les fichiers référencés en bind-mount dans le compose.

Paramètres : `env` (optionnel)

---

## Architecture du package

```
internal/mcp/
├── server.go          # Boucle I/O + dispatch JSON-RPC
├── tools.go           # Définitions Tool + registerAll()
├── handlers.go        # Wiring des vars HandlerFunc
└── handlers/
    ├── stub.go        # Stub générique (retourne "not yet implemented")
    ├── context.go     # HandleContext, HandleGenerateDockerfile, HandleGenerateCompose
    └── lifecycle.go   # HandlePush, HandleDeploy, HandleRollback, HandleSync,
                       # HandleUp, HandleDown, HandleStatus, HandleLogs,
                       # HandlePreflight, HandleSetup
```

### `HandlerFunc`

```go
type HandlerFunc func(ctx context.Context, params map[string]any) (any, error)
```

Chaque handler reçoit les arguments de l'outil et retourne n'importe quelle valeur sérialisable en JSON, ou une erreur.

### Ajouter un handler

1. Créer ou éditer `internal/mcp/handlers/<feature>.go`
2. Implémenter la fonction avec la signature `HandlerFunc`
3. Dans `handlers.go`, remplacer le stub :
   ```go
   // Avant
   var handleInit HandlerFunc = handlers.Stub("pilot_init")
   // Après
   var handleInit HandlerFunc = handlers.HandleInit
   ```

---

## Workflow typique d'un agent AI

```
[initialize]
→ pilot répond avec capabilities + liste des outils

[tools/call: pilot_preflight {"target": "deploy"}]
→ pilot vérifie tous les prérequis
→ retourne next_steps[] avec actions [HUMAN] et [AGENT]

[... l'agent suit les next_steps ...]

[tools/call: pilot_push {"env": "prod"}]
→ pilot build linux/amd64, injecte VITE_*, push

[tools/call: pilot_deploy {"env": "prod", "tag": "abc1234"}]
→ pilot sync + docker pull + docker compose up

[tools/call: pilot_status {"env": "prod"}]
→ pilot retourne l'état des services en JSON
```

---

## Workflow de génération d'infrastructure

```
[tools/call: pilot_context]
→ pilot retourne stack, services, fichiers manquants, prompt complet

[... l'agent analyse, génère le Dockerfile et le compose ...]

[tools/call: pilot_generate_dockerfile {"content": "FROM ..."}]
→ pilot écrit Dockerfile sur disque

[tools/call: pilot_generate_compose {"content": "services: ...", "env": "dev"}]
→ pilot écrit docker-compose.dev.yml sur disque

[tools/call: pilot_up]
→ pilot démarre les services
```
