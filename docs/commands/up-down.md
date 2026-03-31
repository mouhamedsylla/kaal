# pilot up / pilot down

## pilot up

Démarre les services pour l'environnement actif.

```
pilot up [services...] [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--build`, `-b` | Force le rebuild de l'image avant de démarrer |
| `--env`, `-e` | Environnement cible (surcharge `.pilot-current-env`) |

### Comportement

1. Charge `pilot.yaml` (remonte les répertoires si nécessaire)
2. Résout l'environnement actif (`--env` > `.pilot-current-env` > `"dev"`)
3. Vérifie que l'environnement est défini dans `pilot.yaml`
4. Collecte le contexte projet (`Dockerfile`, `docker-compose.<env>.yml`)
5. **Si des fichiers manquent** : affiche les instructions pour l'agent AI et arrête
6. **Si tout est présent** : exécute `docker compose -f docker-compose.<env>.yml up -d`
7. Affiche les URLs des services

### Quand les fichiers sont manquants

```
✗ Fichiers manquants : [Dockerfile, docker-compose.dev.yml]

Demande à ton agent AI de les générer.

  Option 1 : via MCP (Claude Code, Cursor) :
    pilot mcp serve est déjà configuré dans .mcp.json
    Dis à Claude : "Génère les fichiers d'infrastructure manquants pour ce projet"
    Claude appellera pilot_context pour obtenir le contexte complet du projet,
    puis écrira les fichiers directement.

  Option 2 : colle ce contexte dans n'importe quel chat AI :

  Voici le contexte complet de ce projet pilot.
  ...
  (40 premières lignes du prompt, puis "... (N autres lignes : utilise 'pilot context' pour le prompt complet)")

  Lance 'pilot context' pour afficher le prompt complet
  Puis relance 'pilot up' une fois les fichiers créés
```

### Exemples

```bash
# Démarrer tous les services de l'env actif
pilot up

# Démarrer seulement api et db
pilot up api db

# Démarrer l'environnement staging
pilot up --env staging

# Forcer le rebuild
pilot up --build
```

---

## pilot down

Arrête les services de l'environnement actif.

```
pilot down [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--volumes`, `-v` | Supprime aussi les volumes nommés (détruit les données) |
| `--env`, `-e` | Environnement à arrêter |

### Comportement

1. Charge `pilot.yaml`
2. Résout l'environnement actif
3. Exécute `docker compose -f docker-compose.<env>.yml down`
4. Si `--volumes` : ajoute le flag `-v` à docker compose (supprime les données postgres, redis, etc.)

### Exemples

```bash
# Arrêter l'env actif (conteneurs supprimés, données préservées)
pilot down

# Arrêter et supprimer les volumes (ATTENTION : données perdues)
pilot down --volumes

# Arrêter l'env staging
pilot down --env staging
```

---

## Relation avec docker compose

`pilot up` et `pilot down` sont des wrappers autour de `docker compose`. La convention de nommage est :

```
docker-compose.<env>.yml
```

Tu peux toujours utiliser `docker compose` directement si tu préfères : pilot n'ajoute pas de couche supplémentaire incompatible.

## Voir aussi

- [`pilot context`](context.md) : obtenir le prompt pour générer les fichiers manquants
- [Workflow développement local](../workflows/local-dev.md)
- [Workflow agent AI](../workflows/ai-agent.md) : générer Dockerfile et compose avec l'agent
