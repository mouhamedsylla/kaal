# Workflow : Déploiement sur VPS

## Vue d'ensemble

```
Dev local                     VPS (SSH)
─────────────                 ──────────────────────────
kaal push --tag v1.0.0   →   [image sur le registry]
kaal deploy --env prod   →   SSH connect
                              docker compose pull
                              docker compose up -d
                              health check
                         ◄─   ✓ deployed
```

---

## Prérequis

### Sur ton VPS

```bash
# Docker + Compose plugin installés
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker deploy

# Utilisateur deploy avec accès SSH
useradd -m -s /bin/bash deploy
mkdir -p /home/deploy/.ssh
# Colle ta clé publique dans /home/deploy/.ssh/authorized_keys
```

### Dans kaal.yaml

```yaml
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4         # IP ou hostname de ton VPS
    user: deploy          # Utilisateur SSH
    key: ~/.ssh/id_kaal   # Clé privée SSH locale
    port: 22              # Port SSH (défaut: 22)

environments:
  prod:
    target: vps-prod
    runtime: compose
```

### Générer une clé SSH dédiée à kaal (recommandé)

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_kaal -C "kaal deploy key"
# Copie la clé publique sur le VPS
ssh-copy-id -i ~/.ssh/id_kaal.pub deploy@1.2.3.4
```

---

## Étape 1 : Push de l'image

```bash
kaal push --tag v1.0.0
```

**Ce qui se passe :**

1. Lit le registry configuré dans `kaal.yaml` (`registry.provider`)
2. Authentification avec les variables d'env (`GITHUB_TOKEN` pour GHCR, etc.)
3. `docker build -t ghcr.io/user/my-app:v1.0.0 .`
4. `docker push ghcr.io/user/my-app:v1.0.0`

Si `--tag` est omis, kaal utilise le SHA Git court du HEAD.

---

## Étape 2 : Déploiement

```bash
kaal deploy --env prod --tag v1.0.0
```

**Ce qui se passe en détail :**

1. Lit `kaal.yaml` — résout le target (`vps-prod`)
2. Ouvre une connexion SSH vers `deploy@1.2.3.4:22` avec `~/.ssh/id_kaal`
3. Copie `docker-compose.prod.yml` sur le VPS dans `~/my-app/`
4. Copie `.env.prod` si configuré et présent localement
5. Exécute sur le VPS :
   ```bash
   cd ~/my-app
   docker compose -f docker-compose.prod.yml pull
   docker compose -f docker-compose.prod.yml up -d
   ```
6. Vérifie le statut des conteneurs

```
✓ Deployed to vps-prod (1.2.3.4)
  api    running (healthy)
  db     running (healthy)
  cache  running (healthy)
```

---

## Vérifier le déploiement

```bash
kaal status --env prod
```

```
Environment: prod
Target:      vps-prod (1.2.3.4)

SERVICE   STATUS    HEALTH     UPTIME
api       running   healthy    2m 14s
db        running   healthy    2m 14s
cache     running   healthy    2m 14s
```

---

## Rollback

Si quelque chose se passe mal :

```bash
kaal rollback --env prod
```

kaal redémarre le conteneur avec le tag précédent (stocké dans le state du VPS).

Pour rollback vers une version spécifique :

```bash
kaal rollback --env prod --version v0.9.5
```

---

## Logs depuis le VPS

```bash
kaal logs api --env prod
kaal logs api --env prod --follow
kaal logs api --env prod --since 1h
```

kaal se connecte en SSH et exécute `docker compose logs` sur le VPS.

---

## Multi-VPS

Pour des architectures avec plusieurs serveurs :

```yaml
targets:
  app-server-1:
    type: vps
    host: 10.0.0.1
    user: deploy
    key: ~/.ssh/id_kaal
  app-server-2:
    type: vps
    host: 10.0.0.2
    user: deploy
    key: ~/.ssh/id_kaal
  db-server:
    type: vps
    host: 10.0.0.3
    user: deploy
    key: ~/.ssh/id_kaal
```

Déploiement cible par cible :

```bash
kaal deploy --env prod --target app-server-1 --tag v1.0.0
kaal deploy --env prod --target app-server-2 --tag v1.0.0
```

---

## Simulation locale de la prod

La puissance de kaal vient de la symétrie local ↔ prod.

```yaml
environments:
  dev:
    runtime: compose
    resources:
      cpus: "1"
      memory: 1G

  prod:
    target: vps-prod
    runtime: compose
    resources:
      cpus: "1"         # Même contrainte qu'en dev
      memory: 1G        # Si ça marche en local, ça marchera en prod
```

```bash
# Local — identique à la prod
kaal env use dev
kaal up

# Prod
kaal env use prod
kaal deploy --tag v1.0.0
```

Même configuration, même compose file structure, mêmes ressources allouées. Si ton app démarre et fonctionne dans 1G de RAM en local, elle fonctionnera avec les mêmes limites en prod.

---

## Checklist avant le premier déploiement

- [ ] `kaal.yaml` contient la section `targets` et `environments.prod.target`
- [ ] VPS accessible en SSH : `ssh deploy@1.2.3.4`
- [ ] Docker installé sur le VPS : `ssh deploy@1.2.3.4 docker --version`
- [ ] `docker-compose.prod.yml` existe dans le projet
- [ ] Variables d'env registry configurées (`GITHUB_TOKEN` ou `DOCKER_*`)
- [ ] Test du push : `kaal push --tag test-$(date +%s)`
- [ ] Premier deploy : `kaal deploy --env prod --tag <tag-du-push>`
- [ ] Vérification : `kaal status --env prod`
