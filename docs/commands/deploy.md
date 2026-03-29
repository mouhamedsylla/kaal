# kaal deploy

Déploie l'application sur une cible distante.

```
kaal deploy [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement à déployer (défaut : env actif) |
| `--tag`, `-t` | Tag de l'image à déployer (défaut : SHA Git court) |
| `--target` | Nom du target (surcharge le target de l'environnement) |
| `--dry-run` | Affiche ce qui serait fait sans exécuter |

## Comportement (VPS)

1. Résout l'environnement et le target (`environments.<env>.target`)
2. Vérifie que le target est défini dans `kaal.yaml`
3. Connexion SSH au VPS (`targets.<name>.host`, `.user`, `.key`)
4. Copie `docker-compose.<env>.yml` sur le VPS
5. Exécute :
   ```bash
   docker compose -f docker-compose.<env>.yml pull
   docker compose -f docker-compose.<env>.yml up -d
   ```
6. Vérifie le health check des services
7. Affiche le statut post-déploiement

## Exemples

```bash
# Déployer l'env actif avec le SHA Git courant
kaal deploy

# Déployer prod avec un tag explicite
kaal deploy --env prod --tag v1.2.0

# Voir ce qui serait fait sans déployer
kaal deploy --env prod --tag v1.2.0 --dry-run

# Déployer sur un target spécifique
kaal deploy --env prod --target vps-backup --tag v1.2.0
```

## Prérequis

- Le target doit être défini dans `kaal.yaml`
- L'environnement doit référencer ce target (`environments.prod.target: vps-prod`)
- La clé SSH doit être accessible (`targets.vps-prod.key: ~/.ssh/id_kaal`)
- Docker installé sur le VPS
- `docker-compose.<env>.yml` doit exister localement (l'agent l'a généré)

## Voir aussi

- [`kaal push`](push.md) — construire et pousser l'image avant le deploy
- [`kaal rollback`](rollback.md) — revenir en arrière si le deploy échoue
- [`kaal status`](status.md) — vérifier l'état après le deploy
- [Workflow VPS complet](../workflows/deploy-vps.md)
