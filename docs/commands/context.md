# pilot context

Affiche le contexte complet du projet pour les agents AI.

```
pilot context [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--summary` | Affiche un résumé court au lieu du prompt complet |
| `--env`, `-e` | Environnement cible pour le contexte |

## Utilisation

### Prompt complet (à coller dans un chat AI)

```bash
pilot context
```

Affiche un document Markdown structuré avec :
- Contenu de `pilot.yaml`
- Arbre de fichiers du projet
- Fichiers clés détectés (go.mod, package.json, requirements.txt, etc.)
- Dockerfiles existants (avec leur contenu)
- Stack, version et environnement actif
- Services définis dans `pilot.yaml`
- Ce qui manque (Dockerfile, compose files)
- Prompt structuré indiquant à l'agent exactement quoi générer et avec quelles contraintes

### Résumé court

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

## Ce que le prompt contient pour l'agent

Pour un projet Node avec des variables compile-time, le prompt inclut :

```
## Variables compile-time

registry.build_args déclare : VITE_API_URL, VITE_APP_ENV
Ces variables doivent être déclarées en ARG puis ENV dans le builder stage
du Dockerfile, avant la commande RUN npm run build.

ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
ARG VITE_APP_ENV
ENV VITE_APP_ENV=$VITE_APP_ENV
```

Et pour le compose :

```
## Règles obligatoires pour le compose

- Injecter env_file: .env.dev (depuis environments.dev.env_file dans pilot.yaml)
- Pour les services Node : commande avec --mode dev (npm run dev -- --mode dev)
```

## Relation avec pilot up

`pilot up` affiche automatiquement les 40 premières lignes du contexte quand des fichiers manquent, avec les instructions pour l'agent AI et un lien vers `pilot context` pour le prompt complet.

## Relation avec le MCP

`pilot context` est l'équivalent CLI de l'outil MCP `pilot_context`. Utilise `pilot context` pour les chats AI manuels (ChatGPT, Claude.ai), `pilot_context` via MCP pour Claude Code ou Cursor.

## Voir aussi

- [Workflow agent AI](../workflows/ai-agent.md) : comment l'agent utilise ce contexte
- [`pilot_context` via MCP](../internals/mcp-server.md) : outil MCP correspondant
