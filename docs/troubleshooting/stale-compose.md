# Compose désynchronisé après modification de pilot.yaml

## Symptôme

```
✗ docker-compose.dev.yml is stale : pilot.yaml has changed since it was generated

  Regenerate it:
    Ask your AI agent: "Regenerate the compose file for the dev environment"

  The agent will call pilot_generate_compose with the updated pilot.yaml.
  Your existing compose file will be replaced.
```

Cette erreur apparaît au lancement de `pilot up`.

---

## Cause

pilot enregistre un hash SHA-256 de `pilot.yaml` dans `.pilot/compose-meta.json` chaque fois
que l'agent génère un compose via `pilot_generate_compose`. À chaque `pilot up`, pilot compare
le hash actuel de `pilot.yaml` avec le hash enregistré.

Si les deux diffèrent, le compose peut ne plus refléter l'infrastructure réelle :
un nouveau service manque peut-être, un port a changé, un service managé a été ajouté.

**Modifications courantes qui déclenchent l'avertissement :**
- `pilot add <service>` : nouveau service dans `pilot.yaml`
- Modification manuelle de `pilot.yaml` (changement de version, de port, de `hosting`…)
- Suppression de `.pilot/` (le hash est perdu : compose considéré non-généré, non bloquant)

---

## Résolution

### Option 1 : Via ton agent IA (recommandé)

Dans Claude Code ou Cursor :

> *"Régénère les fichiers compose pour tous les environnements"*

ou spécifiquement :

> *"Régénère le compose pour l'environnement dev"*

L'agent appelle `pilot_context` (reçoit le `pilot.yaml` à jour), génère un nouveau compose,
puis appelle `pilot_generate_compose`. Le hash est automatiquement mis à jour.

### Option 2 : Via `pilot context` dans un chat AI

```bash
pilot context
```

Colle le résultat dans ton chat AI et demande de régénérer le compose.
Remplace le fichier existant avec le contenu généré.

---

## Fonctionnement interne

Le fichier `.pilot/compose-meta.json` enregistre les informations de génération par environnement :

```json
{
  "envs": {
    "dev": {
      "pilot_yaml_hash": "a1b2c3d4...",
      "generated_at": "2025-01-15T10:30:00Z",
      "compose_file": "docker-compose.dev.yml"
    },
    "prod": {
      "pilot_yaml_hash": "a1b2c3d4...",
      "generated_at": "2025-01-15T10:30:01Z",
      "compose_file": "docker-compose.prod.yml"
    }
  }
}
```

Ce fichier est dans `.pilot/`, qui est gitignored automatiquement. Il est local à ta machine
et ne doit pas être commité.

---

## Cas particuliers

### `.pilot/` supprimé ou absent

Si `.pilot/compose-meta.json` n'existe pas, pilot n'a aucun hash de référence.
Dans ce cas, **la vérification est ignorée** et `pilot up` continue normalement.
Ce comportement est intentionnel : supprimer `.pilot/` est toujours sûr.

### Compose généré manuellement (sans passer par pilot_generate_compose)

Si tu as écrit le compose à la main ou copié-collé sans passer par le tool MCP,
aucun hash n'est enregistré. La vérification de staleness ne s'applique pas.
Tu peux forcer l'enregistrement en demandant à ton agent de passer par `pilot_generate_compose`
même si le compose est déjà correct.

### Environments multiples

La vérification est faite environnement par environnement. Un compose prod peut être
considéré stale même si le compose dev est à jour (si le compose prod a été généré avant
la dernière modification de `pilot.yaml`).

---

## Voir aussi

- [Guide : Ajouter un service](../guide/adding-services.md) : workflow complet après `pilot add`
- [`pilot up`](../commands/up-down.md) : comportement lors des fichiers manquants
- [`pilot_generate_compose`](../workflows/ai-agent.md#pilot_generate_compose) : tool MCP qui met à jour le hash
