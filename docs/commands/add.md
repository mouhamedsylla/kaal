# pilot add

Ajoute un service à un projet pilot existant.

```
pilot add [type] [flags]
```

---

## Description

`pilot add` met à jour `pilot.yaml` et `.env.example` pour inclure un nouveau service.
Il ne modifie aucun autre fichier existant. Les fichiers compose sont régénérés séparément
par ton agent IA (ou manuellement) après l'ajout.

---

## Arguments

| Argument | Description |
|----------|-------------|
| `[type]` | Type du service à ajouter (optionnel : wizard si absent) |

Types disponibles : `postgres`, `mysql`, `mongodb`, `redis`, `rabbitmq`, `nats`, `kafka`, `elasticsearch`, `storage`, `nginx`, `traefik`, `worker`

---

## Flags

| Flag | Alias | Type | Description |
|------|-------|------|-------------|
| `--name` | `-n` | string | Nom du service dans pilot.yaml. Défaut : le type |
| `--managed` | `-m` | bool | Mode hébergement managé (fournisseur cloud) |
| `--provider` | `-p` | string | Fournisseur managé : requis si `--managed` et `--yes` |
| `--yes` | `-y` | bool | Non-interactif : accepte les défauts (container hosting) |

---

## Exemples

```bash
# Wizard interactif complet
pilot add

# Choisir le type, wizard pour le reste
pilot add redis

# Service managé, wizard pour le fournisseur
pilot add postgres --managed

# Entièrement non-interactif
pilot add postgres --managed --provider neon --yes
pilot add redis --yes
pilot add storage --managed --provider cloudflare-r2 --name assets --yes
```

---

## Mode interactif

Le wizard pose 4 questions dans l'ordre :

1. **Type** (sauté si fourni en argument)
2. **Hébergement** : `container`, `managed`, `local-only` (sauté si le service ne peut pas être managé)
3. **Fournisseur** (sauté si hébergement ≠ `managed`)
4. **Nom** (pré-rempli avec le type)
5. **Confirmation** (affiche les variables d'env qui seront ajoutées)

---

## Mode non-interactif (`--yes`)

Requiert que le type soit fourni en argument :

```bash
# ✓ Valide
pilot add redis --yes
pilot add postgres --managed --provider neon --yes

# ✗ Erreur : type manquant
pilot add --yes
```

En mode non-interactif sans `--managed`, l'hébergement est `container`.

---

## Ce que la commande modifie

### `pilot.yaml`

Ajoute une entrée sous `services:` :

```yaml
# Container
services:
  cache:
    type: redis
    hosting: container

# Managed
services:
  db:
    type: postgres
    hosting: managed
    provider: neon

# Local-only
services:
  db-dev:
    type: postgres
    hosting: local-only
```

### `.env.example`

Ajoute une section de variables pour le service. L'opération est idempotente : si la section existe déjà, elle n'est pas dupliquée.

---

## Fournisseurs disponibles par service

| Service | Fournisseurs managés |
|---------|---------------------|
| `postgres` | `neon`, `supabase`, `railway`, `render` |
| `mysql` | `planetscale`, `railway`, `render` |
| `mongodb` | `atlas`, `railway` |
| `redis` | `upstash`, `railway`, `render` |
| `storage` | `cloudflare-r2`, `aws-s3`, `supabase-storage` |
| `kafka` | `confluent`, `upstash-kafka` |

---

## Après `pilot add`

`pilot.yaml` a changé. Si un fichier compose avait été généré précédemment, pilot le
considère désormais comme potentiellement désynchronisé. Lors du prochain `pilot up` :

```
✗ docker-compose.dev.yml is stale : pilot.yaml has changed since it was generated
  Ask your AI agent: "Regenerate the compose file for the dev environment"
```

L'agent régénère le compose en tenant compte du nouveau service.

---

## Voir aussi

- [Guide : Ajouter un service](../guide/adding-services.md) : walkthrough complet avec exemples
- [Référence pilot.yaml : services](../reference/pilot-yaml.md#services) : champs hosting et provider
- [Staleness detection](../troubleshooting/stale-compose.md) : compose désynchronisé
