# Variables d'environnement vides dans le container

## Symptôme

L'application tourne, `kaal status` montre les services `healthy`, mais les variables
d'environnement affichent des valeurs par défaut ou vides :

```
Environnement Actuel : Inconnu          ← attendu : local-dev
Clé Secrète         : Aucun secret trouvé ← attendu : ma_valeur
Version             : dev               ← attendu : 1.2.0
```

Le fichier `.env.dev` existe bien et contient les bonnes valeurs. La commande docker
`--mode dev` est bien passée. Pourtant les variables sont vides.

---

## Cause racine

### Le pattern ARG/ENV dans le Dockerfile

Un Dockerfile courant pour les projets Node/Vite ressemble à ça :

```dockerfile
# ❌ Pattern dangereux
ARG VITE_APP_ENV
ENV VITE_APP_ENV=$VITE_APP_ENV      # bake "" si pas de --build-arg

ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL   # idem
```

Quand `kaal up` lance `docker compose up --build` **sans** `--build-arg`
(ce qui est le cas pour l'environnement dev), Docker bake une **chaîne vide** dans
l'image :

```
VITE_APP_ENV=""
NEXT_PUBLIC_API_URL=""
```

### La priorité des process env vars dans tous les frameworks

Chaque framework web/backend lit les variables dans cet ordre, du plus prioritaire
au moins prioritaire :

| Priorité | Source |
|----------|--------|
| 1 (max) | **Process env** : ce que Docker a injecté dans le container |
| 2 | Fichiers `.env.*` sur disque (`.env.dev`, `.env.local`, etc.) |
| 3 | Valeurs par défaut dans le code |

Frameworks concernés :

| Framework | Variables build-time | Comportement |
|-----------|----------------------|--------------|
| **Vite** | `VITE_*` | Process env écrase `.env.{mode}` |
| **Next.js** | `NEXT_PUBLIC_*` | Process env écrase `.env.development` |
| **CRA** | `REACT_APP_*` | Process env écrase `.env.development` |
| **SvelteKit** | `PUBLIC_*` | Process env écrase `.env` |
| **Nuxt** | `NUXT_PUBLIC_*` | Process env écrase `.env` |
| **Angular** | variables de build | Process env écrase `environment.ts` |
| **Go** | `os.Getenv()` | Process env en priorité |
| **Python** | `os.environ` | Process env en priorité |
| **Node.js** | `process.env` | Process env en priorité |

Donc même si `--mode dev` dit à Vite de lire `.env.dev`, la chaîne vide `""`
dans le process env **gagne** sur `local-dev` dans le fichier. L'application voit `""`,
qui est falsy, et affiche la valeur de fallback.

---

## Diagnostic rapide

```bash
# Vérifier ce que le container voit réellement
docker exec <nom-container-app> env | grep VITE_
docker exec <nom-container-app> env | grep NEXT_PUBLIC_
docker exec <nom-container-app> env | grep REACT_APP_

# Si les vars sont vides ("") → c'est ce bug
# VITE_APP_ENV=          ← vide = baked "" depuis le Dockerfile
# VITE_APP_ENV=local-dev ← correct
```

---

## Fix 1 : Dockerfile : ARG seul suffit pour le build (fix permanent)

`ARG` vars sont disponibles comme process env dans chaque `RUN` : pas besoin de `ENV`.

```dockerfile
# ✅ Pattern correct : ARG seul, pas de ENV
ARG VITE_APP_ENV
ARG VITE_SUPER_SECRET_KEY
ARG NEXT_PUBLIC_API_URL

COPY . .
RUN npm run build   # ARG vars disponibles ici comme process env
```

Avec ce pattern, **rien n'est baked dans l'image**. Les containers dev démarrent sans
vars VITE_* dans leur process env : Vite lit alors `.env.dev` normalement.

> **Pourquoi ça marche pour le build prod ?**
> `kaal push` passe `--build-arg VITE_APP_ENV=prod` → la valeur est disponible pour
> `npm run build` via le ARG, baked dans le bundle JS. Le container runtime n'a pas
> besoin d'avoir cette var puisqu'elle est compilée dans le JS.

---

## Fix 2 : Compose : env_file sur chaque service app (filet de sécurité)

Même avec le Fix 1, ajouter `env_file` dans le compose est la pratique recommandée.
Elle garantit que les bonnes valeurs arrivent au container, **quelle que soit** la
version du Dockerfile ou le framework utilisé.

```yaml
# docker-compose.dev.yml
services:
  app:
    build:
      context: .
      target: builder
    command: npx vite --host 0.0.0.0 --port 8080 --mode dev
    env_file: .env.dev      # ← TOUJOURS présent
    ...

  backend:
    build: .
    env_file: .env.dev      # ← sur chaque service app
    ...
```

Les variables `env_file` dans docker-compose **écrasent** les `ENV` de l'image au
démarrage du container. C'est la source de vérité runtime.

---

## Fix 3 : Vite : --mode doit correspondre au suffixe du fichier .env

Vite lit les fichiers `.env.{mode}` selon la valeur de `--mode` :

| Commande | Fichier lu |
|----------|------------|
| `vite` (sans --mode) | `.env.development` |
| `vite --mode dev` | `.env.dev` |
| `vite --mode staging` | `.env.staging` |
| `vite --mode prod` | `.env.prod` |

Si ton fichier s'appelle `.env.dev`, la commande **doit** inclure `--mode dev`.
Sans ça, Vite cherche `.env.development` et ne trouve rien.

```yaml
# ✅ correct
command: npx vite --host 0.0.0.0 --port 8080 --mode dev

# ❌ incorrect : cherche .env.development, ignore .env.dev
command: npx vite --host 0.0.0.0 --port 8080
```

Note : `--mode` seul ne suffit pas si le process env contient des vides (Fix 1 + Fix 2
restent nécessaires).

---

## Frameworks non-Vite : même règle, noms différents

### Next.js

```dockerfile
# Dockerfile
ARG NEXT_PUBLIC_API_URL    # ✅ ARG seul
ARG NEXT_PUBLIC_APP_ENV
RUN npm run build
```

```yaml
# docker-compose.dev.yml
services:
  app:
    env_file: .env.dev
    command: node_modules/.bin/next dev -p 3000
```

### Create React App

```dockerfile
ARG REACT_APP_API_URL      # ✅ ARG seul
ARG REACT_APP_ENV
RUN npm run build
```

### SvelteKit / Nuxt / Angular

Même principe. `ARG` pour les vars build-time, jamais `ENV VAR=$ARG`,
`env_file` dans le compose pour injecter les valeurs runtime.

---

## Résumé des règles à retenir

| Règle | Raison |
|-------|--------|
| `ARG VAR` seul dans le Dockerfile pour les vars de build | `ARG` suffit pour `RUN`, `ENV` bake des `""` |
| `env_file: .env.<env>` sur chaque service app dans le compose | Process env override l'image et les fichiers `.env.*` |
| `--mode <env>` dans la commande Vite | Sans ça, Vite lit `.env.development`, pas `.env.dev` |
| Ne jamais hardcoder de valeurs dans le compose | Utiliser uniquement `env_file` + `${VAR}` |

---

## Protections intégrées dans kaal

kaal embarque désormais deux protections automatiques qui détectent ce problème **avant** qu'il cause des dégâts en production.

### Protection 1 : `kaal push` bloque si des vars sont manquantes dans build_args

Quand `registry.build_args` est explicitement défini dans `kaal.yaml`, `kaal push` scanne le fichier env actif à la recherche de vars compile-time (`VITE_*`, `NEXT_PUBLIC_*`, `REACT_APP_*`, `PUBLIC_*`, `NUXT_PUBLIC_*`, `NG_APP_*`) qui ne figurent **pas** dans `build_args`. Si des vars manquantes sont détectées, le push est **bloqué** avec un message d'erreur actionnable :

```
✗ 1 compile-time var(s) in .env.prod are NOT in kaal.yaml registry.build_args:
    - VITE_FEATURE_BROKEN

  These vars would be silently EMPTY in the built image.

  Fix: add them to kaal.yaml:
    registry:
      build_args:
    - VITE_FEATURE_BROKEN

  If these vars are intentionally excluded from the build, run:
    kaal push --force
```

### Protection 2 : `kaal preflight` détecte les deux gaps

`kaal preflight --target push` exécute deux nouveaux contrôles :
- `build_args_gap` (WARNING) : même vérification que ci-dessus, avant même que le push ne démarre
- `compose_env_file` (WARNING) : détecte les services applicatifs dans le compose file qui n'ont pas de `env_file` déclaré

La chaîne de protection complète est donc :
1. `kaal preflight` → avertit sur les deux gaps → l'agent ou le développeur corrige `kaal.yaml` / le compose
2. `kaal push` → bloque si le gap `build_args` subsiste après avoir sauté le preflight
3. `kaal push --force` → contournement quand des vars sont intentionnellement exclues

Les agents AI exécutent toujours le preflight en premier dans leur boucle, ils n'atteindront donc jamais le bloqueur de `kaal push`. Pour les développeurs qui sautent le preflight, le push joue le rôle de dernier filet de sécurité.

---

## Voir aussi

- [kaal up / kaal down](../commands/up-down.md)
- [Workflow dev local](../workflows/local-dev.md)
- [Variables dans CI/CD](../workflows/ci-cd.md)
