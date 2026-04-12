# Workflow : Déploiement sur VPS

## Vue d'ensemble

```
Dev local                       VPS (SSH)
─────────────                   ──────────────────────────────
pilot preflight  →               validation + génère pilot.lock
pilot push       →               [image sur le registry]
pilot plan       →               affiche le plan (rien exécuté)
pilot deploy     →               [1] lock check
                                 [2] résolution secrets
                                 [3] sync compose + config
                                 [4] pre_hooks (si déclarés)
                                 [5] migrations (si détectées)
                                 [6] docker pull + compose up
                                 [7] post_hooks (si déclarés)
                                 [8] healthcheck
                ◄─              ✓ deployed
```

---

## Prérequis

### Sur ton VPS

```bash
# Docker + Compose plugin
curl -fsSL https://get.docker.com | sh

# Utilisateur deploy avec accès SSH
useradd -m -s /bin/bash deploy
mkdir -p /home/deploy/.ssh
cat your-key.pub >> /home/deploy/.ssh/authorized_keys
```

Le user deploy n'a pas besoin d'être dans le groupe docker dès le départ : `pilot setup` s'en occupe.

### Dans pilot.yaml

```yaml
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot

environments:
  prod:
    target: vps-prod
    env_file: .env.prod
    hooks:
      pre_deploy:
        - command: "echo 'starting deploy'"
          description: "notification"
      post_deploy:
        - command: "curl -X POST $WEBHOOK_URL"
          description: "notify webhook"
    migrations:
      tool: prisma
      command: "npx prisma migrate deploy"
      rollback_command: "npx prisma migrate rollback"
      reversible: true
```

### Clé SSH dédiée (recommandé)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_pilot -C "pilot deploy key"
ssh-copy-id -i ~/.ssh/id_pilot.pub deploy@1.2.3.4
```

---

## Étape 0 : Premier setup

Si le user deploy n'est pas encore dans le groupe docker :

```bash
pilot setup --env prod
# → SSH connect
# → sudo usermod -aG docker deploy
# → Verified: deploy is in docker group
```

---

## Étape 1 : Preflight + génération de pilot.lock

Avant chaque déploiement, `pilot preflight` vérifie tout **et génère `pilot.lock`** :

```bash
pilot preflight --target deploy --env prod
```

```
✓ pilot_yaml            project: my-api
✓ registry_image       ghcr.io/mouhamedsylla/my-api
✓ dockerfile           Dockerfile
✓ docker_daemon        reachable
✓ registry_creds       GITHUB_ACTOR=mouhamedsylla ✓
✓ compose_file         docker-compose.prod.yml
✓ target_host          1.2.3.4 (vps-prod)
✓ ssh_key              ~/.ssh/id_pilot
✓ vps_connectivity     connected to deploy@1.2.3.4
✓ vps_docker_group     deploy can run docker commands
✓ vps_env_file         .env.prod synced at ~/pilot/.env.prod
✓ All checks passed : pilot.lock generated
```

**Commiter `pilot.lock` dans le dépôt :**

```bash
git add pilot.lock
git commit -m "chore: update pilot.lock"
```

`pilot.lock` est le contrat de déploiement validé. `pilot deploy` le vérifie au premier pas : si les sources ont changé depuis sa génération, le déploiement est refusé.

---

## Étape 2 : Push de l'image

```bash
pilot push --env prod
# ou avec un tag explicite :
pilot push --tag v1.0.0 --env prod
```

**Ce que pilot fait automatiquement :**

- Détecte macOS ARM64 → build `linux/amd64` pour la compatibilité VPS
- Pour les stacks Node/Vite : scanne `.env.prod` et injecte tous les `VITE_*` / `NEXT_PUBLIC_*` / `REACT_APP_*` en `--build-arg`
- Si le Dockerfile manque les `ARG` correspondants → patch transparent dans un fichier temporaire

```
→ Detected macOS ARM64 : building for linux/amd64 (VPS target)
→ Injecting build args: VITE_APP_ENV, VITE_API_URL
→ Building ghcr.io/mouhamedsylla/my-api:abc1234 [linux/amd64]
→ Pushing ghcr.io/mouhamedsylla/my-api:abc1234
✓ Pushed ghcr.io/mouhamedsylla/my-api:abc1234
```

---

## Étape 3 : Voir le plan (optionnel)

```bash
pilot plan --env prod
```

```
  Execution plan : pilot deploy --env prod

  Steps
  ──────────────────────────────────────────────────
  [1] preflight        verify config, secrets, SSH reachability
  [2] migrations       run prisma migrations (reversible)  (compensable)
  [3] deploy           pull image + docker compose up      (compensable)
  [4] post_hooks       run post-deploy hooks on remote
  [5] healthcheck      wait for all services healthy

  Compensation plan  (LIFO : executed on failure)
  ──────────────────────────────────────────────────
  [1] deploy           restore previous image tag
  [2] migrations       npx prisma migrate rollback
