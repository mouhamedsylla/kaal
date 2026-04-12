# pilot diagnose

Snapshot complet de l'état du système : Docker, SSH, ports, git, registry, suspension en cours.

```
pilot diagnose [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement à diagnostiquer (défaut : env actif) |

## Description

`pilot diagnose` exécute une série de vérifications et produit un rapport structuré. Utile avant un déploiement ou pour déboguer un environnement cassé. Fonctionne même sans `pilot.yaml`.

## Exemple

```
  pilot diagnose  (env: prod)

  ─── System ──────────────────────────────────────
  ✓  Docker CLI                    24.0.5
  ✓  Docker daemon                 running
  ✓  docker compose                2.21.0

  ─── Project ─────────────────────────────────────
  ✓  pilot.yaml                    found
  ✓  Dockerfile                    found
  ✓  .env.prod                     found
  ✗  docker-compose.prod.yml       not found

  ─── Ports ───────────────────────────────────────
  ✓  8080                          free
  ✗  5432                          in use (postgres)

  ─── Registry ────────────────────────────────────
  ✓  ghcr.io                       reachable

  ─── SSH ─────────────────────────────────────────
  ✓  ~/.ssh/id_pilot               found (permissions: 600)
  ✗  1.2.3.4:22                    connection refused

  ─── Git ─────────────────────────────────────────
  ✓  branch                        main
  ✓  working tree                  clean
  ✓  last commit                   abc1234 feat: add nginx

  11/13 checks passed : 2 issue(s) found
```

## Catégories de vérifications

| Catégorie | Vérifications |
|-----------|---------------|
| **System** | Docker CLI présent, daemon actif, plugin compose |
| **Project** | pilot.yaml, Dockerfile, .env file, compose file |
| **Ports** | Ports libres vs occupés (avec processus owner si possible) |
| **Registry** | Connectivité TCP vers le registry déclaré |
| **SSH** | Présence + permissions de la clé SSH, connectivité TCP vers le VPS |
| **Git** | Branche active, état du working tree, dernier commit |

## Suspension en cours

Si une opération TypeC est suspendue, `pilot diagnose` l'affiche :

```
  ─── Pending choice ──────────────────────────────
  ⚠  [PILOT-DEPLOY-003] pilot deploy --env prod
     Suspended: 2026-04-11 14:32:00
     Run: pilot resume [--answer <option>]
```

## Voir aussi

- [`pilot resume`](resume.md) : reprendre une opération suspendue
- [`pilot preflight`](preflight.md) : vérifications pré-déploiement avec plan d'action
