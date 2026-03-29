# Workflow : CI/CD

## Principe

kaal ne remplace pas GitHub Actions, GitLab CI, ou CircleCI. Il fournit les **primitives** que le CI orchestre.

```
CI Runner
  │
  ├─► kaal push --tag $SHA     # Build + push l'image
  ├─► kaal deploy --env staging # Déploie sur staging
  │   [tests d'intégration...]
  └─► kaal deploy --env prod   # Déploie en prod
```

Les mêmes commandes que tu utilises en local fonctionnent en CI. Pas de script séparé, pas de traduction.

---

## GitHub Actions

### Workflow complet : test → push → deploy

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install kaal
        run: |
          curl -sSL https://github.com/mouhamedsylla/kaal/releases/latest/download/kaal-linux-amd64 -o kaal
          chmod +x kaal
          sudo mv kaal /usr/local/bin/

      - name: Push image
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITHUB_ACTOR: ${{ github.actor }}
        run: kaal push --tag ${{ github.sha }}

      - name: Deploy to staging
        env:
          KAAL_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: kaal deploy --env staging --tag ${{ github.sha }}

      - name: Integration tests
        run: |
          # tes tests ici, contre l'URL staging
          curl -f https://staging.my-app.com/health

      - name: Deploy to prod
        if: success()
        env:
          KAAL_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: kaal deploy --env prod --tag ${{ github.sha }}

      - name: Rollback on failure
        if: failure()
        env:
          KAAL_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: kaal rollback --env prod
```

### Workflow séparé : PR preview (staging automatique)

```yaml
# .github/workflows/preview.yml
name: Preview

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  preview:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Push preview image
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITHUB_ACTOR: ${{ github.actor }}
        run: kaal push --tag pr-${{ github.event.pull_request.number }}

      - name: Deploy to staging
        env:
          KAAL_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: kaal deploy --env staging --tag pr-${{ github.event.pull_request.number }}
```

---

## GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - build
  - deploy-staging
  - test
  - deploy-prod

variables:
  TAG: $CI_COMMIT_SHORT_SHA

build:
  stage: build
  script:
    - kaal push --tag $TAG
  only:
    - main

deploy-staging:
  stage: deploy-staging
  script:
    - kaal deploy --env staging --tag $TAG
  environment:
    name: staging
  only:
    - main

integration-tests:
  stage: test
  script:
    - curl -f https://staging.my-app.com/health
  only:
    - main

deploy-prod:
  stage: deploy-prod
  script:
    - kaal deploy --env prod --tag $TAG
  environment:
    name: production
  when: manual
  only:
    - main
```

---

## Variables d'environnement en CI

kaal lit les credentials depuis les variables d'environnement standard.

### Pour `kaal push`

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Token GitHub (auto-injecté par GitHub Actions) |
| `GITHUB_ACTOR` | Username GitHub (auto-injecté par GitHub Actions) |
| `DOCKER_USERNAME` | Username Docker Hub |
| `DOCKER_PASSWORD` | Password Docker Hub |

### Pour `kaal deploy` (VPS)

| Variable | Description |
|----------|-------------|
| `KAAL_SSH_KEY` | Contenu de la clé SSH privée (PEM) |

kaal écrit la clé dans un fichier temporaire, l'utilise pour SSH, puis la supprime.

### Pour les secrets applicatifs

Si `environments.prod.secrets.provider: aws_sm`, kaal a besoin des credentials AWS :
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`

---

## Ce que le CI fait vs ce que kaal fait

| Responsabilité | CI Runner | kaal |
|----------------|-----------|------|
| Déclencher sur un push | ✓ | |
| Cloner le dépôt | ✓ | |
| Lancer les tests unitaires | ✓ | |
| Construire l'image Docker | | ✓ (`kaal push`) |
| Pousser vers le registry | | ✓ (`kaal push`) |
| Déployer sur le serveur | | ✓ (`kaal deploy`) |
| Rollback si échec | | ✓ (`kaal rollback`) |
| Health check post-deploy | (peut déléguer à kaal) | ✓ (`kaal status`) |
| Notifications Slack/email | ✓ | |
| Gestion des branches/PRs | ✓ | |

---

## Stratégies de déploiement

### Rolling update (défaut)

```bash
kaal deploy --env prod --tag v1.2.0
```

docker compose arrête l'ancien conteneur et démarre le nouveau. Brève indisponibilité (~5s).

### Blue-green (roadmap)

```bash
kaal deploy --env prod --tag v1.2.0 --strategy blue-green
```

kaal maintient deux stacks. Switch du load balancer une fois le nouveau stack healthy.

### Canary (roadmap)

```bash
kaal deploy --env prod --tag v1.2.0 --strategy canary --weight 10
```

10% du trafic vers la nouvelle version, 90% vers l'ancienne.

---

## Bonnes pratiques

**Tags immuables** — utilise toujours `--tag $SHA` (SHA Git court), pas `latest`. `latest` est mutable et rend le rollback ambigu.

```bash
# Bien
kaal push --tag ${{ github.sha }}
kaal deploy --env prod --tag ${{ github.sha }}

# Éviter
kaal push  # utilise latest par défaut
```

**Rollback automatique** — configure toujours un step de rollback sur `if: failure()`.

**Staging avant prod** — ne déploie jamais directement en prod sans passer par staging.

**`kaal status` après deploy** — vérifie que tous les services sont `healthy` avant de continuer le pipeline.

```bash
kaal deploy --env prod --tag $SHA
kaal status --env prod --json | jq '.services[] | select(.status != "running")'
```
