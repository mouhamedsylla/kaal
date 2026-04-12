# Guide : Démarrer avec pilot

Ce guide couvre le chemin complet de la création d'un projet à son premier `pilot up`.
Il suit l'ordre chronologique : init → config → génération des fichiers → démarrage.

---

## Prérequis

- Docker Desktop (ou Docker Engine + Compose plugin)
- `pilot` installé et dans ton `$PATH`
- Un agent IA dans ton éditeur (Claude Code, Cursor) **ou** accès à un chat AI (Claude.ai, ChatGPT…)

---

## 1. Initialiser le projet

```bash
# Nouveau projet vide
mkdir my-app && cd my-app
pilot init my-app

# Projet existant : pilot détecte le stack automatiquement
cd mon-projet-existant
pilot init
```

Le wizard TUI se lance. Il collecte tout ce qu'il faut pour écrire `pilot.yaml`.

### Étapes du wizard

**1 : Nom du projet**
Pré-rempli depuis le dossier. Tu peux le modifier.

**2 : Services**
Multi-select parmi le catalogue complet :

| Service | Description |
|---------|-------------|
| `app` | Ton application (toujours présente) |
| `worker` | Processus de fond (queues, crons) |
| `postgres` | PostgreSQL |
| `mysql` | MySQL |
| `mongodb` | MongoDB |
| `redis` | Redis |
| `rabbitmq` | RabbitMQ (interface admin incluse) |
| `nats` | NATS |
| `kafka` | Apache Kafka |
| `elasticsearch` | Elasticsearch |
| `storage` | Stockage objet (S3-compatible) |
| `nginx` | Reverse proxy Nginx |
| `traefik` | Traefik |

**3 : Services managés** (si des services le permettent)
Pour chaque service pouvant être hébergé en externe (postgres, redis, storage…), tu choisis :
- `container` : Docker Compose local
- `managed` : fournisseur cloud (Neon, Supabase, Upstash, Atlas, Cloudflare R2…)
- `local-only` : uniquement en dev, absent de la prod

Si pilot détecte des indices dans tes fichiers `.env*` (ex: `DATABASE_URL` contenant `neon.tech`), il pré-sélectionne automatiquement le bon fournisseur.

**4 : Environnements**
Multi-select : `dev` (toujours présent), `staging`, `prod`, `test`

**5 : Cible de déploiement**
`none`, `vps`, `k8s`, `aws`, `gcp`

**6 : Registry**
`ghcr`, `dockerhub`, `custom`, `none`

**7 : Credentials du registry** (si non trouvés dans l'environnement)
pilot vérifie si les variables requises sont déjà définies (`GITHUB_TOKEN`, `DOCKER_USERNAME`…). Si non, il les demande maintenant pour les écrire dans `.env.local` (mode 600, non commité).

**8 : Confirmation**
Résumé complet avant écriture. `n` annule sans modifier aucun fichier.

### Résultat

```
✓ pilot.yaml written
✓ .mcp.json written
✓ .env.example updated
✓ .gitignore updated
```

---

## 2. Comprendre ce qui a été créé

### `pilot.yaml` : la source de vérité

```yaml
apiVersion: pilot/v1

project:
  name: my-app
  stack: go
  language_version: "1.23"

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/my-app

services:
  api:
    type: app
    port: 8080
  db:
    type: postgres
    hosting: container      # container | managed | local-only
  cache:
    type: redis
    hosting: managed
    provider: upstash       # fournisseur cloud

environments:
  dev:
    env_file: .env.dev
  prod:
    target: vps-prod
    env_file: .env.prod

targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot
```

Les champs `hosting` et `provider` indiquent à l'agent IA comment générer le compose :
- `container` → bloc de service avec image et volumes
- `managed` → **aucun** bloc de service dans le compose, variables d'env uniquement
- `local-only` → présent dans `docker-compose.dev.yml`, absent de `docker-compose.prod.yml`

### `.mcp.json` : connexion avec ton agent IA

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

Claude Code et Cursor lisent ce fichier et démarrent le serveur MCP automatiquement.
Les outils pilot (`pilot_context`, `pilot_generate_dockerfile`, etc.) sont disponibles sans configuration supplémentaire.

### `.env.example` : variables à remplir

pilot génère les sections par service :

```bash
# postgres (container)
DB_NAME=
DB_USER=
DB_PASSWORD=

# redis/upstash (managed)
# Récupère ces valeurs sur https://console.upstash.com
UPSTASH_REDIS_REST_URL=
UPSTASH_REDIS_REST_TOKEN=
```

Copie ce fichier et remplis-le :

```bash
cp .env.example .env.dev
# Édite .env.dev avec tes vraies valeurs locales
```

---

## 3. Générer les fichiers d'infrastructure

`pilot.yaml` décrit ce dont ton projet a besoin. Il manque encore `Dockerfile` et `docker-compose.dev.yml`. L'agent IA les génère à partir du contexte.

### Option A : via MCP (recommandé)

Dans Claude Code ou Cursor, dis simplement :

> *"Génère les fichiers d'infrastructure manquants pour ce projet"*

L'agent :
1. Appelle `pilot_context` → reçoit le contexte complet
2. Génère un `Dockerfile` multi-stage adapté à ton stack
3. Génère un `docker-compose.dev.yml` avec les bons services (containers uniquement, les services managés sont ignorés)
4. Appelle `pilot_generate_dockerfile` et `pilot_generate_compose` → écrit les fichiers sur disque

> Si tu as défini plusieurs environnements (dev + prod), l'agent génère **tous** les compose manquants en un seul appel.

### Option B : via un chat AI (sans MCP)

```bash
pilot context
```

Copie le contenu et colle-le dans Claude.ai, ChatGPT ou n'importe quel LLM. Demande :

> *"Génère le Dockerfile et le docker-compose.dev.yml pour ce projet"*

Crée les fichiers manuellement avec le contenu généré.

### Option C : `pilot up` détecte et t'oriente

```bash
pilot up
```

Si les fichiers manquent, pilot arrête et affiche exactement ce qui manque avec les instructions pour chacune des deux options ci-dessus.

---

## 4. Démarrer l'environnement

```bash
pilot up
```

Ce qui se passe :
1. Charge `pilot.yaml`
2. Résout l'environnement actif (`dev` par défaut)
3. Vérifie la présence de `Dockerfile` et `docker-compose.dev.yml`
4. Lance `docker compose -f docker-compose.dev.yml up -d`

```
✓ Environment "dev" is up

  api    http://localhost:8080
  db     (interne)
  cache  (interne)
```

---

## 5. Vérifier

```bash
pilot status          # état des services
pilot logs api        # logs du service api
pilot logs --follow   # flux de logs en temps réel
```

---

## Prochaines étapes

- [Ajouter un service après l'init](adding-services.md) : `pilot add` pour étendre l'infra
- [Déployer sur un VPS](../workflows/deploy-vps.md) : pipeline complet
- [Workflow avec l'agent IA](../workflows/ai-agent.md) : déploiement piloté par l'agent
- [Référence pilot.yaml](../reference/pilot-yaml.md) : tous les champs documentés
