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
| `--dry-run` | Affiche ce qui serait fait sans exécuter |
| `--no-rollback` | Désactive le rollback automatique en cas de service `unhealthy` : utile pour déboguer un déploiement en échec |

## Comportement (VPS)

1. Résout l'environnement et le target (`environments.<env>.target` dans `pilot.yaml`)
2. Connexion SSH au VPS (`targets.<name>.host`, `.user`, `.key`)
3. **Sync automatique** : copie vers `~/pilot/` sur le VPS :
   - `docker-compose.<env>.yml`
   - Fichier env déclaré dans `environments.<env>.env_file`
   - Tous les fichiers référencés en bind-mount dans le compose (ex: `./nginx/prod.conf`)
4. `docker pull <image>:<tag>` sur le VPS
5. `IMAGE_TAG=<tag> docker compose -f ~/pilot/docker-compose.<env>.yml up -d --remove-orphans`
6. Sauvegarde le tag dans `~/.pilot/<project>/current-tag` pour le rollback

```
→ Deploying prod to vps-prod (vps:1.2.3.4)
→ Syncing files to remote
→ Pulling image and restarting services (tag: abc1234)
✓ Deployed my-api:abc1234 → vps-prod (1.2.3.4)
```

## Le répertoire de travail distant

Tous les fichiers vivent dans `~/pilot/` sur le VPS. pilot exécute toujours docker compose avec le chemin complet :

```bash
docker compose -f ~/pilot/docker-compose.prod.yml up -d
```

Jamais de commandes dans le home directory racine. Jamais de chemins relatifs.

## Sync automatique des fichiers de config

`pilot deploy` inclut un `pilot sync` implicite avant chaque déploiement. Pour pousser des fichiers de config sans redéployer :

```bash
pilot sync --env prod
```

Pour les bind-mounts nginx, pilot lit le compose file, détecte les sources locales (ex: `./nginx/prod.conf`) et les copie sur le VPS en préservant la structure de répertoires :

```
local: ./nginx/prod.conf
remote: ~/pilot/nginx/prod.conf
```

Docker compose trouve le fichier exactement là où il l'attend.

## Exemples

```bash
# Déployer l'env actif avec le SHA Git courant
pilot deploy

# Déployer prod avec un tag explicite
pilot deploy --env prod --tag v1.2.0

# Voir ce qui serait fait sans déployer
pilot deploy --env prod --tag v1.2.0 --dry-run

# Déployer sur un target spécifique
pilot deploy --env prod --target vps-backup --tag v1.2.0
```

## Rollback automatique

`pilot deploy` vérifie automatiquement la santé des containers après chaque déploiement. Le processus est le suivant :

- Après le `docker compose up -d`, pilot interroge l'état des services toutes les **5 secondes** pendant au maximum **60 secondes**
- Si un service passe en état `unhealthy`, pilot déclenche automatiquement un rollback vers la version précédente (lue depuis `~/.pilot/<project>/prev-tag`)
- Le rollback utilise le même mécanisme que `pilot rollback` : aucun état supplémentaire n'est requis

```
→ Deploying prod to vps-prod (vps:1.2.3.4)
→ Pulling image and restarting services (tag: abc1234)
→ Waiting for services to be healthy...
✗ Service "app" is unhealthy : triggering automatic rollback
→ Rolling back to v1.1.0
✓ Rollback complete : vps-prod is running v1.1.0
```

Pour **désactiver le rollback automatique** : par exemple pour inspecter les logs du container défaillant : utiliser le flag `--no-rollback` :

```bash
pilot deploy --env prod --tag v1.2.0 --no-rollback
# → Le container reste en place même s'il est unhealthy
# → pilot status + pilot logs pour diagnostiquer
```

| Flag | Comportement |
|------|-------------|
| _(par défaut)_ | Rollback automatique si un service est `unhealthy` sous 60s |
| `--no-rollback` | Pas de rollback : le déploiement échoué reste en place pour débogage |

## Prérequis

- Le target doit être défini dans `pilot.yaml`
- L'environnement doit référencer ce target (`environments.prod.target: vps-prod`)
- La clé SSH doit être accessible (`targets.vps-prod.key: ~/.ssh/id_pilot`)
- Docker installé sur le VPS, user deploy dans le groupe docker (`pilot setup` si besoin)
- `docker-compose.<env>.yml` doit exister localement

## Workflow recommandé avant le premier déploiement

```bash
# Vérifier que tout est prêt
pilot preflight --target deploy --env prod

# Si le user deploy n'est pas dans le groupe docker
pilot setup --env prod

# Build + push
pilot push --env prod

# Déployer
pilot deploy --env prod
```

## Voir aussi

- [`pilot preflight`](../workflows/deploy-vps.md) : vérifier les prérequis avant de déployer
- [`pilot push`](push.md) : construire et pousser l'image avant le deploy
- [`pilot sync`](../workflows/deploy-vps.md#sync-manuel) : synchroniser les fichiers sans redéployer
- [`pilot rollback`](../workflows/deploy-vps.md#rollback) : revenir à la version précédente
- [Workflow VPS complet](../workflows/deploy-vps.md)
