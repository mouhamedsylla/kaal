# Workflow : Déploiement sur VPS

## Vue d'ensemble

```
Dev local                       VPS (SSH)
─────────────                   ──────────────────────────────
kaal preflight  →               validation de tous les prérequis
kaal push       →               [image sur le registry]
kaal deploy     →               SSH connect
                                ~/kaal/ ← compose + env + config files
                                docker compose pull
                                docker compose up -d
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

Le user deploy n'a pas besoin d'être dans le groupe docker dès le départ : `kaal setup` s'en occupe.

### Dans kaal.yaml

```yaml
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_kaal

environments:
  prod:
    target: vps-prod
    env_file: .env.prod
```

### Clé SSH dédiée (recommandé)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_kaal -C "kaal deploy key"
ssh-copy-id -i ~/.ssh/id_kaal.pub deploy@1.2.3.4
```

---

## Étape 0 : Premier setup

Si le user deploy n'est pas encore dans le groupe docker :

```bash
kaal setup --env prod
# → SSH connect
# → sudo usermod -aG docker deploy
# → Verified: deploy is in docker group
```

---

## Étape 1 : Preflight

Avant chaque déploiement, `kaal preflight` vérifie tout et retourne un plan d'action :

```bash
kaal preflight --target deploy
# (auto-détecte l'env prod si l'env actif est dev)
```

```
✓ kaal_yaml            project: my-api
✓ registry_image       ghcr.io/mouhamedsylla/my-api
✓ dockerfile           Dockerfile
✓ docker_daemon        reachable
✓ registry_creds       GITHUB_ACTOR=mouhamedsylla ✓
✓ compose_file         docker-compose.prod.yml
✓ target_host          1.2.3.4 (vps-prod)
✓ ssh_key              ~/.ssh/id_kaal
✓ vps_connectivity     connected to deploy@1.2.3.4
✓ vps_docker_group     deploy can run docker commands
✓ vps_env_file         .env.prod synced at ~/kaal/.env.prod
✓ All checks passed : ready to deploy
```

Si une vérification échoue, le rapport indique :
- `[HUMAN]` : ce que tu dois faire toi-même (crédentials, clé SSH, port firewall)
- `[AGENT]` : ce que ton agent AI peut fixer automatiquement (`kaal_setup`, `kaal_sync`…)

---

## Étape 2 : Push de l'image

```bash
kaal push
# ou avec un tag explicite :
kaal push --tag v1.0.0
```

**Ce que kaal fait automatiquement :**

- Détecte macOS ARM64 → build `linux/amd64` pour la compatibilité VPS
- Pour les stacks Node/Vite : scanne `.env.prod` et injecte tous les `VITE_*` / `NEXT_PUBLIC_*` / `REACT_APP_*` en `--build-arg` pour qu'ils soient baked dans le bundle
- Si le Dockerfile manque les `ARG` correspondants → patch transparent dans un fichier temporaire (l'original n'est pas modifié)

```
→ Detected macOS ARM64 : building for linux/amd64 (VPS target)
→ Injecting build args: VITE_APP_ENV, VITE_API_URL
  ARG/ENV lines auto-injected into builder stage (original Dockerfile unchanged)
→ Building ghcr.io/mouhamedsylla/my-api:abc1234 [linux/amd64]
→ Pushing ghcr.io/mouhamedsylla/my-api:abc1234
✓ Pushed ghcr.io/mouhamedsylla/my-api:abc1234
```

---

## Étape 3 : Déploiement

```bash
kaal deploy --env prod
# ou avec un tag précis :
kaal deploy --env prod --tag v1.0.0
```

**Ce que kaal fait automatiquement :**

1. Résout le target (`vps-prod`) depuis `kaal.yaml`
2. Ouvre une connexion SSH
3. **Sync automatique** : copie vers `~/kaal/` sur le VPS :
   - `kaal.yaml`
   - `docker-compose.prod.yml`
   - `.env.prod` (déclaré dans `environments.prod.env_file`)
   - Tous les fichiers référencés en bind-mount dans le compose (ex: `./nginx/prod.conf`)
