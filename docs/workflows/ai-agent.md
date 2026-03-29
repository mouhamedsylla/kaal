# Workflow : Intégration AI agent

## Vue d'ensemble

kaal est conçu pour travailler *avec* les agents AI. L'idée centrale : kaal collecte le contexte du projet, le rend disponible aux agents, et les agents génèrent les fichiers d'infra adaptés à chaque projet spécifique.

```
Toi                  kaal                Agent AI
───                  ────                ────────
kaal up         →    "fichiers manquants"
                     kaal_context    ──► contexte complet
                                    ◄── génère Dockerfile
                     kaal_generate_dockerfile (écrit sur disque)
                                    ◄── génère docker-compose
                     kaal_generate_compose (écrit sur disque)
kaal up         →    docker compose up -d ✓
```

---

## Option 1 : MCP (Claude Code / Cursor) — recommandé

### Configuration

Crée `.mcp.json` à la racine du projet (kaal init le crée automatiquement) :

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

kaal démarre en tant que serveur MCP quand Claude Code ou Cursor s'ouvre dans ce dossier. Les outils kaal sont disponibles automatiquement — aucune configuration supplémentaire.

### Utilisation

Dans Claude Code, dis simplement :
> "Génère les fichiers d'infrastructure manquants pour ce projet"

L'agent :
1. Appelle `kaal_context` → reçoit le contexte complet (kaal.yaml, arbre de fichiers, stack détecté, services définis, ce qui manque)
2. Analyse le contexte — comprend que c'est un projet Go 1.23 avec postgres et redis
3. Génère un Dockerfile multi-stage optimisé pour Go
4. Génère un `docker-compose.dev.yml` avec les bons services, healthchecks, volumes
5. Appelle `kaal_generate_dockerfile` et `kaal_generate_compose` → écrit les fichiers
6. Te dit de lancer `kaal up`

### Autres demandes possibles

```
"Ajoute un service RabbitMQ au docker-compose dev"
"Optimise le Dockerfile pour réduire la taille de l'image"
"Génère le docker-compose.prod.yml avec des limites de ressources"
"Mets à jour la version de postgres à 17"
```

---

## Option 2 : Coller dans n'importe quel chat AI

```bash
kaal context
```

Copie le output. Il ressemble à ça :

```markdown
Here is the full context of this kaal project.

## kaal.yaml

```yaml
apiVersion: kaal/v1
project:
  name: taskflow
  stack: go
  language_version: "1.23"
...
```

## Project structure

```
go.mod
go.sum
main.go
internal/
  api/
  db/
  handlers/
```

## Key files detected

- go.mod
- go.sum

## Stack

- Language: go 1.23
- Active environment: dev

## Services defined in kaal.yaml

```yaml
api:
  type: app
  port: 8080
db:
  type: postgres
  version: "16"
```

## What is needed

- **Dockerfile** is missing. Please generate a production-ready Dockerfile for this project.
- **docker-compose.dev.yml** is missing. Please generate a docker-compose file for this environment.
```

Colle ça dans ChatGPT, Gemini, Claude.ai, ou n'importe quel LLM. Demande les fichiers. Crée-les manuellement.

### `kaal context --summary`

Pour une vue courte (utile pour vérifier rapidement) :

```bash
kaal context --summary
```

```
Project:  taskflow
Stack:    go 1.23
Env:      dev

Services:
  api          type=app        port=8080
  db           type=postgres
  cache        type=redis
```

---

## Outils MCP exposés

### `kaal_context`

**Le premier outil à appeler.** Retourne le contexte complet du projet.

Paramètres :
- `env` (optionnel) — environnement cible (défaut : env actif)

Réponse (JSON) :
```json
{
  "kaal_yaml": "...",
  "stack": "go",
  "language_version": "1.23",
  "file_tree": "...",
  "key_files": ["go.mod", "go.sum"],
  "existing_dockerfiles": [],
  "existing_compose_files": [],
  "missing_dockerfile": true,
  "missing_compose": true,
  "active_env": "dev",
  "agent_prompt": "...",
  "services": {...},
  "environments": {...}
}
```

### `kaal_generate_dockerfile`

Écrit un Dockerfile sur disque.

Paramètres :
- `content` (requis) — contenu complet du Dockerfile
- `path` (optionnel) — chemin de destination (défaut : `Dockerfile`)

### `kaal_generate_compose`

Écrit un `docker-compose.<env>.yml` sur disque.

Paramètres :
- `content` (requis) — contenu complet du fichier docker-compose
- `env` (optionnel) — nom de l'environnement (défaut : env actif)
- `path` (optionnel) — chemin de destination custom

### `kaal_up`

Démarre l'environnement local.

Paramètres :
- `env` (optionnel)
- `services` (optionnel) — services spécifiques à démarrer

### `kaal_down`

Arrête l'environnement local.

### `kaal_status`

Retourne l'état complet en JSON (local + remote).

### `kaal_logs`

Retourne les logs d'un service.

Paramètres :
- `service` — nom du service
- `lines` — nombre de lignes (défaut : 100)
- `since` — depuis (ex: `5m`, `1h`)

### `kaal_push`

Build + push de l'image.

Paramètres :
- `tag` — tag de l'image

### `kaal_deploy`

Déploie sur une cible.

Paramètres :
- `env` — environnement cible
- `tag` — tag à déployer

### `kaal_rollback`

Revient à la version précédente.

### `kaal_config_get` / `kaal_config_set`

Lit/modifie `kaal.yaml` avec une notation dot.

```
kaal_config_get: { "key": "project.name" }
kaal_config_set: { "key": "services.db.version", "value": "17" }
```

---

## Pourquoi pas de génération statique ?

Les templates statiques produisent des Dockerfiles génériques qui ne correspondent pas à ton projet :

```dockerfile
# Template générique Go — peut fonctionner, mais :
FROM golang:1.23
WORKDIR /app
COPY . .
RUN go build -o main .
CMD ["./main"]
```

Un agent qui lit ton code peut faire mieux :

```dockerfile
# Généré par l'agent après avoir lu go.mod, le code, les dépendances :
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/api ./cmd/api

FROM gcr.io/distroless/static-debian12
COPY --from=builder /bin/api /api
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
```

Multi-stage, distroless, pas de CGO, binaire strippé — adapté à ce projet spécifique.
