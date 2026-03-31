# kaal push

Build l'image Docker et la pousse vers le registry configuré.

```
kaal push [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--tag`, `-t` | Tag de l'image (défaut : SHA Git court) |
| `--env`, `-e` | Environnement dont le `.env` est lu pour les build args (défaut : env actif) |
| `--no-cache` | Désactive le cache Docker lors du build |
| `--platform` | Plateforme cible (défaut : `linux/amd64`) |
| `--force` | Ignore la vérification des vars compile-time manquantes. À utiliser quand une var `VITE_*` / `NEXT_PUBLIC_*` est intentionnellement exclue de l'image (ex : uniquement nécessaire à l'exécution via `env_file`). Sans `--force`, `kaal push` bloque si des vars compile-time présentes dans le fichier env sont absentes de `registry.build_args`. |

## Comportement

1. Lit `registry.provider` et `registry.image` dans `kaal.yaml`
2. Résout le tag (`--tag` ou SHA Git court du HEAD)
3. **Détection de plateforme** : sur macOS ARM64 (Apple Silicon), build automatiquement pour `linux/amd64` sans intervention
4. **Auto-injection des vars compile-time** : pour les stacks Node, scanne le `.env.<env>` et passe tous les `VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*` en `--build-arg`
5. **Patch transparent du Dockerfile** : si des `ARG` manquent pour les vars injectées, kaal les ajoute dans un fichier temporaire avant le build ; l'original n'est jamais modifié
6. Authentification avec les credentials depuis les variables d'env
7. `docker build --platform linux/amd64 -t <image>:<tag> .`
8. `docker push <image>:<tag>`

## Gestion automatique des vars compile-time (Vite, Next.js, CRA)

Les variables `VITE_*` doivent être connues au moment du `npm run build`, pas à l'exécution. kaal s'en charge sans config :

```bash
# .env.prod
VITE_APP_ENV=prod
VITE_API_URL=https://api.mon-app.com

kaal push --env prod
# → Injecting build args: VITE_APP_ENV, VITE_API_URL
#   ARG/ENV lines auto-injected into builder stage (original Dockerfile unchanged)
# → Building mouhamedsylla/my-app:abc1234 [linux/amd64]
```

Pas de config dans `kaal.yaml`. Pas de modification manuelle du Dockerfile. kaal détecte les conventions de nommage standard.

Pour des vars non-standard, déclaration explicite optionnelle :

```yaml
# kaal.yaml
registry:
  build_args:
    - MY_CUSTOM_BUILD_VAR
```

## Détection des vars manquantes

Quand `registry.build_args` est défini dans `kaal.yaml`, `kaal push` vérifie que toutes les vars `VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`, `PUBLIC_*`, `NUXT_PUBLIC_*` et `NG_APP_*` présentes dans le fichier env sont bien listées dans `build_args`.

Si des vars manquent, **le push est bloqué** avec les instructions exactes pour corriger la situation :

```
✗ 1 compile-time var(s) in .env.prod are NOT in kaal.yaml registry.build_args:
    - VITE_FEATURE_BROKEN

  Fix: add them to kaal.yaml:
    registry:
      build_args:
    - VITE_FEATURE_BROKEN

  If these vars are intentionally excluded from the build, run:
    kaal push --force
```

Pour corriger, ajouter les vars manquantes dans `kaal.yaml` :

```yaml
registry:
  build_args:
    - VITE_APP_ENV
    - VITE_API_URL
    - VITE_FEATURE_BROKEN   # ← var ajoutée
```

Si la var est intentionnellement exclue de l'image (ex : elle n'est nécessaire qu'à l'exécution via `env_file` et non au moment du build), utiliser `kaal push --force` pour contourner la vérification.

> **Conseil :** `kaal preflight --target push` exécute cette même vérification en amont via le contrôle `build_args_gap`, avant même que le push ne commence. Les agents AI appellent toujours le preflight en premier : le bloqueur du push n'est là que pour les humains qui sautent le preflight.

## Exemples

```bash
# Push avec SHA Git automatique (linux/amd64 par défaut)
kaal push

# Push pour prod (lit .env.prod pour les build args VITE_*)
kaal push --env prod

# Push avec tag explicite
kaal push --tag v1.2.0

# Push sans cache (utile en CI)
kaal push --tag $SHA --no-cache

# Push multi-architecture
kaal push --platform linux/amd64,linux/arm64

# Push pour un VPS ARM
kaal push --platform linux/arm64
```

## Variables d'environnement

| Registry | Variables requises |
|----------|-------------------|
| `ghcr` | `GITHUB_TOKEN`, `GITHUB_ACTOR` |
| `dockerhub` | `DOCKER_USERNAME`, `DOCKER_PASSWORD` |
| `custom` | `REGISTRY_USERNAME`, `REGISTRY_PASSWORD` |

## Relation avec kaal deploy

`kaal push` et `kaal deploy` sont intentionnellement séparés :
- `kaal push` : construit et archive l'image (registry)
- `kaal deploy` : déploie une image existante (cible)

```bash
kaal push --tag v1.2.0
kaal deploy --env staging --tag v1.2.0
# tests...
kaal deploy --env prod --tag v1.2.0   # même image, pas de rebuild
```

## Voir aussi

- [`kaal preflight`](../workflows/deploy-vps.md) : vérifier les prérequis avant de pousser
- [`kaal deploy`](deploy.md) : déployer l'image poussée
