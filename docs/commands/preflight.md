# pilot preflight

Vérifie tous les prérequis avant `pilot push` ou `pilot deploy`, et **génère `pilot.lock`**.

```
pilot preflight [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--target`, `-t` | Cible de vérification : `up`, `push`, `deploy` (défaut : `deploy`) |
| `--env`, `-e` | Environnement à vérifier (défaut : env actif) |
| `--json` | Sortie JSON structurée |

## Ce que fait preflight

`pilot preflight --target deploy` fait deux choses :

1. **Vérifie** tous les prérequis (Docker, SSH, registry, clés, fichiers...)
2. **Génère `pilot.lock`** si tout passe : le fichier qui autorise le prochain `pilot deploy`

`pilot.lock` doit être **commité dans le dépôt**. C'est le contrat signé : ce qui a été validé par l'équipe est ce qui s'exécute en production.

```bash
pilot preflight --target deploy --env prod
git add pilot.lock
git commit -m "chore: update pilot.lock"
```

Si `pilot.lock` est absent ou périmé, `pilot deploy` refuse de continuer.

---

## Les 13 vérifications

| # | Nom | Ce qui est vérifié | Type de correction |
|---|-----|-------------------|--------------------|
| 1 | `pilot_yaml` | `pilot.yaml` existe et est syntaxiquement valide | FixHuman |
| 2 | `registry_image` | `registry.image` est renseigné et n'est pas un placeholder | FixHuman |
| 3 | `dockerfile` | Un `Dockerfile` existe à la racine du projet | FixAgent |
| 4 | `docker_daemon` | Le démon Docker est en cours d'exécution | FixHuman |
| 5 | `registry_creds` | Les variables d'authentification registry sont exportées | FixHuman |
| 6 | `compose_file` | `docker-compose.<env>.yml` existe | FixAgent |
| 7 | `compose_env_file` | Tous les services applicatifs déclarent `env_file` dans le compose | FixAgent |
| 8 | `build_args_gap` | Toutes les variables compile-time du `.env` sont dans `registry.build_args` | FixHuman |
| 9 | `target_host` | La cible de déploiement est configurée dans `pilot.yaml` | FixHuman |
| 10 | `ssh_key` | La clé SSH est disponible (fichier ou variable `PILOT_SSH_KEY`) | FixHuman |
| 11 | `vps_connectivity` | La connexion SSH au VPS aboutit | FixHuman |
| 12 | `vps_docker_group` | L'utilisateur deploy peut exécuter docker sans sudo | FixAgent |
| 13 | `vps_env_file` | Le fichier env est synchronisé sur le VPS (`~/pilot/.env.prod` existe) | FixAgent |

### Détail des vérifications

**1. pilot_yaml** : Lit et parse `pilot.yaml`. Échoue si le fichier est absent ou contient une erreur YAML.

**2. registry_image** : Vérifie que `registry.image` est défini et ne contient pas `your-image` ou une valeur vide.

**3. dockerfile** : Vérifie l'existence de `Dockerfile`. L'agent MCP peut le générer via `pilot_generate_dockerfile`.

**4. docker_daemon** : Tente une connexion au socket Docker local.

**5. registry_creds** : Vérifie la présence des variables d'env selon le provider (`GITHUB_TOKEN`+`GITHUB_ACTOR` pour ghcr, `DOCKER_USERNAME`+`DOCKER_PASSWORD` pour dockerhub...).

**6. compose_file** : Vérifie que `docker-compose.<env>.yml` existe pour l'environnement actif.

**7. compose_env_file** *(avertissement)* : Vérifie que tous les services applicatifs déclarent `env_file` dans le compose. Sans cette directive, les variables `VITE_*`, `NEXT_PUBLIC_*` seront vides au démarrage.

**8. build_args_gap** *(avertissement)* : Compare les variables du fichier `.env` avec `registry.build_args` dans `pilot.yaml`. Si une variable compile-time est présente dans `.env` mais absente de `build_args`, elle sera silencieusement vide dans l'image.

**9. target_host** : Vérifie qu'une section `targets` est configurée pour l'environnement.

**10. ssh_key** : Vérifie que le fichier de clé SSH référencé dans `pilot.yaml` existe, ou que `PILOT_SSH_KEY` est exporté.

**11. vps_connectivity** : Ouvre une connexion SSH réelle au VPS.

**12. vps_docker_group** : Vérifie que l'utilisateur deploy appartient au groupe `docker`. Correction : `pilot setup --env <env>`.

**13. vps_env_file** : Vérifie que `~/pilot/.env.<env>` existe sur le VPS. Correction : `pilot sync --env <env>`.

---

## Génération de `pilot.lock`

