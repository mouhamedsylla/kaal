# kaal preflight

Vérifie tous les prérequis avant d'exécuter `kaal push` ou `kaal deploy`.

```
kaal preflight [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--target`, `-t` | Cible de vérification : `up`, `push`, `deploy` (défaut : `deploy`) |
| `--env`, `-e` | Environnement à vérifier (défaut : env actif) |
| `--json` | Sortie JSON structurée |

## Les 13 vérifications

| # | Nom | Ce qui est vérifié | Type de correction |
|---|-----|-------------------|--------------------|
| 1 | `kaal_yaml` | `kaal.yaml` existe et est syntaxiquement valide | FixHuman |
| 2 | `registry_image` | `registry.image` est renseigné et n'est pas un placeholder | FixHuman |
| 3 | `dockerfile` | Un `Dockerfile` existe à la racine du projet | FixAgent |
| 4 | `docker_daemon` | Le démon Docker est en cours d'exécution | FixHuman |
| 5 | `registry_creds` | Les variables d'authentification registry sont exportées | FixHuman |
| 6 | `compose_file` | `docker-compose.<env>.yml` existe | FixAgent |
| 7 | `compose_env_file` | Tous les services applicatifs déclarent `env_file` dans le compose | FixAgent |
| 8 | `build_args_gap` | Toutes les variables compile-time du `.env` sont listées dans `registry.build_args` | FixHuman |
| 9 | `target_host` | La cible de déploiement est configurée dans `kaal.yaml` | FixHuman |
| 10 | `ssh_key` | La clé SSH est disponible (fichier ou variable `KAAL_SSH_KEY`) | FixHuman |
| 11 | `vps_connectivity` | La connexion SSH au VPS aboutit | FixHuman |
| 12 | `vps_docker_group` | L'utilisateur deploy peut exécuter docker sans sudo | FixAgent |
| 13 | `vps_env_file` | Le fichier env est synchronisé sur le VPS (`~/kaal/.env.prod` existe) | FixAgent |

### Détail des vérifications

**1. kaal_yaml** : Lit et parse `kaal.yaml`. Échoue si le fichier est absent ou contient une erreur YAML.
Correction : exécuter `kaal init` pour créer le fichier, ou corriger la syntaxe manuellement.

**2. registry_image** : Vérifie que `registry.image` est défini et ne contient pas `your-image` ou une valeur vide.
Correction : éditer `kaal.yaml` et renseigner l'image complète (ex: `ghcr.io/mouhamedsylla/mon-projet`).

**3. dockerfile** : Vérifie l'existence de `Dockerfile` à la racine.
Correction : l'agent MCP peut le générer via `kaal_generate_dockerfile`.

**4. docker_daemon** : Tente une connexion au socket Docker local.
Correction : démarrer Docker Desktop ou le démon Docker.

**5. registry_creds** : Vérifie la présence des variables d'environnement selon le provider :
- `dockerhub` → `DOCKER_USERNAME` + `DOCKER_PASSWORD`
- `ghcr` → `GITHUB_TOKEN` + `GITHUB_ACTOR`

Correction : exporter les variables dans le shell avant d'exécuter `kaal push`.

**6. compose_file** : Vérifie que `docker-compose.<env>.yml` existe pour l'environnement actif.
Correction : l'agent MCP peut le générer via `kaal_generate_compose`.

**7. compose_env_file** *(avertissement)* : Vérifie que tous les services applicatifs déclarent `env_file` dans le compose. Sans cette directive, les variables `VITE_*`, `NEXT_PUBLIC_*` et autres variables runtime seront vides au démarrage du conteneur.
Correction : l'agent MCP peut ajouter la directive `env_file` aux services concernés.

**8. build_args_gap** *(avertissement)* : Compare les variables du fichier `.env` avec la section `registry.build_args` dans `kaal.yaml`. Si une variable compile-time est présente dans `.env` mais absente de `build_args`, elle sera silencieusement vide dans l'image construite.
Correction : ajouter les variables manquantes dans `registry.build_args` de `kaal.yaml`.

