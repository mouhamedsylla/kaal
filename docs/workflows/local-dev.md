# Workflow : Développement local

Ce document décrit pas à pas ce qui se passe quand tu travailles sur un projet avec pilot, de l'initialisation jusqu'au premier `pilot up`.

---

## Étape 1 : Initialiser le projet

```bash
# Dans un nouveau dossier vide
mkdir my-app && cd my-app
pilot init my-app
```

Le wizard TUI démarre. Il te demande :

1. **Nom du projet** (pré-rempli avec `my-app`)
2. **Services** : multi-select depuis le catalogue complet (`app`, `postgres`, `redis`, `rabbitmq`, `kafka`, `mongodb`, `storage`, `nginx`, `traefik`…)
3. **Services managés** : pour chaque service pouvant être hébergé en externe, choix du mode (`container`, `managed`, `local-only`) et du fournisseur cloud (Neon, Supabase, Upstash, Cloudflare R2…). Si pilot trouve des indices dans tes `.env*` existants (ex: `neon.tech` dans `DATABASE_URL`), il pré-sélectionne automatiquement.
4. **Environnements** : multi-select : `dev` (toujours présent), `staging`, `prod`, `test`
5. **Target de déploiement** : `none`, `vps`, `k8s`, `aws`, `gcp`
6. **Registry** : `ghcr`, `dockerhub`, `custom`, `none`
7. **Credentials du registry** : si les variables requises (`GITHUB_TOKEN`, `DOCKER_USERNAME`…) ne sont pas dans l'environnement, pilot les demande maintenant (écrits dans `.env.local` mode 600, gitignored)
8. **Confirmation** : résumé avant écriture : `n` annule sans modifier quoi que ce soit

Résultat : `pilot.yaml`, `.mcp.json`, `.env.example` créés à la racine.

### Initialiser dans un projet existant

```bash
cd mon-projet-existant
pilot init
```

pilot détecte le stack depuis `go.mod`, `package.json`, `Cargo.toml`, etc. et pré-remplit le wizard. Il ne modifie aucun fichier existant sauf `pilot.yaml`.

---

## Étape 2 : Générer les fichiers d'infra

À ce stade tu as `pilot.yaml` mais pas de `Dockerfile` ni de `docker-compose.dev.yml`.

### Option A : Via MCP (recommandé avec Claude Code / Cursor)

Le serveur MCP est automatiquement disponible si `.mcp.json` est présent à la racine :

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

Dans Claude Code, dis simplement :
> "Génère les fichiers d'infrastructure manquants pour ce projet"

Claude appelle automatiquement `pilot_context` pour obtenir le contexte, génère le Dockerfile et le docker-compose adaptés à ton projet spécifique, puis les écrit via `pilot_generate_dockerfile` et `pilot_generate_compose`.

**Ce que l'agent reçoit via `pilot_context` :**
- Contenu complet de `pilot.yaml`
- Arbre de fichiers du projet (3 niveaux, bruit filtré)
- Fichiers clés détectés (`go.mod`, `package.json`...)
- Dockerfiles existants (avec leur contenu)
- Liste précise de ce qui manque
- Stack et version détectés

### Option B : Coller le contexte dans n'importe quel AI chat

```bash
pilot context
```

Copie le output et colle-le dans ChatGPT, Claude.ai, Gemini, etc. Demande de générer le Dockerfile et le docker-compose. Crée les fichiers manuellement avec le contenu généré.

### Option C : `pilot up` t'indique quoi faire

```bash
pilot up
```

Si les fichiers manquent, pilot arrête et affiche exactement ce qui manque + les instructions pour les deux options ci-dessus.

---

## Étape 3 : Démarrer l'environnement

```bash
pilot up
```

**Ce qui se passe en détail :**

1. `config.Load(".")` : cherche `pilot.yaml` dans le dossier courant, puis remonte
2. `env.Active("")` : lit `.pilot-current-env` ou utilise `dev` par défaut
3. Collecte le contexte depuis `internal/mcp/context` (pour vérifier les fichiers manquants)
4. Vérifie que `Dockerfile` existe
5. Vérifie que `docker-compose.dev.yml` existe
6. Si tout est OK : `docker compose -f docker-compose.dev.yml up -d`
7. Affiche les URLs des services

```
✓ Environment "dev" is up

  api            http://localhost:8080
  db             (interne, pas exposé)
  cache          (interne, pas exposé)

pilot logs --follow   stream logs
pilot down            stop services
pilot status          inspect services
```

### Démarrer seulement certains services

```bash
pilot up api db       # Démarre uniquement api et db
```

### Forcer le rebuild de l'image

```bash
pilot up --build      # Rebuild avant de démarrer
```

---

## Étape 4 : Travailler

```bash
# Voir les logs en temps réel
pilot logs --follow
pilot logs api --follow

# Vérifier l'état des services
pilot status

# Comparer les environnements avant de déployer
pilot env diff dev prod

# Switcher d'environnement
pilot env use staging
pilot up               # Démarre l'environnement staging
```

---

## Étape 5 : Arrêter

```bash
pilot down             # Arrête les conteneurs (données préservées)
pilot down --volumes   # Arrête ET supprime les volumes (données perdues)
```

---

## Gestion des variables d'environnement

pilot ne gère pas les secrets en local : il utilise les fichiers `.env` standard.

```yaml
# pilot.yaml
environments:
  dev:
    env_file: .env.dev
```

```bash
# .env.dev
DATABASE_URL=postgresql://postgres:postgres@db:5432/dev_db
SECRET_KEY=dev-secret-key-not-for-prod
```

Si `.env.dev` est absent au moment de `pilot up`, pilot affiche un avertissement mais démarre quand même. Les services peuvent échouer s'ils ont besoin de variables définies dans ce fichier.

---

## Convention de nommage des fichiers

| Fichier | Description |
|---------|-------------|
| `pilot.yaml` | Source de vérité du projet : commiter |
| `pilot.lock` | Plan de déploiement validé : commiter |
| `.mcp.json` | Config agent AI : commiter |
| `.env.example` | Variables requises documentées : commiter |
| `docker-compose.<env>.yml` | Compose généré pour l'environnement `<env>` |
| `Dockerfile` | Image de l'application (généré par l'agent AI) |
| `.env.<env>` | Variables d'environnement locales : **ne pas commiter** |
| `.env.local` | Credentials de registry (créé par le wizard, mode 600) : **ne pas commiter** |
| `.pilot-current-env` | Environnement actif persisté : **ne pas commiter** |
| `.pilot/` | État runtime (compose-meta.json, suspended.json) : **ne pas commiter** |
