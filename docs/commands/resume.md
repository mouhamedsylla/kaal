# pilot resume

Reprend une opération suspendue après une erreur TypeC (choix requis).

```
pilot resume [--answer <option>] [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--answer`, `-a` | Répondre par index (`0`, `1`) ou par texte exact |

## Description

Certaines erreurs pilot nécessitent un choix humain avant de continuer. Dans ce cas, pilot **suspend l'opération**, affiche les options disponibles, et attend.

`pilot resume` reprend depuis le début de l'opération en appliquant le choix fourni (ou en demandant interactivement).

## Exemple de suspension

```bash
pilot deploy --env prod

  ✓  lock check     OK
  ✓  sync           4 files → ~/pilot/

  ✗  user "deploy" n'est pas dans le groupe docker sur 1.2.3.4

     Actions possibles :
     → [0] pilot setup --env prod   (recommandé)
       [1] ssh deploy@1.2.3.4 'sudo usermod -aG docker deploy'

     Après avoir pris une action : pilot resume
```

L'état est sauvegardé dans `.pilot/suspended.json`.

## Reprendre

```bash
# Par index (recommandé)
pilot resume --answer 0

# Par texte exact
pilot resume --answer "pilot setup --env prod"

# Interactif (pilot affiche les options)
pilot resume
```

## Fichier `.pilot/suspended.json`

```json
{
  "error_code": "PILOT-DEPLOY-003",
  "command": "pilot deploy --env prod",
  "args": {"env": "prod"},
  "options": [
    "pilot setup --env prod",
    "ssh deploy@1.2.3.4 'sudo usermod -aG docker deploy'"
  ],
  "recommended": "pilot setup --env prod",
  "suspended_at": "2026-04-11T14:32:00Z"
}
```

`.pilot/` est dans `.gitignore` et ne doit pas être commité.

## Pour les agents AI

L'output JSON (`--json`) d'une erreur TypeC contient tout le nécessaire pour décider :

```json
{
  "status": "awaiting_choice",
  "code": "PILOT-DEPLOY-003",
  "message": "user 'deploy' is not in the docker group on 1.2.3.4",
  "options": ["pilot setup --env prod", "ssh deploy@1.2.3.4 '...'"],
  "recommended": "pilot setup --env prod",
  "resume_with": "pilot resume --answer 0"
}
```

L'agent exécute `resume_with` après avoir pris l'action recommandée.

## Voir aussi

- [`pilot diagnose`](diagnose.md) : vérifier s'il y a une suspension en cours
- [Architecture — cycle TypeC](../architecture.md#typec-suspension--resume-cycle)
