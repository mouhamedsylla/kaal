# pilot deploy

Déploie l'application sur une cible distante via SSH.

```
pilot deploy [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement à déployer (défaut : env actif) |
| `--tag`, `-t` | Tag de l'image à déployer (défaut : SHA Git court) |
| `--target` | Surcharge le target défini dans `pilot.yaml` |
| `--dry-run` | Affiche le plan complet sans exécuter |
| `--no-rollback` | Désactive le rollback automatique en cas de service `unhealthy` |

## Pipeline de déploiement (8 étapes)

`pilot deploy` exécute un pipeline structuré. Chaque étape ne s'active que si elle est pertinente pour le projet (déclaré dans `pilot.lock`).

```
[1] lock check      valide que pilot.lock n'est pas périmé
[2] secrets         résout les refs → fichier env temporaire
[3] sync            pousse compose + fichiers de config vers le remote
[4] pre_hooks       exécute les commandes pre-deploy via SSH       (si déclarés)
[5] migrations      applique les changements de schéma             (si détectés)
[6] deploy          docker pull + docker compose up
[7] post_hooks      exécute les commandes post-deploy via SSH      (si déclarés)
[8] healthcheck     attend que tous les services soient healthy
```

```
→ [pilot deploy --env prod]
✓  lock check     pilot.lock OK (hash: abc123)
✓  secrets        3 refs resolved → .pilot/env.tmp
✓  sync           4 files → ~/pilot/
✓  pre_hooks      echo 'starting deploy'
✓  migrations     npx prisma migrate deploy (reversible)
✓  deploy         my-api:a1b2c3d pulled, services restarted
✓  post_hooks     curl -X POST $WEBHOOK_URL
✓  healthcheck    api healthy · db healthy · proxy healthy
```

## Prérequis : pilot.lock

`pilot deploy` refuse de continuer si `pilot.lock` est périmé ou absent.

```bash
# Générer ou régénérer pilot.lock
pilot preflight --target deploy --env prod

# Puis déployer
pilot deploy --env prod
```

`pilot.lock` doit être **commité dans le repo**. Si un des fichiers sources (pilot.yaml, compose, schéma) change, le lock devient périmé et pilot arrête avec une instruction claire.

## LIFO compensation

En cas d'échec à partir de l'étape 4, pilot exécute automatiquement les compensations en ordre inverse :

```
échec à l'étape [6] deploy
  → compensation : restaure le tag d'image précédent (toujours)
  → compensation : rollback migration (si reversible: true + rollback_command défini)

échec à l'étape [5] migrations
  → compensation : rollback migration (si reversible: true)
  (deploy n'a pas encore démarré, pas d'image à restaurer)
```

## TypeC : choix requis

Certaines erreurs suspendent le déploiement et présentent des options :

```
✗  user "deploy" n'est pas dans le groupe docker sur 1.2.3.4

   Actions possibles :
   → [0] pilot setup --env prod   (automatique)
     [1] ssh deploy@1.2.3.4 'sudo usermod -aG docker deploy'   (manuel)

   Après avoir pris une action : pilot resume
```

`pilot resume --answer 0` reprend depuis le début sans relancer le déploiement complet depuis zéro.

## Répertoire de travail distant

Tous les fichiers vivent dans `~/pilot/` sur le VPS. Docker compose est toujours appelé avec un chemin complet :

```bash
docker compose -f ~/pilot/docker-compose.prod.yml up -d --remove-orphans
```

## Sync automatique

`pilot deploy` inclut un `pilot sync` implicite (étape 3) avant chaque déploiement. Pour synchroniser les fichiers de config sans redéployer :

```bash
pilot sync --env prod
```

Pour les bind-mounts nginx, pilot lit le compose file, détecte les sources locales (ex: `./nginx/prod.conf`) et les copie sur le VPS en préservant la structure de répertoires.

## Exemples

```bash
# Déployer l'env actif
pilot deploy

# Déployer prod avec un tag explicite
pilot deploy --env prod --tag v1.2.0

# Afficher le plan sans exécuter
pilot deploy --env prod --dry-run

# Voir d'abord le plan structuré
pilot plan --env prod

# Déployer sur un target spécifique
pilot deploy --env prod --target vps-backup --tag v1.2.0
```

## Rollback automatique

Après l'étape [6], pilot surveille la santé des containers :

- Interroge toutes les **5 secondes** pendant **60 secondes**
- Si un service est `unhealthy`, déclenche automatiquement un rollback vers le tag précédent

```
→ Waiting for services to be healthy...
✗ Service "api" is unhealthy : triggering automatic rollback
→ Rolling back to v1.1.0
✓ Rollback complete : vps-prod is running v1.1.0
```

`--no-rollback` désactive ce comportement pour permettre d'inspecter les logs du container défaillant.

## Prérequis

- `pilot preflight --target deploy` doit avoir été exécuté et `pilot.lock` commité
- Le target doit être défini dans `pilot.yaml`
- L'environnement doit référencer ce target (`environments.prod.target: vps-prod`)
- La clé SSH doit être accessible (`targets.vps-prod.key: ~/.ssh/id_pilot`)
- Docker installé sur le VPS, user deploy dans le groupe docker (`pilot setup` si besoin)
- `docker-compose.<env>.yml` doit exister localement

## Workflow recommandé avant le premier déploiement

```bash
# 1. Vérifier les prérequis + générer pilot.lock
pilot preflight --target deploy --env prod

# 2. Si le user n'est pas dans le groupe docker
pilot setup --env prod

# 3. Build + push
pilot push --env prod

# 4. Voir le plan
pilot plan --env prod

# 5. Déployer
pilot deploy --env prod
```

## Voir aussi

- [`pilot preflight`](preflight.md) : vérifier les prérequis et générer pilot.lock
- [`pilot plan`](../architecture.md) : afficher le plan d'exécution sans déployer
- [`pilot push`](push.md) : construire et pousser l'image avant le deploy
- [`pilot sync`](sync.md) : synchroniser les fichiers sans redéployer
- [`pilot rollback`](rollback.md) : revenir à la version précédente
- [`pilot resume`](../commands/resume.md) : reprendre après une suspension TypeC
- [Workflow VPS complet](../workflows/deploy-vps.md)
