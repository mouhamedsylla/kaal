# Workflow : Intégration Agent AI

## Vue d'ensemble

pilot est conçu pour travailler *avec* les agents AI. L'idée centrale : pilot collecte le contexte du projet, le rend disponible aux agents, et les agents génèrent les fichiers d'infra adaptés à chaque projet spécifique : ou orchestrent tout le cycle de déploiement.

```
Toi                    pilot                    Agent AI
───                    ────                    ────────
pilot up           →    "fichiers manquants"
                       pilot_context        ──► contexte complet
                                          ◄── génère Dockerfile + compose
                       pilot_generate_*         (écrit sur disque)
pilot up           →    docker compose up -d ✓

"déploie en prod" →    pilot_preflight      ──► plan d'action structuré
                       pilot_push               build + push image
                       pilot_deploy             sync + restart services
                                          ◄─── ✓ Deployed
```

---

## Option 1 : MCP (Claude Code / Cursor) : recommandé

### Configuration

`pilot init` crée automatiquement `.mcp.json` à la racine du projet :

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

Claude Code et Cursor démarrent le serveur MCP automatiquement. Les outils pilot sont disponibles sans configuration supplémentaire.

### Génération des fichiers d'infrastructure

Dans Claude Code :
> *"Génère les fichiers d'infrastructure manquants pour ce projet"*

L'agent :
1. Appelle `pilot_context` → reçoit le contexte complet (pilot.yaml, arbre de fichiers, stack, services, fichiers manquants, prompt structuré)
2. Génère un Dockerfile multi-stage optimisé pour le stack détecté (Go, Node, Python...)
3. Génère un `docker-compose.dev.yml` avec services, healthchecks, volumes, env_file
4. Appelle `pilot_generate_dockerfile` et `pilot_generate_compose` → écrit les fichiers sur disque
5. Te dit de lancer `pilot up`

### Déploiement complet piloté par l'agent

> *"Les tests passent, déploie la v2.3 en prod"*

```
Agent: pilot_preflight {"target": "deploy", "env": "prod"}
       → all_ok: false
       → [HUMAN] registry_creds: export GITHUB_TOKEN=...

Toi:   (configures le token)

Agent: pilot_preflight {"target": "deploy", "env": "prod"}
       → all_ok: true

Agent: pilot_push {"env": "prod", "tag": "v2.3"}
       → Injecte VITE_*, patch Dockerfile, build linux/amd64, push

Agent: pilot_deploy {"env": "prod", "tag": "v2.3"}
       → Sync compose + env + nginx/prod.conf → ~/pilot/
       → docker pull + docker compose up -d

Agent: pilot_status {"env": "prod"}
       → {"services": [{"name": "app", "status": "running", "health": "healthy"}]}

Agent: "✓ v2.3 déployée en prod. Tous les services sont healthy."
```

Tu n'as pas ouvert un terminal. Tu n'as pas tapé une commande.

### Autres demandes possibles

```
"Ajoute un reverse proxy nginx à l'architecture prod"
→ Agent: pilot_context → pilot_generate_compose → pilot_sync → pilot_deploy

"Rollback en prod, le service app répond 500"
→ Agent: pilot_rollback {"env": "prod"} → pilot_status

"Optimise le Dockerfile pour réduire la taille de l'image"
→ Agent: pilot_context → analyse → pilot_generate_dockerfile

"Mets à jour la version de postgres à 17"
→ Agent: pilot_context → pilot_generate_compose (met à jour le service db)
```

---

## Option 2 : Coller dans n'importe quel chat AI

```bash
pilot context
```

Copie le output et colle-le dans ChatGPT, Gemini, Claude.ai, ou n'importe quel LLM :

```markdown
## pilot.yaml

```yaml
apiVersion: pilot/v1
project:
  name: taskflow
  stack: go
  language_version: "1.23"
...
```

## Structure du projet

```
go.mod
go.sum
main.go
internal/
  api/
  db/
```

## Ce qui manque

- **Dockerfile** manquant. Génère un Dockerfile production-ready pour ce projet.
- **docker-compose.dev.yml** manquant. Génère le fichier compose pour cet environnement.
```

### `pilot context --summary`

