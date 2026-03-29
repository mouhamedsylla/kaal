# Workflow : Développement local

Ce document décrit pas à pas ce qui se passe quand tu travailles sur un projet avec kaal, de l'initialisation jusqu'au premier `kaal up`.

---

## Étape 1 : Initialiser le projet

```bash
# Dans un nouveau dossier vide
mkdir my-app && cd my-app
kaal init my-app
```

Le wizard TUI démarre. Il te demande :

1. **Nom du projet** (pré-rempli avec `my-app`)
2. **Services** — multi-select : `app` (toujours présent), `postgres`, `redis`, `rabbitmq`, `nats`, `nginx`, `mongodb`, `mysql`
3. **Environnements** — multi-select : `dev` (toujours présent), `staging`, `prod`, `test`
4. **Target de déploiement** — `none`, `vps`, `k8s`, `aws`, `gcp`
5. **Registry** — `ghcr`, `dockerhub`, `custom`, `none`
6. **Confirmation** — résumé avant écriture

Résultat : `kaal.yaml` créé à la racine.

### Initialiser dans un projet existant

```bash
cd mon-projet-existant
kaal init
```

kaal détecte le stack depuis `go.mod`, `package.json`, `Cargo.toml`, etc. et pré-remplit le wizard. Il ne modifie aucun fichier existant sauf `kaal.yaml`.

---

## Étape 2 : Générer les fichiers d'infra

À ce stade tu as `kaal.yaml` mais pas de `Dockerfile` ni de `docker-compose.dev.yml`.

### Option A — Via MCP (recommandé avec Claude Code / Cursor)

Le serveur MCP est automatiquement disponible si `.mcp.json` est présent à la racine :

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

Dans Claude Code, dis simplement :
> "Génère les fichiers d'infrastructure manquants pour ce projet"

Claude appelle automatiquement `kaal_context` pour obtenir le contexte, génère le Dockerfile et le docker-compose adaptés à ton projet spécifique, puis les écrit via `kaal_generate_dockerfile` et `kaal_generate_compose`.

**Ce que l'agent reçoit via `kaal_context` :**
- Contenu complet de `kaal.yaml`
- Arbre de fichiers du projet (3 niveaux)
- Fichiers clés détectés (`go.mod`, `package.json`...)
- Dockerfiles existants (avec leur contenu)
- Liste précise de ce qui manque
- Stack et version détectés

### Option B — Coller le contexte dans n'importe quel AI chat

```bash
kaal context
```

Copie le output et colle-le dans ChatGPT, Claude.ai, Gemini, etc. Demande de générer le Dockerfile et le docker-compose. Crée les fichiers manuellement avec le contenu généré.

### Option C — `kaal up` t'indique quoi faire

```bash
kaal up
```

Si les fichiers manquent, kaal arrête et affiche exactement ce qui manque + les instructions pour les deux options ci-dessus.

---

## Étape 3 : Démarrer l'environnement

```bash
kaal up
```

**Ce qui se passe en détail :**

1. `config.Load(".")` — cherche `kaal.yaml` dans le dossier courant, puis remonte
2. `env.Active("")` — lit `.kaal-current-env` ou utilise `dev` par défaut
3. `kaalctx.Collect(env)` — collecte le contexte (pour vérifier les fichiers manquants)
4. Vérifie que `Dockerfile` existe (ou qu'un `dockerfile:` custom est défini)
5. Vérifie que `docker-compose.dev.yml` existe
6. Si tout est OK : `docker compose -f docker-compose.dev.yml up -d`
7. Affiche les URLs des services

```
✓ Environment "dev" is up

  api            http://localhost:8080
  db             (interne, pas exposé)
  cache          (interne, pas exposé)

kaal logs --follow   stream logs
kaal down            stop services
kaal status          inspect services
```

### Démarrer seulement certains services

```bash
kaal up api db       # Démarre uniquement api et db
```

### Forcer le rebuild de l'image

```bash
kaal up --build      # Rebuild avant de démarrer
```

---

## Étape 4 : Travailler

```bash
# Voir les logs en temps réel
kaal logs --follow
kaal logs api --follow

# Vérifier l'état des services
kaal status

# Switcher d'environnement
kaal env use staging
kaal up               # Démarre l'environnement staging
```

---

## Étape 5 : Arrêter

```bash
kaal down             # Arrête les conteneurs (données préservées)
kaal down --volumes   # Arrête ET supprime les volumes (données perdues)
```

---

## Gestion des variables d'environnement

kaal ne gère pas les secrets en local — il utilise les fichiers `.env` standard.

```bash
# kaal.yaml
environments:
  dev:
    env_file: .env.dev

# .env.dev
DATABASE_URL=postgresql://postgres:postgres@db:5432/dev_db
SECRET_KEY=dev-secret-key-not-for-prod
```

Si `.env.dev` est absent au moment de `kaal up`, kaal affiche un avertissement mais démarre quand même. Les services peuvent échouer s'ils ont besoin de variables définies dans ce fichier.

---

## Convention de nommage des fichiers

| Fichier | Description |
|---------|-------------|
| `kaal.yaml` | Source de vérité du projet |
| `docker-compose.<env>.yml` | Compose généré pour l'environnement `<env>` |
| `Dockerfile` | Image de l'application (généré par l'agent AI) |
| `.env.<env>` | Variables d'environnement locales (jamais commité) |
| `.kaal-current-env` | Environnement actif persisté (committé ou non, selon préférence) |
