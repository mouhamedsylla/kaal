# pilot secrets

Gère les secrets d'un environnement pilot.

```
pilot secrets <sous-commande> [flags]
```

---

## Sous-commandes

| Sous-commande | Description |
|---------------|-------------|
| `pilot secrets list` | Liste les clés du fichier `.env.<env>` actif |
| `pilot secrets get KEY` | Lit la valeur d'une clé |
| `pilot secrets set KEY VALUE` | Écrit ou met à jour une valeur |
| `pilot secrets inject` | Résout et affiche tous les secrets configurés dans `pilot.yaml` |

---

## `pilot secrets list`

Liste les clés déclarées dans le fichier `.env.<env>` de l'environnement actif.
Croise avec les `refs` déclarés dans `pilot.yaml` pour indiquer les secrets mappés.

```bash
pilot secrets list
pilot secrets list --env prod
```

```
Secrets for environment "dev" (.env.dev)

  DATABASE_URL                    ← DATABASE_URL
  SECRET_KEY
  REDIS_URL
  VITE_API_URL
```

---

## `pilot secrets get KEY`

Lit et affiche la valeur d'une clé dans `.env.<env>`.

```bash
pilot secrets get DATABASE_URL
pilot secrets get DATABASE_URL --env prod
```

---

## `pilot secrets set KEY VALUE`

Écrit ou met à jour une clé dans `.env.<env>`. Crée le fichier s'il n'existe pas.

```bash
pilot secrets set DATABASE_URL "postgresql://user:pass@host:5432/db"
pilot secrets set SECRET_KEY "mon-secret-local" --env dev
```

> Pour les valeurs avec espaces ou caractères spéciaux, utilise des guillemets.

---

## `pilot secrets inject`

Résout **tous** les secrets déclarés dans `pilot.yaml` pour l'environnement actif,
en utilisant le provider configuré (`local`, `aws_sm`, `gcp_sm`).

Affiche les clés résolues (valeurs masquées par défaut).

```bash
pilot secrets inject
pilot secrets inject --show-values   # affiche les valeurs : attention
pilot secrets inject --env prod
```

```
Resolved secrets : env: prod, provider: aws_sm

  DATABASE_URL=<redacted>
  SECRET_KEY=<redacted>

  2 secret(s) resolved
```

### Flags de `inject`

| Flag | Description |
|------|-------------|
| `--show-values` | Affiche les valeurs en clair (à utiliser avec précaution) |

---

## Flags globaux

| Flag | Alias | Description |
|------|-------|-------------|
| `--env` | `-e` | Environnement cible (surcharge `.pilot-current-env`) |

---

## Providers de secrets

La commande `inject` utilise le provider configuré dans `pilot.yaml` :

```yaml
environments:
  prod:
    secrets:
      provider: aws_sm         # local | aws_sm | gcp_sm
      refs:
        DATABASE_URL: prod/myapp/database-url
        SECRET_KEY: prod/myapp/secret-key
```

| Provider | Backend |
|----------|---------|
| `local` | Lit depuis le fichier `env_file` déclaré dans pilot.yaml |
| `aws_sm` | AWS Secrets Manager (credentials AWS standard) |
| `gcp_sm` | GCP Secret Manager |

---

## Voir aussi

- [Référence pilot.yaml : environments.secrets](../reference/pilot-yaml.md#environments) : configuration des providers
- [Déploiement VPS](../workflows/deploy-vps.md) : injection des secrets lors du déploiement
