# kaal context

Affiche le contexte complet du projet pour les agents AI.

```
kaal context [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--summary` | Affiche un résumé court au lieu du prompt complet |
| `--env`, `-e` | Environnement cible pour le contexte |

## Utilisation

### Prompt complet (à coller dans un chat AI)

```bash
kaal context
```

Affiche un document Markdown structuré avec :
- Contenu de `kaal.yaml`
- Arbre de fichiers du projet
- Fichiers clés détectés (go.mod, package.json, etc.)
- Dockerfiles existants (avec leur contenu)
- Stack et version détectés
- Services définis
- Ce qui manque explicitement

### Résumé court

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

## Relation avec kaal up

`kaal up` affiche automatiquement les 40 premières lignes du contexte quand des fichiers manquent, avec un lien vers `kaal context` pour le prompt complet.

## Relation avec le MCP

`kaal context` est l'équivalent CLI de l'outil MCP `kaal_context`. Utilise `kaal context` pour les chats AI manuels, `kaal_context` via MCP pour Claude Code / Cursor.
