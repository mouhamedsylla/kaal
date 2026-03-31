# kaal deploy

Déploie l'application sur une cible distante via SSH.

```
kaal deploy [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement à déployer (défaut : env actif) |
| `--tag`, `-t` | Tag de l'image à déployer (défaut : SHA Git court) |
| `--target` | Surcharge le target défini dans `kaal.yaml` |
| `--dry-run` | Affiche ce qui serait fait sans exécuter |
| `--no-rollback` | Désactive le rollback automatique en cas de service `unhealthy` : utile pour déboguer un déploiement en échec |

## Comportement (VPS)

1. Résout l'environnement et le target (`environments.<env>.target` dans `kaal.yaml`)
2. Connexion SSH au VPS (`targets.<name>.host`, `.user`, `.key`)
3. **Sync automatique** : copie vers `~/kaal/` sur le VPS :
   - `docker-compose.<env>.yml`
   - Fichier env déclaré dans `environments.<env>.env_file`
   - Tous les fichiers référencés en bind-mount dans le compose (ex: `./nginx/prod.conf`)
4. `docker pull <image>:<tag>` sur le VPS
5. `IMAGE_TAG=<tag> docker compose -f ~/kaal/docker-compose.<env>.yml up -d --remove-orphans`
6. Sauvegarde le tag dans `~/.kaal/<project>/current-tag` pour le rollback

```
→ Deploying prod to vps-prod (vps:1.2.3.4)
→ Syncing files to remote
→ Pulling image and restarting services (tag: abc1234)
✓ Deployed my-api:abc1234 → vps-prod (1.2.3.4)
```

## Le répertoire de travail distant

Tous les fichiers vivent dans `~/kaal/` sur le VPS. kaal exécute toujours docker compose avec le chemin complet :

```bash
docker compose -f ~/kaal/docker-compose.prod.yml up -d
```

Jamais de commandes dans le home directory racine. Jamais de chemins relatifs.

## Sync automatique des fichiers de config

`kaal deploy` inclut un `kaal sync` implicite avant chaque déploiement. Pour pousser des fichiers de config sans redéployer :

```bash
kaal sync --env prod
```

Pour les bind-mounts nginx, kaal lit le compose file, détecte les sources locales (ex: `./nginx/prod.conf`) et les copie sur le VPS en préservant la structure de répertoires :

```
local: ./nginx/prod.conf
remote: ~/kaal/nginx/prod.conf
```

Docker compose trouve le fichier exactement là où il l'attend.

## Exemples

```bash
# Déployer l'env actif avec le SHA Git courant
kaal deploy

# Déployer prod avec un tag explicite
kaal deploy --env prod --tag v1.2.0

# Voir ce qui serait fait sans déployer
kaal deploy --env prod --tag v1.2.0 --dry-run

# Déployer sur un target spécifique
kaal deploy --env prod --target vps-backup --tag v1.2.0
```

## Rollback automatique

`kaal deploy` vérifie automatiquement la santé des containers après chaque déploiement. Le processus est le suivant :

- Après le `docker compose up -d`, kaal interroge l'état des services toutes les **5 secondes** pendant au maximum **60 secondes**
- Si un service passe en état `unhealthy`, kaal déclenche automatiquement un rollback vers la version précédente (lue depuis `~/.kaal/<project>/prev-tag`)
- Le rollback utilise le même mécanisme que `kaal rollback` : aucun état supplémentaire n'est requis

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
kaal deploy --env prod --tag v1.2.0 --no-rollback
# → Le container reste en place même s'il est unhealthy
# → kaal status + kaal logs pour diagnostiquer
```

| Flag | Comportement |
|------|-------------|
| _(par défaut)_ | Rollback automatique si un service est `unhealthy` sous 60s |
| `--no-rollback` | Pas de rollback : le déploiement échoué reste en place pour débogage |

## Prérequis

- Le target doit être défini dans `kaal.yaml`
- L'environnement doit référencer ce target (`environments.prod.target: vps-prod`)
- La clé SSH doit être accessible (`targets.vps-prod.key: ~/.ssh/id_kaal`)
- Docker installé sur le VPS, user deploy dans le groupe docker (`kaal setup` si besoin)
- `docker-compose.<env>.yml` doit exister localement

## Workflow recommandé avant le premier déploiement

```bash
# Vérifier que tout est prêt
kaal preflight --target deploy --env prod

# Si le user deploy n'est pas dans le groupe docker
kaal setup --env prod

# Build + push
kaal push --env prod

# Déployer
kaal deploy --env prod
```

## Voir aussi

- [`kaal preflight`](../workflows/deploy-vps.md) : vérifier les prérequis avant de déployer
- [`kaal push`](push.md) : construire et pousser l'image avant le deploy
- [`kaal sync`](../workflows/deploy-vps.md#sync-manuel) : synchroniser les fichiers sans redéployer
- [`kaal rollback`](../workflows/deploy-vps.md#rollback) : revenir à la version précédente
- [Workflow VPS complet](../workflows/deploy-vps.md)
