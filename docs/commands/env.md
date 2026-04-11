# pilot env

Gère l'environnement actif du projet.

## Sous-commandes

| Sous-commande | Description |
|---|---|
| `pilot env use <env>` | Définit l'environnement actif |
| `pilot env current` | Affiche l'environnement actuellement actif |
| `pilot env diff <env1> <env2>` | Compare deux environnements et signale les divergences |

## Environnement actif

L'environnement actif est stocké dans `.pilot-current-env` à la racine du projet. Toutes les commandes pilot lisent ce fichier lorsque `--env` n'est pas passé explicitement.

### `.pilot-current-env` et `.gitignore`

Ce fichier **doit être ajouté à `.gitignore`**. Chaque développeur peut avoir un environnement actif différent sans affecter les autres.

```gitignore
.pilot-current-env
```

## `pilot env diff`

Compare les variables d'environnement, les ports et les services entre deux environnements.

```bash
pilot env diff dev prod
```

```
env diff  dev  ↔  prod

  Variables
  ──────────────────────────────────────────────────
  SENTRY_DSN                              only in prod
  DEBUG                                   only in dev
  API_URL                                 empty in dev

  Ports
  ──────────────────────────────────────────────────
  SERVICE               dev           prod
  api                   8080          80
  db                    5432          —

  Services
  ──────────────────────────────────────────────────
  mailhog                               only in dev compose file

  3 divergence(s) found
```

Utile avant un déploiement pour détecter les divergences "ça marche en dev, ça casse en prod" avant qu'elles ne surviennent.

## Exemples

```bash
# Passer en environnement de production
pilot env use prod

# Afficher l'environnement actif
pilot env current
# → prod

# Comparer dev et prod
pilot env diff dev prod

# Vérifier l'état de prod sans changer l'env actif
pilot status --env prod
```

## Surcharge avec `--env`

Le flag `--env` permet d'utiliser un environnement spécifique pour une seule commande, sans modifier `.pilot-current-env` :

```bash
# L'env actif est "dev"
pilot deploy --env prod   # déploie en prod pour cette commande uniquement
pilot env current         # → dev (inchangé)
```

## Environnements disponibles

Les environnements valides sont ceux déclarés dans `environments` dans `pilot.yaml` :

```yaml
environments:
  dev:
    runtime: compose
    env_file: .env.dev
  staging:
    target: vps-staging
    env_file: .env.staging
  prod:
    target: vps-prod
    env_file: .env.prod
```
