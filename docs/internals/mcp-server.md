# Internals : Serveur MCP

## Protocole

kaal implémente le **Model Context Protocol (MCP)** en JSON-RPC 2.0 sur stdin/stdout.

- **Pas de port réseau** — communication par pipe standard
- **Pas de processus séparé** — `kaal mcp serve` *est* le serveur
- **Intégration IDE** — Claude Code et Cursor gèrent le cycle de vie du processus

---

## Démarrage

```bash
kaal mcp serve
```

Le processus lit les requêtes JSON-RPC depuis stdin, écrit les réponses sur stdout.

**Configuration client (`.mcp.json`) :**

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

L'IDE démarre `kaal mcp serve` dans le dossier du projet. `cwd` est crucial — kaal lit `kaal.yaml` depuis le répertoire courant.

---

## Structure des messages

### Requête (stdin)

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "kaal_context",
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
        "text": "{\"kaal_yaml\": \"...\", \"stack\": \"go\", ...}"
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
    "content": [{"type": "text", "text": "kaal_context: kaal.yaml not found"}],
    "isError": true
  }
}
```

Note : les erreurs d'outil retournent un `result` avec `isError: true`, pas un champ `error` JSON-RPC — conformément à la spec MCP.

---

## Méthodes supportées

| Méthode | Description |
|---------|-------------|
| `initialize` | Handshake initial — retourne les capabilities du serveur |
| `tools/list` | Liste tous les outils disponibles avec leur schéma |
| `tools/call` | Appelle un outil avec les paramètres donnés |

---

## Outils implémentés

| Outil | Handler | Statut |
|-------|---------|--------|
| `kaal_context` | `handlers.HandleContext` | ✅ |
| `kaal_generate_dockerfile` | `handlers.HandleGenerateDockerfile` | ✅ |
| `kaal_generate_compose` | `handlers.HandleGenerateCompose` | ✅ |
| `kaal_init` | stub | 🔲 |
| `kaal_env_switch` | stub | 🔲 |
| `kaal_up` | stub | 🔲 |
| `kaal_down` | stub | 🔲 |
| `kaal_push` | stub | 🔲 |
| `kaal_deploy` | stub | 🔲 |
| `kaal_rollback` | stub | 🔲 |
| `kaal_sync` | stub | 🔲 |
| `kaal_status` | stub | 🔲 |
| `kaal_logs` | stub | 🔲 |
| `kaal_config_get` | stub | 🔲 |
| `kaal_config_set` | stub | 🔲 |
| `kaal_secrets_inject` | stub | 🔲 |

---

## Architecture du package

```
internal/mcp/
├── server.go        # Boucle I/O + dispatch JSON-RPC
├── tools.go         # Définitions Tool + registerAll()
├── handlers.go      # Wiring des vars HandlerFunc
└── handlers/
    ├── stub.go      # Stub générique
    └── context.go   # HandleContext, HandleGenerateDockerfile, HandleGenerateCompose
```

### `HandlerFunc`

```go
type HandlerFunc func(ctx context.Context, params map[string]any) (any, error)
```

Chaque handler reçoit les arguments de l'outil et retourne n'importe quelle valeur sérialisable en JSON, ou une erreur.

### Ajouter un handler

1. Créer `internal/mcp/handlers/<feature>.go`
2. Implémenter la fonction avec la signature `HandlerFunc`
3. Dans `handlers.go`, remplacer le stub :
   ```go
   // Avant
   var handleUp HandlerFunc = handlers.Stub("kaal_up")
   // Après
   var handleUp HandlerFunc = handlers.HandleUp
   ```

---

## Workflow typique d'un agent

```
[initialize]
→ kaal répond avec capabilities

[tools/list]
→ kaal liste les 16 outils avec leurs schémas

[tools/call: kaal_context]
→ kaal retourne le contexte complet du projet

[... l'agent analyse et génère ...]

[tools/call: kaal_generate_dockerfile]
→ kaal écrit Dockerfile sur disque

[tools/call: kaal_generate_compose]
→ kaal écrit docker-compose.dev.yml sur disque

[tools/call: kaal_up]
→ kaal démarre les services
```
