# pilot plan

Affiche le plan d'exécution du prochain déploiement sans rien exécuter.

```
pilot plan [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement cible (défaut : env actif) |

## Description

`pilot plan` lit `pilot.lock` et affiche :

- Les étapes actives du pipeline de déploiement (dans l'ordre)
- Lesquelles sont compensables (rollback possible)
- Le plan de compensation LIFO en cas d'échec

Rien n'est exécuté. Requiert un `pilot.lock` valide — lancez `pilot preflight` d'abord.

## Exemple

```
  Execution plan — pilot deploy --env prod

  Steps
  ──────────────────────────────────────────────────
  [1] preflight        verify config, secrets, SSH reachability
  [2] migrations       run prisma migrations (npx prisma migrate deploy) — reversible  (compensable)
  [3] deploy           pull image + docker compose up  [provider: compose]  (compensable)
  [4] post_hooks       run post-deploy hooks on remote
  [5] healthcheck      wait for all services healthy

  Compensation plan  (LIFO — executed on failure)
  ──────────────────────────────────────────────────
  [1] deploy           restore previous image tag
  [2] migrations       npx prisma migrate rollback

  To execute this plan:  pilot deploy --env prod
  To preview only:       pilot deploy --env prod --dry-run
```

## Notes

- Si `pilot.lock` est absent ou périmé, `pilot plan` affiche un plan par défaut avec un avertissement.
- Le plan affiché est identique à ce que `pilot deploy --dry-run` rendrait.
- Seules les étapes présentes dans `pilot.lock.execution_plan.nodes_active` sont affichées.

## Voir aussi

- [`pilot preflight`](preflight.md) : générer `pilot.lock`
- [`pilot deploy --dry-run`](deploy.md) : même plan + confirmation d'exécution
- [Architecture — pipeline de déploiement](../architecture.md#the-deploy-pipeline)
