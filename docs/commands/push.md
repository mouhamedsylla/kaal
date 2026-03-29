# kaal push

Build l'image Docker et la pousse vers le registry configuré.

```
kaal push [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--tag`, `-t` | Tag de l'image (défaut : SHA Git court) |
| `--no-cache` | Désactive le cache Docker lors du build |
| `--env`, `-e` | Environnement (influence le Dockerfile à utiliser) |

## Comportement

1. Lit `registry.provider` et `registry.image` dans `kaal.yaml`
2. Résout le tag (paramètre ou SHA Git court du HEAD)
3. Authentification avec les credentials depuis les variables d'env
4. `docker build -t <image>:<tag> -f Dockerfile .`
5. `docker push <image>:<tag>`

## Exemples

```bash
# Push avec SHA Git automatique
kaal push

# Push avec tag explicite
kaal push --tag v1.2.0

# Push sans cache (utile en CI)
kaal push --tag $SHA --no-cache
```

## Variables d'environnement

| Registry | Variables requises |
|----------|-------------------|
| `ghcr` | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom` | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |

## Relation avec kaal deploy

`kaal push` et `kaal deploy` sont deux commandes séparées intentionnellement :
- `kaal push` = construire et archiver l'image (registry)
- `kaal deploy` = déployer une image existante (cible)

Ça permet de déployer la même image plusieurs fois (staging → prod) sans rebuild.

```bash
kaal push --tag v1.2.0
kaal deploy --env staging --tag v1.2.0
# tests...
kaal deploy --env prod --tag v1.2.0  # même image, pas de rebuild
```