```bash
pilot context --summary
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

### `pilot_context`

**Le premier outil à appeler.** Retourne le contexte complet du projet.

Paramètres : `env` (optionnel)

Réponse (JSON) :
```json
{
  "pilot_yaml": "...",
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

### `pilot_generate_dockerfile`

Écrit un Dockerfile sur disque.

Paramètres :
- `content` (requis) : contenu complet du Dockerfile
- `path` (optionnel) : chemin de destination (défaut : `Dockerfile`)

**Exigences de génération :** multi-stage build, image finale minimale (distroless ou alpine), utilisateur non-root, healthcheck, COPY sélectif (pas `COPY . .` en final), binaire strippé pour Go.

### `pilot_generate_compose`

Écrit un `docker-compose.<env>.yml` sur disque.

Paramètres :
- `content` (requis) : contenu complet du fichier compose
- `env` (optionnel) : nom de l'environnement (défaut : env actif)
- `path` (optionnel) : chemin custom

**Exigences de génération :** `env_file` depuis `pilot.yaml`, healthchecks sur tous les services, limites de ressources (cpus/memory), volumes nommés pour les données, `depends_on` avec `condition: service_healthy`. Pour stack node avec `VITE_*` : commande avec `--mode <env>`.

### `pilot_preflight`

Lance la checklist pré-déploiement.

Paramètres : `target` (`push` ou `deploy`), `env` (optionnel)

Réponse :
```json
{
  "all_ok": false,
  "checks": [
    {"name": "registry_creds", "ok": false, "message": "GITHUB_TOKEN not set"},
    {"name": "vps_docker_group", "ok": true}
  ],
  "next_steps": [
    {"type": "HUMAN", "action": "export GITHUB_TOKEN=<token>"},
    {"type": "AGENT", "tool": "pilot_push"}
  ]
}
```

L'agent suit `next_steps[]` dans l'ordre. Les étapes `[HUMAN]` bloquent : l'agent demande l'action à l'utilisateur. Les étapes `[AGENT]` sont exécutées automatiquement.

### `pilot_push`

Build + push de l'image.

Paramètres : `tag` (optionnel), `env` (optionnel), `no_cache` (optionnel), `platform` (optionnel)

Comportement automatique :
- Détecte macOS ARM64 → build `linux/amd64`
- Stack node : injecte `VITE_*`/`NEXT_PUBLIC_*`/`REACT_APP_*` depuis l'env file en `--build-arg`
- Patch transparent du Dockerfile si des `ARG` manquent

### `pilot_deploy`

Déploie sur la cible configurée.

Paramètres : `env` (optionnel), `tag` (optionnel)

Sync automatique vers `~/pilot/` : compose files, env files, fichiers bind-mount.

### `pilot_rollback`

Revient à la version précédente (ou une version explicite).

Paramètres : `env` (optionnel), `version` (optionnel)

### `pilot_setup`

Ajoute le user deploy au groupe docker sur le VPS.

Paramètres : `env` (optionnel)

À appeler quand `pilot_preflight` retourne `vps_docker_group: false`.

### `pilot_sync`

Pousse les fichiers de config vers le VPS sans redéployer.

Paramètres : `env` (optionnel)

### `pilot_up` / `pilot_down`

Démarre / arrête l'environnement local.

### `pilot_status`

Retourne l'état complet en JSON (local + remote).

### `pilot_logs`

Retourne les logs d'un service.

Paramètres : `service`, `lines` (défaut : 100), `since` (ex: `5m`, `1h`), `env`

---

## Pourquoi pas de templates statiques ?

Les templates statiques produisent des Dockerfiles génériques :

```dockerfile
# Template générique Go : peut fonctionner, mais :
FROM golang:1.23
WORKDIR /app
COPY . .
RUN go build -o main .
CMD ["./main"]
```

Un agent qui lit `pilot_context` peut générer quelque chose d'adapté :

```dockerfile
# Généré par l'agent après avoir lu pilot.yaml, go.mod, la structure du projet :
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
HEALTHCHECK --interval=30s CMD ["/api", "health"]
ENTRYPOINT ["/api"]
```

Multi-stage, distroless, pas de CGO, binaire strippé, healthcheck : adapté à ce projet spécifique.

**L'agent ne devine pas. pilot lui dit exactement ce qui existe, ce qui manque, et comment le faire.**
