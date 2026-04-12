# pilot mcp context

Affiche le contexte complet du projet pour les agents AI.

```
pilot mcp context [flags]
```

---

## Flags

| Flag | Description |
|------|-------------|
| `--summary` | Affiche un résumé court au lieu du prompt complet |
| `--env`, `-e` | Environnement cible pour le contexte |

---

## Utilisation

### Prompt complet (à coller dans un chat AI)

```bash
pilot mcp context
```

Affiche un document Markdown structuré avec :
- Contenu de `pilot.yaml`
- Arbre de fichiers du projet
- Fichiers clés détectés (`go.mod`, `package.json`, `requirements.txt`, etc.)
- Dockerfiles existants (avec leur contenu)
- Stack, version et environnement actif
- Services définis dans `pilot.yaml` (avec mode d'hébergement)
- Services managés : section CRITICAL pour l'agent
- Ce qui manque (Dockerfile, compose files, pour tous les environnements)
- Prompt structuré indiquant à l'agent exactement quoi générer et avec quelles contraintes

### Résumé court

```bash
pilot mcp context --summary
```

```
Project:  taskflow
Stack:    go 1.23
Env:      dev

Services:
  api          type=app        port=8080      hosting=container
  db           type=postgres   hosting=managed   provider=neon
  cache        type=redis      hosting=managed   provider=upstash
  queue        type=rabbitmq   hosting=container
```

---

## Relation avec `pilot up`

`pilot up` affiche automatiquement les 40 premières lignes du contexte quand des fichiers
manquent, avec les instructions pour l'agent AI et un renvoi vers `pilot mcp context`
pour le prompt complet.

---

## Relation avec le serveur MCP

`pilot mcp context` est l'équivalent CLI de l'outil MCP `pilot_context`.

| Cas d'usage | Commande |
|-------------|----------|
| Chat AI manuel (ChatGPT, Claude.ai) | `pilot mcp context` → copier-coller |
| Claude Code / Cursor (MCP) | `pilot_context` appelé automatiquement par l'agent |

---

## Voir aussi

- [Workflow agent IA](../workflows/ai-agent.md) : comment l'agent utilise ce contexte
- [`pilot mcp`](mcp.md) : démarrer le serveur MCP
- [Moteur de contexte](../internals/context-engine.md) : fonctionnement interne
