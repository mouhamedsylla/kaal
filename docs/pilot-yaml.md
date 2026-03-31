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
    version: "16"

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

### Type `app` : Ton application

```yaml
services:
  api:
    type: app
    port: 8080              # Port exposé (optionnel)
    dockerfile: Dockerfile  # Chemin custom (défaut: ./Dockerfile)
    image: ""               # Image externe (remplace le build local)
```

Quand `image` est vide et `dockerfile` est absent, pilot cherche `./Dockerfile`. Si absent, `pilot up` demande à l'agent de le générer.

### Type `postgres`

```yaml
services:
  db:
    type: postgres
    version: "16"       # Défaut: "16"
    port: 5432          # Défaut: 5432
```

Variables d'env injectées automatiquement :
- `POSTGRES_DB` → `${DB_NAME:-<env>_db}`
- `POSTGRES_USER` → `${DB_USER:-postgres}`
- `POSTGRES_PASSWORD` → `${DB_PASSWORD:-postgres}`

### Type `mysql`

```yaml
services:
  db:
    type: mysql
    version: "8"        # Défaut: "8"
```

### Type `mongodb`

```yaml
services:
  db:
    type: mongodb
    version: "7"        # Défaut: "7"
```

### Type `redis`

```yaml
services:
  cache:
    type: redis
    version: "7"        # Défaut: "7"
```

Health check : `redis-cli ping`

### Type `rabbitmq`

```yaml
services:
  queue:
    type: rabbitmq
    version: "3"        # Défaut: "3"
```

Image : `rabbitmq:3-management-alpine` (inclut l'interface web sur le port 15672)

### Type `nats`

```yaml
services:
  messaging:
    type: nats
```

Image : `nats:alpine`, port 4222.

### Type `nginx`

```yaml
services:
  proxy:
    type: nginx
```

Image : `nginx:alpine`, port 80.

### Image personnalisée (type générique)

```yaml
services:
  meilisearch:
    type: custom
    image: getmeili/meilisearch:v1.5
    port: 7700
```

---

## `environments`

Chaque clé sous `environments:` est un nom d'environnement (`dev`, `staging`, `prod`, etc.).

```yaml
environments:
  dev:
    runtime: compose          # compose | k3d | lima (défaut: compose)
    env_file: .env.dev        # Fichier de variables d'env chargé par compose
    resources:
      cpus: "0.5"
      memory: 512M

  staging:
    runtime: compose
    target: vps-staging       # Cible de déploiement pour pilot deploy
    env_file: .env.staging
    secrets:
      provider: local         # local | aws_sm | gcp_sm
      refs:
        DATABASE_URL: DATABASE_URL

  prod:
    runtime: compose
    target: vps-prod
    secrets:
      provider: aws_sm
      refs:
        DATABASE_URL: prod/database/url
        SECRET_KEY: prod/app/secret-key
```

| Champ       | Type   | Description |
|-------------|--------|-------------|
| `runtime`   | string | Moteur d'exécution local. `compose` = Docker Compose, `k3d` = Kubernetes local, `lima` = VM légère. |
| `env_file`  | string | Fichier `.env` chargé par docker-compose. pilot avertit s'il est absent (non bloquant). |
| `target`    | string | Nom d'une cible dans `targets:`. Utilisé par `pilot deploy`. |
| `resources` | objet  | Limites de ressources appliquées à TOUS les services de cet environnement (section `deploy.resources.limits` dans compose). |
| `secrets`   | objet  | Configuration du gestionnaire de secrets pour cet environnement. |

### `resources`

```yaml
resources:
  cpus: "0.5"     # Nombre de CPUs (string, ex: "0.5", "1.0", "2")
  memory: 512M    # Mémoire (ex: 512M, 1G, 2048M)
```

Les ressources locales limitées = les mêmes contraintes qu'en prod = moins de surprises.

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
    host: 1.2.3.4       # IP ou hostname
    user: deploy        # Utilisateur SSH
    key: ~/.ssh/id_pilot # Chemin vers la clé SSH privée
    port: 22            # Port SSH (défaut: 22)
    resources:          # Ressources disponibles sur ce VPS
      cpus: "2"
      memory: 4G
```

pilot se connecte via SSH, copie les fichiers nécessaires, puis exécute `docker compose up -d`.

### Type `k8s` : Cluster Kubernetes

```yaml
targets:
  cluster-prod:
    type: k8s
    cluster: prod-k3s     # Nom du contexte kubectl
    region: eu-west-1     # Optionnel (pour clusters cloud)
```

### Types cloud (stubs, roadmap)

```yaml
targets:
  aws-prod:
    type: aws
    region: eu-west-1
    cluster: prod-eks

  gcp-prod:
    type: gcp
    project: mon-projet-gcp
    region: europe-west1
    cluster: prod-gke
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
    type: app
    port: 9090
    dockerfile: worker/Dockerfile
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
    version: "7"
  queue:
    type: rabbitmq
    version: "3"

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
    resources:
      cpus: "0.5"
      memory: 512M

  prod:
    runtime: compose
    target: vps-prod
    secrets:
      provider: aws_sm
      refs:
        DATABASE_URL: prod/taskflow/database-url
        SECRET_KEY: prod/taskflow/secret-key
    resources:
      cpus: "1"
      memory: 2G

targets:
  vps-staging:
    type: vps
    host: 10.0.0.10
    user: deploy
    key: ~/.ssh/id_pilot
    port: 22

  vps-prod:
    type: vps
    host: 10.0.0.20
    user: deploy
    key: ~/.ssh/id_pilot
    port: 22
    resources:
      cpus: "4"
      memory: 8G
```