Quand `--target deploy` et que toutes les vérifications passent (ou n'ont que des avertissements), preflight génère `pilot.lock` :

```yaml
# pilot.lock : generated automatically, commit this file.
schema_version: 1
generated_at: 2026-04-11T14:00:00Z
generated_from:
  - pilot.yaml
  - docker-compose.prod.yml
  - prisma/schema.prisma
project_hash: "abc123..."

execution_plan:
  nodes_active: [preflight, migrations, deploy, post_hooks, healthcheck]
  migrations:
    tool: prisma
    command: npx prisma migrate deploy
    rollback_command: npx prisma migrate rollback
    reversible: true
    detected_from: prisma/schema.prisma
execution_provider: compose
```

**Ce que pilot.lock encode :**
- Les fichiers sources qui ont été validés (avec leur hash SHA-256)
- Les étapes actives du pipeline de déploiement
- La configuration de migrations auto-détectée (outil, commande, rollback, réversibilité)
- Le provider d'exécution (compose, k8s...)

**Auto-détection des migrations :**

| Fichier détecté | Outil | Commande |
|-----------------|-------|----------|
| `prisma/schema.prisma` | prisma | `npx prisma migrate deploy` |
| `alembic.ini` | alembic | `alembic upgrade head` |
| `flyway.conf` | flyway | `flyway migrate` |
| `db/migrations/` | goose | `goose up` |
| `migrations/` | goose | `goose -dir migrations up` |

La détection auto ne définit pas `rollback_command` ni `reversible: true` : déclare-les explicitement dans `pilot.yaml` si tu veux le rollback de migration automatique.

---

## Sortie terminal

```
→ Running preflight checks for deploy (env: prod)

  ✓  pilot_yaml            pilot.yaml valide
  ✓  registry_image       ghcr.io/mouhamedsylla/mon-projet
  ✓  dockerfile           Dockerfile trouvé
  ✓  docker_daemon        Docker en cours d'exécution
  ✓  registry_creds       GITHUB_TOKEN + GITHUB_ACTOR présents
  ✓  compose_file         docker-compose.prod.yml trouvé
  ⚠  compose_env_file     Service "app" ne déclare pas env_file
  ⚠  build_args_gap       VITE_API_URL présent dans .env mais absent de build_args
  ✓  target_host          vps-prod → 1.2.3.4
  ✓  ssh_key              ~/.ssh/id_pilot trouvé
  ✓  vps_connectivity     SSH OK (1.2.3.4:22)
  ✓  vps_docker_group     deploy ∈ docker
  ✗  vps_env_file         ~/pilot/.env.prod introuvable → pilot sync

2 avertissements, 1 bloquant
→ Exécuter : pilot sync --env prod
```

Quand tout passe :

```
✓ All checks passed : pilot.lock generated
→ Commit pilot.lock to your repository
```

---

## Sortie JSON (`--json`)

```json
{
  "env": "prod",
  "target": "deploy",
  "checks": [
    {"name": "pilot_yaml", "status": "ok", "message": "pilot.yaml valide"},
    {
      "name": "vps_env_file",
      "status": "error",
      "message": "~/pilot/.env.prod introuvable",
      "fix_type": "agent",
      "fix_action": "pilot_sync"
    }
  ],
  "blockers": 1,
  "warnings": 2,
  "ok": false,
  "lock_generated": false
}
```

---

## Codes de sortie

| Code | Signification |
|------|---------------|
| `0` | Toutes les vérifications passent (ou seulement des avertissements) : `pilot.lock` généré |
| `1` | Au moins un bloquant détecté : `pilot.lock` non généré |

---

## Quand l'exécuter

- Avant le premier `pilot push` sur un nouveau projet
- Avant le premier `pilot deploy` vers un nouveau VPS
- Après avoir modifié `pilot.yaml`, le compose file ou le schéma de migration
- En CI comme première étape de validation (le résultat est déjà commité normalement)

---

## Utilisation par les agents IA

Le champ `fix_type` dans la sortie JSON indique à l'agent ce qu'il peut corriger seul :

| Valeur | Signification |
|--------|---------------|
| `agent` | L'agent peut corriger via un outil MCP (`pilot_generate_dockerfile`, `pilot_sync`, `pilot_setup`…) |
| `human` | L'action requiert une intervention humaine (exporter une variable, démarrer Docker, SSH, firewall...) |

Un agent bien conçu exécute `pilot_preflight` en premier, traite tous les `agent`, puis demande à l'humain de résoudre les `human` avant de continuer.

## Voir aussi

- [`pilot plan`](plan.md) : afficher le plan issu de `pilot.lock` sans déployer
- [`pilot deploy`](deploy.md) : exécuter le plan validé
- [Architecture : pilot.lock](../architecture.md#pilotlock)
