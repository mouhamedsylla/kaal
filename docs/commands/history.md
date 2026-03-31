# kaal history

Affiche les 10 derniers enregistrements de déploiement pour l'environnement actif.

```
kaal history [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement cible (défaut : env actif) |
| `--json` | Sortie JSON structurée |

## Source de données

kaal se connecte au VPS via SSH et lit le fichier `~/.kaal/<project-name>/deployments.json`. Ce fichier est mis à jour automatiquement à chaque `kaal deploy` et `kaal rollback`.

L'historique est limité aux **10 déploiements les plus récents**. Les enregistrements les plus anciens sont supprimés automatiquement au-delà de cette limite.

## Colonnes de sortie

| Colonne | Description |
|---------|-------------|
| `TAG` | Tag de l'image déployée |
| `ENV` | Environnement cible |
| `DATE` | Horodatage du déploiement (UTC) |
| `STATUS` | Résultat : `ok` ou `failed` |
| `MESSAGE` | Détail succinct (ex: message d'erreur en cas d'échec) |

## Exemple de sortie

```
→ Deployment history prod (vps-prod · 1.2.3.4)

  TAG         ENV    DATE                  STATUS   MESSAGE
  ────────────────────────────────────────────────────────────────────
  def5678     prod   2026-03-31 10:15:32   failed   healthcheck timeout
  abc1234     prod   2026-03-31 09:48:11   ok       -
  9f3e210     prod   2026-03-30 17:22:05   ok       -
  7a1b3c4     prod   2026-03-29 14:10:48   ok       -
  ...
```

### Sortie JSON (`--json`)

```json
{
  "env": "prod",
  "target": "vps-prod",
  "deployments": [
    {
      "tag": "def5678",
      "env": "prod",
      "timestamp": "2026-03-31T10:15:32Z",
      "status": "failed",
      "message": "healthcheck timeout"
    },
    {
      "tag": "abc1234",
      "env": "prod",
      "timestamp": "2026-03-31T09:48:11Z",
      "status": "ok",
      "message": ""
    }
  ]
}
```

## Cas d'usage

- **Connaître la version en production** : le premier enregistrement `ok` est la version actuellement stable
- **Identifier un tag pour le rollback** : trouver un tag stable et le passer à `kaal rollback --version <tag>`
- **Suivre les déploiements échoués** : repérer des patterns d'instabilité avant de déployer à nouveau

```bash
# Voir l'historique, puis rollback vers un tag précis
kaal history --env prod
kaal rollback --version 9f3e210 --env prod
```
