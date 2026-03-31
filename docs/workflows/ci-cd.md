# Workflow : CI/CD

## Principe

pilot ne remplace pas GitHub Actions, GitLab CI, ou CircleCI. Il fournit les **primitives** que le CI orchestre.

```
CI Runner
  │
  ├─► pilot push --tag $SHA --env prod   # Build + push l'image (avec vars compile-time)
  ├─► pilot deploy --env staging         # Déploie sur staging
  │   [tests d'intégration...]
  └─► pilot deploy --env prod            # Déploie en prod
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

      - name: Install pilot
        run: |
          curl -sSL https://github.com/mouhamedsylla/pilot/releases/latest/download/pilot-linux-amd64 -o pilot
          chmod +x pilot
          sudo mv pilot /usr/local/bin/

      - name: Push image
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GITHUB_ACTOR: ${{ github.actor }}
        run: pilot push --tag ${{ github.sha }} --env prod
        # --env prod : lit .env.prod pour injecter les VITE_* en build-arg

      - name: Deploy to staging
        env:
          PILOT_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: pilot deploy --env staging --tag ${{ github.sha }}

      - name: Integration tests
        run: |
          curl -f https://staging.my-app.com/health

      - name: Deploy to prod
        if: success()
        env:
          PILOT_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: pilot deploy --env prod --tag ${{ github.sha }}

      - name: Rollback on failure
        if: failure()
        env:
          PILOT_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: pilot rollback --env prod
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
        run: pilot push --tag pr-${{ github.event.pull_request.number }} --env staging

      - name: Deploy to staging
        env:
          PILOT_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        run: pilot deploy --env staging --tag pr-${{ github.event.pull_request.number }}
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
    - pilot push --tag $TAG --env prod
  only:
    - main

deploy-staging:
  stage: deploy-staging
  script:
    - pilot deploy --env staging --tag $TAG
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
    - pilot deploy --env prod --tag $TAG
  environment:
    name: production
  when: manual
  only:
    - main
```

---

## Variables d'environnement en CI

pilot lit les credentials depuis les variables d'environnement standard.

### Pour `pilot push`

| Variable | Description |
|----------|-------------|
| `GITHUB_TOKEN` | Token GitHub (auto-injecté par GitHub Actions) |
| `GITHUB_ACTOR` | Username GitHub (auto-injecté par GitHub Actions) |
| `DOCKER_USERNAME` | Username Docker Hub |
| `DOCKER_PASSWORD` | Password Docker Hub |
| `REGISTRY_USERNAME` | Username registry custom |
| `REGISTRY_PASSWORD` | Password registry custom |

### Pour `pilot deploy` (VPS)

| Variable | Description |
|----------|-------------|
| `PILOT_SSH_KEY` | Contenu de la clé SSH privée (PEM) |

pilot écrit la clé dans un fichier temporaire, l'utilise pour SSH, puis la supprime.

### Pour les vars compile-time (Vite, Next.js, CRA)

Si ton projet est stack `node` avec des variables `VITE_*` ou `NEXT_PUBLIC_*`, configure-les dans les secrets CI et expose-les dans un `.env.prod` ou directement comme variables d'environnement CI :

```yaml
- name: Push image
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    GITHUB_ACTOR: ${{ github.actor }}
    VITE_API_URL: ${{ secrets.VITE_API_URL }}
    VITE_APP_ENV: production
  run: pilot push --tag ${{ github.sha }} --env prod
```

pilot détecte automatiquement les `VITE_*` dans l'env file et les injecte en `--build-arg`.

---

## Ce que le CI fait vs ce que pilot fait

| Responsabilité | CI Runner | pilot |
|----------------|-----------|------|
| Déclencher sur un push | ✓ | |
| Cloner le dépôt | ✓ | |
| Lancer les tests unitaires | ✓ | |
| Construire l'image Docker | | ✓ (`pilot push`) |
| Injecter les vars compile-time | | ✓ (auto-détection `VITE_*`) |
| Pousser vers le registry | | ✓ (`pilot push`) |
| Synchroniser les fichiers de config | | ✓ (`pilot deploy` implicite) |
| Déployer sur le serveur | | ✓ (`pilot deploy`) |
| Rollback si échec | | ✓ (`pilot rollback`) |
| Vérifier la santé post-deploy | (peut déléguer à pilot) | ✓ (`pilot status`) |
| Notifications Slack/email | ✓ | |
| Gestion des branches/PRs | ✓ | |

---

## Stratégies de déploiement

### Rolling update (défaut)

```bash
pilot deploy --env prod --tag v1.2.0
```

docker compose arrête l'ancien conteneur et démarre le nouveau. Brève indisponibilité (~5s).

### Blue-green (roadmap)

```bash
pilot deploy --env prod --tag v1.2.0 --strategy blue-green
```

### Canary (roadmap)

```bash
pilot deploy --env prod --tag v1.2.0 --strategy canary --weight 10
```

---

## Bonnes pratiques

**Tags immuables** : utilise toujours `--tag $SHA` (SHA Git), jamais `latest`. `latest` est mutable et rend le rollback ambigu.

```bash
# Bien
pilot push --tag ${{ github.sha }} --env prod
pilot deploy --env prod --tag ${{ github.sha }}

# Éviter
pilot push  # tag par défaut = SHA courant, ok mais moins explicite en CI
```

**`--env` sur pilot push** : toujours préciser l'env en CI pour que pilot lise le bon fichier `.env.<env>` et injecte les bonnes variables compile-time.

**Rollback automatique** : configure toujours un step de rollback sur `if: failure()`.

**Staging avant prod** : ne déploie jamais directement en prod sans passer par staging.

**`pilot status` après deploy** : vérifie que tous les services sont `healthy` avant de continuer le pipeline.

```bash
pilot deploy --env prod --tag $SHA
pilot status --env prod --json | jq '.services[] | select(.status != "running")'
```
