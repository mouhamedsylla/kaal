# pilot env

Gère l'environnement actif du projet.

## Sous-commandes

| Sous-commande | Description |
|---|---|
| `pilot env use <env>` | Définit l'environnement actif |
| `pilot env current` | Affiche l'environnement actuellement actif |

## Environnement actif

L'environnement actif est stocké dans le fichier `.pilot-current-env` à la racine du projet. Toutes les commandes pilot lisent ce fichier pour déterminer l'environnement à utiliser lorsque le flag `--env` n'est pas passé explicitement.

```
# Contenu de .pilot-current-env
prod
```

### `.pilot-current-env` et `.gitignore`

Ce fichier **doit être ajouté à `.gitignore`**. Chaque développeur de l'équipe peut avoir un environnement actif différent (l'un en `dev`, un autre en `staging`) sans que cela n'affecte les autres.

```gitignore
# .gitignore
.pilot-current-env
```

## Exemples

```bash
# Passer en environnement de développement
pilot env use dev

# Passer en environnement de production
pilot env use prod

# Afficher l'environnement actif
pilot env current
# → prod

# Vérifier l'état de prod sans changer l'env actif
pilot status --env prod
```

## Surcharge avec `--env`

Le flag `--env` permet d'utiliser un environnement spécifique pour une seule commande, sans modifier l'environnement actif stocké dans `.pilot-current-env` :

```bash
# L'env actif est "dev"
pilot env current
# → dev

# Déploie en prod pour cette commande uniquement
pilot deploy --env prod

# L'env actif est toujours "dev"
pilot env current
# → dev
```

## Environnements disponibles

Les environnements disponibles sont ceux déclarés dans la section `environments` de `pilot.yaml` :

```yaml
environments:
  dev:
    compose_file: docker-compose.dev.yml
    env_file: .env.dev
  staging:
    target: vps-staging
    compose_file: docker-compose.staging.yml
  prod:
    target: vps-prod
    compose_file: docker-compose.prod.yml
```

Ici, les environnements valides sont `dev`, `staging` et `prod`. pilot retourne une erreur si l'on tente d'utiliser un environnement non déclaré.