**9. target_host** : Vérifie qu'une section `targets` est configurée dans `kaal.yaml` pour l'environnement courant.
Correction : ajouter la section `targets` dans `kaal.yaml` avec `host`, `user`, `key`.

**10. ssh_key** : Vérifie que le fichier de clé SSH référencé dans `kaal.yaml` existe, ou que la variable `KAAL_SSH_KEY` est exportée avec le contenu de la clé.
Correction : générer une clé avec `ssh-keygen` ou exporter `KAAL_SSH_KEY`.

**11. vps_connectivity** : Ouvre une connexion SSH réelle au VPS.
Correction : vérifier le `host`, le `port`, la clé SSH, et que le serveur SSH du VPS est accessible.

**12. vps_docker_group** : Se connecte en SSH et vérifie que l'utilisateur deploy appartient au groupe `docker`.
Correction : exécuter `kaal setup --env <env>` : l'agent MCP peut déclencher cette action.

**13. vps_env_file** : Se connecte en SSH et vérifie que `~/kaal/.env.<env>` existe sur le VPS.
Correction : exécuter `kaal sync --env <env>` : l'agent MCP peut déclencher `kaal_sync`.

## Sortie

```
→ Running preflight checks for deploy (env: prod)

  ✓  kaal_yaml            kaal.yaml valide
  ✓  registry_image       ghcr.io/mouhamedsylla/mon-projet
  ✓  dockerfile           Dockerfile trouvé
  ✓  docker_daemon        Docker en cours d'exécution
  ✓  registry_creds       GITHUB_TOKEN + GITHUB_ACTOR présents
  ✓  compose_file         docker-compose.prod.yml trouvé
  ⚠  compose_env_file     Service "app" ne déclare pas env_file
  ⚠  build_args_gap       VITE_API_URL présent dans .env mais absent de build_args
  ✓  target_host          vps-prod → 1.2.3.4
  ✓  ssh_key              ~/.ssh/id_kaal trouvé
  ✓  vps_connectivity     SSH OK (1.2.3.4:22)
  ✓  vps_docker_group     deploy ∈ docker
  ✗  vps_env_file         ~/kaal/.env.prod introuvable → kaal sync

2 avertissements, 1 bloquant
→ Exécuter : kaal sync --env prod
```

### Sortie JSON (`--json`)

```json
{
  "env": "prod",
  "target": "deploy",
  "checks": [
    {
      "name": "kaal_yaml",
      "status": "ok",
      "message": "kaal.yaml valide"
    },
    {
      "name": "compose_env_file",
      "status": "warning",
      "message": "Service \"app\" ne déclare pas env_file",
      "fix_type": "FixAgent",
      "fix_action": "add_env_file_directive"
    },
    {
      "name": "vps_env_file",
      "status": "error",
      "message": "~/kaal/.env.prod introuvable",
      "fix_type": "FixAgent",
      "fix_action": "kaal_sync"
    }
  ],
  "blockers": 1,
  "warnings": 2,
  "ok": false
}
```

## Codes de sortie

| Code | Signification |
|------|---------------|
| `0` | Toutes les vérifications passent (ou seulement des avertissements) |
| `1` | Au moins un bloquant détecté |

## Quand l'exécuter

- Avant le premier `kaal push` sur un nouveau projet
- Avant le premier `kaal deploy` vers un nouveau VPS
- En cas d'erreur inexpliquée lors du push ou du déploiement
- En routine CI/CD comme étape de validation initiale

## Utilisation par les agents IA

Le champ `fix_type` dans la sortie JSON indique à l'agent ce qu'il peut corriger seul :

| Valeur | Signification |
|--------|---------------|
| `FixAgent` | L'agent peut corriger via un outil MCP (`kaal_generate_dockerfile`, `kaal_sync`, `kaal_setup`…) |
| `FixHuman` | L'action requiert une intervention humaine (exporter une variable, éditer un fichier de config, démarrer Docker) |

Un agent IA bien conçu exécute `kaal_preflight` en premier, traite automatiquement tous les `FixAgent`, puis demande à l'humain de résoudre les `FixHuman` avant de continuer.