4. `docker pull <image>:<tag>` sur le VPS
5. `IMAGE_TAG=<tag> docker compose -f ~/kaal/docker-compose.prod.yml up -d --remove-orphans`
6. Sauvegarde le tag dans `~/.kaal/<project>/current-tag` pour permettre un rollback

```
→ Deploying prod to vps-prod (vps:1.2.3.4)
→ Syncing files to remote
→ Pulling image and restarting services (tag: abc1234)
✓ Deployed my-api:abc1234 → vps-prod (1.2.3.4)
```

### Le répertoire de travail remote

Tous les fichiers vivent dans `~/kaal/` sur le VPS : jamais dans le home directory racine. docker compose est toujours lancé avec le chemin complet `~/kaal/docker-compose.<env>.yml`.

---

## Sync manuel

Pour pousser les fichiers de config sans redéployer :

```bash
kaal sync --env prod
# ✓ kaal.yaml, compose files, env files and bind-mount config files copied to ~/kaal/
```

Utile après avoir modifié `nginx/prod.conf` ou `.env.prod` sans changer l'image.

---

## Sync sans redéploiement

Quand seuls des **fichiers de configuration** changent (ex : `nginx/prod.conf`, `.env.prod`), utiliser `kaal sync` plutôt que de relancer un push + deploy complet.

### Ce que `kaal sync` fait

`kaal sync --env prod` copie vers `~/kaal/` sur le VPS :
- Les fichiers plats déclarés dans `kaal.yaml` : fichier compose, fichier env
- Tous les fichiers référencés en **bind-mount** dans le compose (ex : `./nginx/prod.conf` → `~/kaal/nginx/prod.conf`)

### Rechargement automatique de nginx

Si des fichiers de configuration nginx ont été mis à jour, kaal exécute automatiquement `nginx -s reload` à l'intérieur du container proxy : **sans interruption de service**. Aucune commande manuelle n'est nécessaire.

```
→ Syncing files to remote
  ✓ kaal.yaml
  ✓ docker-compose.prod.yml
  ✓ .env.prod
  ✓ nginx/prod.conf
→ Nginx config updated : reloading proxy container
✓ nginx -s reload (zero downtime)
```

### Quand utiliser quoi

Utiliser `kaal push` + `kaal deploy` uniquement quand le **code source ou le Dockerfile** change.

| Changement | Commande |
|---|---|
| Code source (`App.jsx`, `main.go`, etc.) | `kaal push --env prod --tag vX` + `kaal deploy --env prod --tag vX` |
| Config nginx (`nginx/prod.conf`) | `kaal sync --env prod` (+ reload auto) |
| Variables d'env (`.env.prod`) | `kaal sync --env prod` + `kaal deploy --env prod` |
| `docker-compose.prod.yml` | `kaal sync --env prod` + `kaal deploy --env prod` |

---

## Vérifier le déploiement

```bash
kaal status --env prod
```

```
SERVICE   STATUS    HEALTH
app       running   healthy
proxy     running   healthy
db        running   healthy
```

```bash
kaal logs app --env prod --follow
kaal logs app --env prod --since 1h
```

---

## Rollback

```bash
kaal rollback --env prod
# → revient automatiquement au tag précédent

kaal rollback --env prod --version v0.9.5
# → version explicite
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
      - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf:ro  # ← kaal sync copie ce fichier
    depends_on:
      app:
        condition: service_healthy

  app:
    image: ghcr.io/mouhamedsylla/my-api:${IMAGE_TAG}
    expose:
      - "8080"
    env_file:
      - .env.prod    # ← kaal sync copie ce fichier
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      retries: 3
    restart: unless-stopped
```

kaal sync détecte `./nginx/prod.conf` dans les volumes, le copie à `~/kaal/nginx/prod.conf`. docker compose le trouve exactement là où il l'attend.

---

## Checklist avant le premier déploiement

```bash
# 1. kaal.yaml configuré avec targets et environments.prod
kaal preflight --target deploy

# 2. Si docker group manquant
kaal setup --env prod

# 3. Push test
kaal push --tag test-$(date +%s)

# 4. Premier deploy
kaal deploy --env prod

# 5. Vérification
kaal status --env prod
kaal logs app --env prod
```
