# Guide : Ajouter un service après l'init

`pilot add` permet d'étendre l'infrastructure d'un projet existant sans tout réinitialiser.
Tu ajoutes un service → `pilot.yaml` est mis à jour → ton agent IA régénère le compose.

---

## Quand utiliser `pilot add`

- Tu as initialisé le projet avec `pilot init` et tu veux ajouter une dépendance
- Tu migres d'un service managé vers un container (ou l'inverse)
- Tu ajoutes un reverse proxy ou un outil de queue en cours de développement

---

## Exemples rapides

```bash
# Wizard interactif complet
pilot add

# Choisir le type en argument, compléter le reste en interactif
pilot add postgres
pilot add redis

# Non-interactif : service managé Neon
pilot add postgres --managed --provider neon --yes

# Non-interactif : Redis container avec nom personnalisé
pilot add redis --name session-store --yes

# Non-interactif : storage Cloudflare R2
pilot add storage --managed --provider cloudflare-r2 --yes
```

---

## Le wizard interactif

Si tu lances `pilot add` sans `--yes`, le wizard te pose 4 questions :

**1 : Type de service**
Tout le catalogue est disponible (postgres, redis, rabbitmq, kafka, elasticsearch, storage, nginx, traefik…).
Si tu passes le type en argument (`pilot add redis`), cette étape est sautée.

**2 : Mode d'hébergement** (si le service peut être managé)

| Choix | Description |
|-------|-------------|
| `container` | Service Docker Compose local |
| `managed` | Fournisseur cloud externe |
| `local-only` | Dev uniquement, ignoré en prod |

**3 : Fournisseur** (si `managed` choisi)
Liste des fournisseurs disponibles pour ce service, avec les variables d'env qu'ils requièrent.

**4 : Nom du service**
Pré-rempli avec le type. Utile si tu veux deux instances du même type (ex: `db-main` et `db-analytics`).

**5 : Confirmation**
Affiche les variables d'env qui seront ajoutées à `.env.example` avant d'écrire quoi que ce soit.

---

## Ce que `pilot add` modifie

### `pilot.yaml`

```yaml
# Avant
services:
  api:
    type: app
    port: 8080

# Après pilot add redis --name session-store
services:
  api:
    type: app
    port: 8080
  session-store:
    type: redis
    hosting: container
```

### `.env.example`

Une section est ajoutée pour les variables du service. Si la section existe déjà, elle n'est pas dupliquée (idempotent) :

```bash
# redis/session-store (container)
# Variables for redis session-store service
REDIS_PASSWORD=
```

Pour un service managé :

```bash
# redis/upstash (managed)
# Récupère ces valeurs sur https://console.upstash.com
UPSTASH_REDIS_REST_URL=
UPSTASH_REDIS_REST_TOKEN=
```

---

## Après avoir ajouté un service

### 1. Remplir les variables d'env

```bash
# Pour un service container
echo "REDIS_PASSWORD=my-local-password" >> .env.dev

# Pour un service managé : récupère les valeurs sur le tableau de bord du fournisseur
echo "UPSTASH_REDIS_REST_URL=https://..." >> .env.dev
echo "UPSTASH_REDIS_REST_TOKEN=..." >> .env.dev
```

### 2. Régénérer le compose

`pilot.yaml` a changé : le compose est maintenant désynchronisé. pilot le détecte :

```bash
pilot up
# → ✗ docker-compose.dev.yml is stale : pilot.yaml has changed since it was generated
#   Ask your AI agent: "Regenerate the compose file for the dev environment"
```

Dans ton agent IA :
> *"Régénère le compose pour l'environnement dev"*

Ou directement :
> *"pilot.yaml a été modifié, mets à jour les fichiers compose pour tous les environnements"*

L'agent appelle `pilot_context`, voit le nouveau service, régénère le compose.

### 3. Relancer

```bash
pilot up
# ✓ Environment "dev" is up
```

---

## Services managés : le flux complet

Exemple : ajouter Neon (PostgreSQL managé) après avoir commencé avec un container local.

```bash
pilot add postgres --managed --provider neon --name db
```

`pilot.yaml` est mis à jour :

```yaml
services:
  db:
    type: postgres
    hosting: managed
    provider: neon
```

`.env.example` reçoit :

```bash
# postgres/neon (managed)
# Récupère ces valeurs sur https://neon.tech/docs/connect/connect-from-any-app
DATABASE_URL=
```

Ensuite :
1. Crée un projet sur neon.tech
2. Copie la connection string dans `.env.dev`
3. Demande à ton agent de régénérer le compose : il ne générera **pas** de bloc `db:` pour les services managés
4. `pilot up` démarre uniquement les containers (ton app se connecte à Neon directement)

---

## Flags de `pilot add`

| Flag | Alias | Description |
|------|-------|-------------|
| `--name` | `-n` | Nom du service dans pilot.yaml (défaut : type) |
| `--managed` | `-m` | Marquer comme service managé |
| `--provider` | `-p` | Fournisseur managé (neon, supabase, upstash, atlas…) |
| `--yes` | `-y` | Non-interactif : accepte les défauts (container) |

---

## Voir aussi

- [Référence `pilot add`](../commands/add.md) : documentation complète de la commande
- [Référence pilot.yaml : services](../reference/pilot-yaml.md#services) : champs hosting et provider
- [Catalogue des services](../reference/pilot-yaml.md#catalogue-des-services) : tous les types et fournisseurs
- [Staleness detection](../troubleshooting/stale-compose.md) : pourquoi pilot.yaml doit rester synchronisé avec le compose
