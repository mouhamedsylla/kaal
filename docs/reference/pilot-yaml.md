# pilot.yaml : Référence complète

`pilot.yaml` est la source de vérité de ton projet. Tous les comportements de pilot en dérivent.

---

## Structure générale

```yaml
apiVersion: pilot/v1

project:
  name: mon-projet
  stack: go
  language_version: "1.23"

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/mon-projet

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
    provider: upstash

environments:
  dev:
    runtime: compose
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
    port: 22
```

---

## `project`

| Champ              | Type   | Requis | Description |
|--------------------|--------|--------|-------------|
| `name`             | string | oui    | Nom du projet. Utilisé comme préfixe des ressources Docker. |
| `stack`            | string | non    | Langage détecté ou déclaré (`go`, `node`, `python`, `rust`, `java`). Utilisé par l'agent AI pour générer le Dockerfile adapté. |
| `language_version` | string | non    | Version du langage (ex: `"1.23"`, `"20"`, `"3.12"`). |

Si `stack` et `language_version` sont absents, pilot les détecte automatiquement depuis `go.mod`, `package.json`, `pyproject.toml`, etc.

---

## `registry`

| Champ      | Type   | Requis | Description |
|------------|--------|--------|-------------|
| `provider` | string | non    | `ghcr`, `dockerhub`, `ecr`, `gcr`, `acr`, `custom` |
| `image`    | string | non    | Nom complet de l'image (ex: `ghcr.io/user/mon-projet`) |

### Variables d'environnement pour l'authentification

| Provider    | Variables requises |
|-------------|-------------------|
| `ghcr`      | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom`    | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |
| `ecr`       | Credentials AWS standard (`AWS_*`) |

---

## `services`

Chaque service est une clé sous `services:`. Le nom de la clé devient le nom du service dans docker-compose.

### Champs communs à tous les services

| Champ      | Type   | Requis | Description |
|------------|--------|--------|-------------|
| `type`     | string | oui    | Type du service (voir catalogue ci-dessous) |
| `hosting`  | string | non    | Mode d'hébergement : `container` (défaut), `managed`, `local-only` |
| `provider` | string | non    | Fournisseur cloud (si `hosting: managed`) |

### Modes d'hébergement (`hosting`)

| Mode | Description | Compose généré |
|------|-------------|----------------|
| `container` | Service Docker Compose (défaut) | Bloc service avec image, volumes, healthcheck |
| `managed` | Service cloud externe | **Aucun bloc** : variables d'env uniquement dans l'env_file |
| `local-only` | Dev uniquement | Présent dans `docker-compose.dev.yml`, absent de la prod |

> **Important pour l'agent IA :** quand `hosting: managed`, l'agent ne doit **jamais** générer
> de bloc de service dans le compose. Le service est fourni par un tiers : ton app s'y connecte
> via les variables d'env.

---

## Catalogue des services

### `app` : Application principale

```yaml
services:
  api:
    type: app
    port: 8080              # Port exposé (optionnel)
    dockerfile: Dockerfile  # Chemin custom (défaut: ./Dockerfile)
    image: ""               # Image externe (remplace le build local)
```

### `worker` : Processus de fond

```yaml
services:
  queue-worker:
    type: worker
    dockerfile: worker/Dockerfile
```

Identique à `app` mais sans port exposé. Utile pour les consommateurs de queue, crons, etc.

### `postgres` : PostgreSQL

```yaml
services:
  db:
    type: postgres
    version: "16"       # Défaut: "16"
    port: 5432          # Défaut: 5432
    hosting: container  # ou managed
    provider: neon      # si hosting: managed
```

| Fournisseur | Variables d'env |
|-------------|-----------------|
| container   | `DB_NAME`, `DB_USER`, `DB_PASSWORD` |
| `neon`      | `DATABASE_URL` |
| `supabase`  | `DATABASE_URL`, `SUPABASE_URL`, `SUPABASE_ANON_KEY` |
| `railway`   | `DATABASE_URL` |
| `render`    | `DATABASE_URL` |

### `mysql` : MySQL

```yaml
services:
  db:
    type: mysql
    version: "8"        # Défaut: "8"
    hosting: container
    provider: planetscale  # si hosting: managed
```

Fournisseurs managés : `planetscale`, `railway`, `render`

### `mongodb` : MongoDB

```yaml
services:
  db:
    type: mongodb
    version: "7"        # Défaut: "7"
    hosting: container
    provider: atlas     # si hosting: managed
```

| Fournisseur | Variables d'env |
|-------------|-----------------|
| container   | `MONGO_URI` (ex: `mongodb://user:pass@db:27017`) |
| `atlas`     | `MONGO_URI` (ex: `mongodb+srv://...`) |
| `railway`   | `MONGO_URI` |

### `redis` : Redis

```yaml
services:
  cache:
    type: redis
    version: "7"        # Défaut: "7"
    hosting: container
    provider: upstash   # si hosting: managed
```

| Fournisseur | Variables d'env |
|-------------|-----------------|
| container   | `REDIS_PASSWORD` (optionnel) |
| `upstash`   | `UPSTASH_REDIS_REST_URL`, `UPSTASH_REDIS_REST_TOKEN` |
| `railway`   | `REDIS_URL` |
| `render`    | `REDIS_URL` |

Health check (container) : `redis-cli ping`

### `rabbitmq` : RabbitMQ

```yaml
services:
  queue:
    type: rabbitmq
    version: "3"        # Défaut: "3"
```

