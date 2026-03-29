# Concepts & Philosophie

## Le problème que kaal résout

Quand tu développes une application, tu jonglles entre plusieurs réalités :

- En local : tu lances ton app avec `go run .` ou `npm start`, la base de données tourne dans Docker, Redis aussi, les variables d'env sont dans un `.env` que tu gères manuellement.
- En CI : un script fait `docker build`, pousse l'image, mais les tests tournent peut-être sans la vraie infra.
- En prod : Compose sur un VPS, ou Kubernetes, avec des secrets injectés depuis AWS Secrets Manager.

Le résultat : trois configurations différentes, des "ça marche chez moi" fréquents, et beaucoup de friction à chaque déploiement.

**kaal part d'un principe différent : décrire l'infrastructure une seule fois dans `kaal.yaml`, et la simuler fidèlement en local.**

---

## Le modèle mental

```
kaal.yaml
    │
    ├── Ce que tu veux (services, ressources, contraintes)
    │
    ├── En local  → docker-compose.dev.yml  (ou k3d, ou Lima VM)
    │
    ├── En CI     → kaal push + kaal deploy  (mêmes commandes)
    │
    └── En prod   → docker-compose.prod.yml sur VPS, ou k8s manifests
```

**La clé : kaal.yaml est la source de vérité. Tout le reste en est dérivé.**

---

## Principes fondamentaux

### 1. Infra-first, pas app-first

Tu décris tes services (app, postgres, redis, rabbitmq...) dans `kaal.yaml` *avant* d'écrire la configuration Docker. kaal (ou un agent AI) génère les fichiers d'infra à partir de cette description.

```yaml
services:
  api:
    type: app
    port: 8080
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
```

Ce YAML est lisible par un humain *et* par un AI. C'est intentionnel.

### 2. Local = Production

Les environnements locaux simulent l'infra de production avec les mêmes contraintes :

```yaml
environments:
  dev:
    runtime: compose
    resources:
      cpus: "0.5"
      memory: 512M        # Même limite que le VPS prod
  prod:
    target: vps-prod
    resources:
      cpus: "0.5"
      memory: 512M        # Identique
```

Si ton app fonctionne dans les limites mémoire en local, elle fonctionnera en prod.

### 3. AI-natif

kaal est conçu pour être utilisé *avec* un agent AI, pas malgré lui.

- `kaal up` s'arrête et donne à l'agent le contexte complet quand les fichiers d'infra sont manquants.
- `kaal context` exporte un prompt structuré que tu colles dans n'importe quel chat AI.
- Le serveur MCP expose des outils que Claude / Cursor / Copilot peuvent appeler directement.
- L'agent génère le Dockerfile et le docker-compose adaptés à *ton* projet spécifique, pas un template générique.

### 4. Zéro friction local → prod

```bash
kaal up              # Démarre en local
kaal push            # Build + push l'image
kaal deploy --env prod  # Déploie exactement la même infra sur le VPS
```

Les mêmes commandes, dans n'importe quel contexte (terminal, CI, IDE via MCP).

---

## Ce que kaal n'est pas

- **Pas un wrapper Docker** — kaal supporte compose, k3d (Kubernetes local), Lima (VMs légères), et les runtimes cloud.
- **Pas un outil CI** — kaal fournit les primitives (`push`, `deploy`, `rollback`). GitHub Actions ou GitLab CI font le séquençage.
- **Pas un générateur de templates** — Les Dockerfiles et compose files sont générés par un AI qui comprend ton projet spécifique, pas par des templates statiques.
- **Pas Terraform** — kaal gère ton *application* et ses dépendances directes. La création de l'infrastructure cloud (VMs, VPCs, bases de données managées) reste pour Terraform/Pulumi.

---

## Topologies supportées

kaal peut décrire et gérer des architectures allant du plus simple au plus complexe.

### Simple : une app sur un VPS

```
[Dev local]                [VPS]
  docker-compose.dev.yml     docker-compose.prod.yml
  api + postgres + redis  →  api + postgres + redis
```

### Multi-VPS

```yaml
targets:
  app-server:
    type: vps
    host: 10.0.0.1      # Serveur applicatif
  db-server:
    type: vps
    host: 10.0.0.2      # Serveur base de données dédié
```

### Kubernetes (k3s / k3d)

```yaml
environments:
  prod:
    runtime: k8s
targets:
  cluster:
    type: k8s
    cluster: prod-k3s
```

### Cloud managé (roadmap)

```yaml
targets:
  cloud:
    type: aws
    region: eu-west-1
    cluster: prod-eks
```

L'interface `Orchestrator` est la même quelle que soit la cible — kaal abstrait les différences d'implémentation.

---

## Cycle de vie d'un projet kaal

```
kaal init my-app
    │
    └─► kaal.yaml créé (description de l'infra)
        Wizard TUI : services, environnements, target, registry

kaal context  (ou via MCP automatiquement)
    │
    └─► Agent AI reçoit le contexte complet
        Génère Dockerfile + docker-compose.dev.yml
        Écrit les fichiers dans le projet

kaal up
    │
    └─► Lance docker-compose.dev.yml
        Services disponibles localement

# Développement...

kaal push --tag v1.0.0
    │
    └─► docker build
        docker push ghcr.io/user/my-app:v1.0.0

kaal deploy --env prod --tag v1.0.0
    │
    └─► SSH sur le VPS
        docker compose pull
        docker compose up -d
        Health check
```