```

Rien n'est exécuté. C'est la prévisualisation exacte de ce que `pilot deploy` fera.

---

## Étape 4 : Déploiement

```bash
pilot deploy --env prod
# ou avec un tag précis :
pilot deploy --env prod --tag v1.0.0
```

Le pipeline complet :

```
✓  [1] lock check     pilot.lock OK (hash: abc123)
✓  [2] secrets        3 refs resolved → .pilot/env.tmp
✓  [3] sync           4 files → ~/pilot/
   │  ✓ docker-compose.prod.yml
   │  ✓ .env.prod
   │  ✓ nginx/prod.conf
✓  [4] pre_hooks      echo 'starting deploy'
✓  [5] migrations     npx prisma migrate deploy
✓  [6] deploy         my-api:abc1234 pulled, services restarted
✓  [7] post_hooks     curl -X POST $WEBHOOK_URL
✓  [8] healthcheck    api healthy · db healthy · proxy healthy
✓  Deployed my-api:abc1234 → vps-prod (1.2.3.4)
```

### Compensation LIFO en cas d'échec

Si le déploiement échoue à partir de l'étape 4, pilot compense en LIFO automatiquement :

```
✗  [6] deploy         docker compose up failed

→  Compensating (LIFO):
   [1] Restoring previous image tag (v0.9.5)
   [2] Rolling back migrations (npx prisma migrate rollback)
✓  VPS restored to previous state
```

---

## Sync manuel

Pour pousser les fichiers de config sans redéployer :

```bash
pilot sync --env prod
```

Utile après avoir modifié `nginx/prod.conf` ou `.env.prod` sans changer l'image.

### Rechargement automatique de nginx

Si des fichiers de configuration nginx ont été mis à jour, pilot exécute automatiquement `nginx -s reload` dans le container : sans interruption de service.

```
→ Syncing files to remote
  ✓ docker-compose.prod.yml
  ✓ .env.prod
  ✓ nginx/prod.conf
→ Nginx config updated : reloading proxy container
✓ nginx -s reload (zero downtime)
```

### Quand utiliser quoi

| Changement | Commande |
|---|---|
| Code source | `pilot push --tag vX` + `pilot deploy --tag vX` |
| Config nginx | `pilot sync` (+ reload auto) |
| Variables d'env | `pilot sync` + `pilot deploy` |
| `docker-compose.prod.yml` | `pilot sync` + `pilot deploy` |

---

## Vérifier le déploiement

```bash
pilot status --env prod
```

```
SERVICE   STATUS    HEALTH
api       running   healthy
proxy     running   healthy
db        running   healthy
```

```bash
pilot logs api --env prod --follow
pilot logs api --env prod --since 1h
```

---

## Rollback

```bash
pilot rollback --env prod          # revient au tag précédent
pilot rollback --env prod --version v0.9.5   # version explicite
```

---

## Architecture typique en prod

```yaml
# docker-compose.prod.yml (généré par l'agent)
services:
  proxy:
    image: nginx:1.27-alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf:ro
    depends_on:
      app:
        condition: service_healthy

  app:
    image: ghcr.io/mouhamedsylla/my-api:${IMAGE_TAG}
    expose:
      - "8080"
    env_file:
      - .env.prod
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      retries: 3
    restart: unless-stopped
```

---

## Checklist avant le premier déploiement

```bash
# 1. Prérequis + génération de pilot.lock
pilot preflight --target deploy --env prod
git add pilot.lock && git commit -m "chore: update pilot.lock"

# 2. Si docker group manquant
pilot setup --env prod

# 3. Push
pilot push --tag v1.0.0 --env prod

# 4. Voir le plan
pilot plan --env prod

# 5. Premier deploy
pilot deploy --env prod --tag v1.0.0

# 6. Vérification
pilot status --env prod
pilot logs api --env prod
```
