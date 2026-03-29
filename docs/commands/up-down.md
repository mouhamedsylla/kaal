# kaal up / kaal down

## kaal up

Démarre les services pour l'environnement actif.

```
kaal up [services...] [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--build`, `-b` | Force le rebuild de l'image avant de démarrer |
| `--env`, `-e` | Environnement cible (surcharge `.kaal-current-env`) |

### Comportement

1. Charge `kaal.yaml` (remonte les répertoires si nécessaire)
2. Résout l'environnement actif (`--env` > `.kaal-current-env` > `"dev"`)
3. Vérifie que l'environnement est défini dans `kaal.yaml`
4. Collecte le contexte projet (`Dockerfile`, `docker-compose.<env>.yml`)
5. **Si des fichiers manquent** : affiche les instructions pour l'agent AI et arrête
6. **Si tout est présent** : exécute `docker compose -f docker-compose.<env>.yml up -d`
7. Affiche les URLs des services

### Quand les fichiers sont manquants

```
✗ Missing: [Dockerfile, docker-compose.dev.yml]

Ask your AI agent to generate them.

  Option 1 — via MCP (Claude Code, Cursor):
    kaal mcp serve is already configured in .mcp.json
    Ask Claude: "Generate the missing infrastructure files for this project"
    Claude will call kaal_context to get the full project details,
    then write the files directly.

  Option 2 — paste this context into any AI chat:

  Here is the full context of this kaal project.
  ...
  (40 premières lignes du prompt, puis "... (N more lines — use 'kaal context' to get the full prompt)")

  Run 'kaal context' to print the full agent prompt
  Then re-run 'kaal up' once the files are created
```

### Exemples

```bash
# Démarrer tous les services de l'env actif
kaal up

# Démarrer seulement api et db
kaal up api db

# Démarrer l'environnement staging
kaal up --env staging

# Forcer le rebuild
kaal up --build
```

---

## kaal down

Arrête les services de l'environnement actif.

```
kaal down [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--volumes`, `-v` | Supprime aussi les volumes nommés (détruit les données) |
| `--env`, `-e` | Environnement à arrêter |

### Comportement

1. Charge `kaal.yaml`
2. Résout l'environnement actif
3. Exécute `docker compose -f docker-compose.<env>.yml down`
4. Si `--volumes` : ajoute le flag `-v` à docker compose (supprime les données postgres, redis, etc.)

### Exemples

```bash
# Arrêter l'env actif (conteneurs préservés, données ok)
kaal down

# Arrêter et supprimer les volumes (ATTENTION: données perdues)
kaal down --volumes

# Arrêter l'env staging
kaal down --env staging
```

---

## Relation avec docker compose

kaal `up` et `down` sont des wrappers autour de `docker compose`. La convention de nommage est :

```
docker-compose.<env>.yml
```

Tu peux toujours utiliser `docker compose` directement si tu préfères — kaal n'ajoute pas de couche supplémentaire incompatible.