Image : `rabbitmq:3-management-alpine` (inclut l'interface web sur le port 15672).
Non manageable (container uniquement).

### `nats` : NATS

```yaml
services:
  messaging:
    type: nats
```

Image : `nats:alpine`, port 4222. Non manageable (container uniquement).

### `kafka` : Apache Kafka

```yaml
services:
  events:
    type: kafka
    hosting: container
    provider: confluent  # si hosting: managed
```

| Fournisseur  | Variables d'env |
|--------------|-----------------|
| container    | `KAFKA_BOOTSTRAP_SERVERS` (ex: `kafka:9092`) |
| `confluent`  | `KAFKA_BOOTSTRAP_SERVERS`, `KAFKA_API_KEY`, `KAFKA_API_SECRET` |
| `upstash-kafka` | `UPSTASH_KAFKA_REST_URL`, `UPSTASH_KAFKA_REST_USERNAME`, `UPSTASH_KAFKA_REST_PASSWORD` |

### `elasticsearch` : Elasticsearch

```yaml
services:
  search:
    type: elasticsearch
    version: "8"
```

Non manageable (container uniquement).

### `storage` : Stockage objet (S3-compatible)

```yaml
services:
  files:
    type: storage
    hosting: managed
    provider: cloudflare-r2
```

| Fournisseur        | Variables d'env |
|--------------------|-----------------|
| `cloudflare-r2`    | `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET_NAME` |
| `aws-s3`           | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, `S3_BUCKET_NAME` |
| `supabase-storage` | `SUPABASE_URL`, `SUPABASE_SERVICE_KEY` |

### `nginx` : Reverse proxy Nginx

```yaml
services:
  proxy:
    type: nginx
```

Image : `nginx:alpine`, port 80. Non manageable.

### `traefik` : Traefik

```yaml
services:
  proxy:
    type: traefik
```

Non manageable.

---

## `environments`

Chaque clé sous `environments:` est un nom d'environnement (`dev`, `staging`, `prod`, etc.).

```yaml
environments:
  dev:
    runtime: compose          # compose | k3d | lima (défaut: compose)
    env_file: .env.dev
    resources:
      cpus: "0.5"
      memory: 512M

  staging:
    target: vps-staging
    env_file: .env.staging
    secrets:
      provider: local
      refs:
        DATABASE_URL: DATABASE_URL

  prod:
    target: vps-prod
    secrets:
      provider: aws_sm
      refs:
        DATABASE_URL: prod/database/url
        SECRET_KEY: prod/app/secret-key
```

| Champ       | Type   | Description |
|-------------|--------|-------------|
| `runtime`   | string | Moteur d'exécution local. `compose` = Docker Compose, `k3d` = Kubernetes local. |
| `env_file`  | string | Fichier `.env` chargé par docker-compose. pilot avertit s'il est absent (non bloquant). |
| `target`    | string | Nom d'une cible dans `targets:`. Utilisé par `pilot deploy`. |
| `resources` | objet  | Limites de ressources appliquées à tous les services de cet environnement. |
| `secrets`   | objet  | Configuration du gestionnaire de secrets pour cet environnement. |

### `resources`

```yaml
resources:
  cpus: "0.5"     # Nombre de CPUs (string, ex: "0.5", "1.0", "2")
  memory: 512M    # Mémoire (ex: 512M, 1G, 2048M)
```

### `secrets`

```yaml
secrets:
  provider: local     # ou aws_sm, gcp_sm
  refs:
    ENV_VAR_NAME: secret-path-or-key
```

- `local` : lit le secret depuis le fichier `env_file`
- `aws_sm` : appelle AWS Secrets Manager avec le path fourni
- `gcp_sm` : appelle GCP Secret Manager

---

## `targets`

Les cibles sont les destinations de déploiement (utilisées par `pilot deploy`).

### Type `vps` : VPS via SSH

```yaml
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot
    port: 22            # Défaut: 22
    resources:
      cpus: "2"
      memory: 4G
```

### Type `k8s` : Cluster Kubernetes

```yaml
targets:
  cluster-prod:
    type: k8s
    cluster: prod-k3s
    region: eu-west-1
```

### Types cloud (roadmap)

```yaml
targets:
  aws-prod:
    type: aws
    region: eu-west-1
    cluster: prod-eks
```

---

## Exemple complet

```yaml
apiVersion: pilot/v1

project:
  name: taskflow
  stack: go
  language_version: "1.23"

registry:
  provider: ghcr
  image: ghcr.io/mouhamedsylla/taskflow

services:
  api:
    type: app
    port: 8080
  worker:
    type: worker
    dockerfile: worker/Dockerfile
  db:
    type: postgres
    version: "16"
    hosting: managed
    provider: neon
  cache:
    type: redis
    hosting: managed
    provider: upstash
  queue:
    type: rabbitmq
    version: "3"
    hosting: container
  files:
    type: storage
    hosting: managed
    provider: cloudflare-r2

environments:
  dev:
    runtime: compose
    env_file: .env.dev
    resources:
      cpus: "1"
      memory: 1G

  staging:
    runtime: compose
    target: vps-staging
    env_file: .env.staging

  prod:
    runtime: compose
    target: vps-prod
    secrets:
      provider: aws_sm
      refs:
        DATABASE_URL: prod/taskflow/database-url

targets:
  vps-staging:
    type: vps
    host: 10.0.0.10
    user: deploy
    key: ~/.ssh/id_pilot

  vps-prod:
    type: vps
    host: 10.0.0.20
    user: deploy
    key: ~/.ssh/id_pilot
    resources:
      cpus: "4"
      memory: 8G
```

L'agent IA qui lit ce `pilot.yaml` générera un compose avec :
- `rabbitmq` : seul container (les autres services de données sont managés)
- **Pas de bloc** pour `db`, `cache`, `files` : ils sont fournis par Neon, Upstash, Cloudflare R2
- Les variables d'env correctes pour chaque fournisseur dans l'`env_file`
