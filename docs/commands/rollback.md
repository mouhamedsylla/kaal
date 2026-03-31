# kaal rollback

Revient au déploiement précédent ou à une version spécifique.

```
kaal rollback [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--version`, `-v` | Tag précis à restaurer (ex: `v1.2.3`, `abc1234`) |
| `--env`, `-e` | Environnement cible (défaut : env actif) |

## Fonctionnement

### État stocké sur le VPS

kaal maintient un répertoire d'état par projet sur le VPS :

```
~/.kaal/<project-name>/
├── current-tag        # tag actuellement déployé
├── prev-tag           # tag du déploiement précédent
└── deployments.json   # historique des 10 derniers déploiements
```

### Sans `--version` (rollback rapide)

1. Connexion SSH au VPS
2. Lecture de `~/.kaal/<project-name>/prev-tag`
3. `docker pull <image>:<prev-tag>` sur le VPS
4. `IMAGE_TAG=<prev-tag> docker compose -f ~/kaal/docker-compose.<env>.yml up -d`
5. Mise à jour de `current-tag` ← `prev-tag`

### Avec `--version <tag>`

Identique, mais utilise le tag spécifié au lieu de lire `prev-tag`. Permet de restaurer n'importe quelle version listée dans `deployments.json`.

```
→ Rolling back prod to abc1234
→ Pulling image ghcr.io/mouhamedsylla/mon-projet:abc1234
→ Restarting services
✓ Rolled back to abc1234 (vps-prod)
```

## Limitation : un seul pas en arrière

`prev-tag` ne stocke que le tag immédiatement précédent : pas un historique complet.

| Déploiements | current-tag | prev-tag |
|---|---|---|
| Après deploy v1 | v1 | : |
| Après deploy v2 | v2 | v1 |
| Après deploy v3 | v3 | v2 |
| Après `kaal rollback` | v2 | v2 |
| Après un 2e `kaal rollback` | v2 | v2 *(no-op)* |

Un second rollback consécutif est un **no-op** : `prev-tag` et `current-tag` pointent tous deux vers v2.

Pour revenir plus loin : utiliser `--version` avec un tag issu de `kaal history`.

```bash
# Voir l'historique des déploiements
kaal history --env prod

# Revenir à une version spécifique
kaal rollback --version v1.0.0 --env prod
```

## Rollback automatique

`kaal deploy` déclenche un rollback automatique si le healthcheck post-déploiement échoue :

```
→ Deploying prod (tag: def5678)
→ Running healthcheck...
✗ Healthcheck failed after 3 attempts
→ Auto-rolling back to abc1234
✓ Rolled back to abc1234
```

Pour désactiver ce comportement : passer `--no-rollback` à `kaal deploy`.

## Erreurs courantes

**"no previous deployment found"**
Le VPS n'a pas de fichier `prev-tag` : c'est le premier déploiement, ou l'état a été perdu.
Correction : utiliser `--version <tag>` pour spécifier explicitement le tag cible.

**"image not found"**
Le tag demandé n'existe plus dans le registry.
Correction : vérifier que l'image n'a pas été supprimée du registry, ou choisir un autre tag via `kaal history`.

## Exemple de workflow

```bash
# 1. Déploiement de v3 (qui s'avère défectueux)
kaal push
kaal deploy --env prod

# 2. Constatation du problème
kaal status --env prod
kaal logs app --env prod

# 3. Rollback immédiat vers v2
kaal rollback --env prod
# → Retour à l'état stable en quelques secondes

# 4. Vérification
kaal status --env prod
```
