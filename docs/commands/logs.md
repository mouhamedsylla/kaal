# pilot logs

Affiche ou diffuse les logs d'un ou plusieurs services.

```
pilot logs [service] [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--follow`, `-f` | Diffuse les logs en temps réel (Ctrl+C pour arrêter) |
| `--since` | Affiche les logs depuis une durée ou un instant donné (ex: `5m`, `1h`, `2006-01-02T15:04:05`) |
| `--lines`, `-n` | Nombre de lignes à afficher (défaut : `100`) |
| `--env`, `-e` | Environnement cible (défaut : env actif) |

## Comportement

**Environnement local** : encapsule `docker compose logs` localement.

**Environnement distant** (avec `target` configuré) : se connecte au VPS via SSH et exécute `docker compose logs` depuis `~/pilot/`.

## Argument `service`

L'argument `service` est optionnel :

- **Omis** : affiche les logs de tous les services, entrelacés et préfixés par le nom du service
- **Spécifié** : affiche uniquement les logs du service indiqué

Le nom du service correspond à celui défini dans le fichier `docker-compose.<env>.yml`.

## Exemples

```bash
# Dernières 100 lignes de tous les services (env actif)
pilot logs

# Diffusion en temps réel du service "app"
pilot logs app --follow

# Dernières 10 minutes de logs du service "proxy"
pilot logs proxy --since 10m

# 500 dernières lignes du service "worker"
pilot logs worker --lines 500

# Logs de production depuis le VPS
pilot logs --env prod

# Diffusion en temps réel de "app" en production
pilot logs app --follow --env prod

# Logs depuis un instant précis
pilot logs app --since 2006-01-02T15:04:05
```

## Exemples de sortie

```
→ Logs app (env: prod · vps-prod)

app    | 2026-03-31T10:23:14Z INFO  Server started on :8080
app    | 2026-03-31T10:23:15Z INFO  Connected to database
app    | 2026-03-31T10:24:01Z INFO  GET /api/health 200 1.2ms
app    | 2026-03-31T10:24:33Z ERROR Failed to process job: context deadline exceeded
```

Avec plusieurs services (sans argument) :

```
→ Logs all services (env: prod · vps-prod)

proxy  | 2026-03-31T10:24:01Z 172.18.0.3 - GET /api/health HTTP/1.1 200
app    | 2026-03-31T10:24:01Z INFO  GET /api/health 200 1.2ms
db     | 2026-03-31T10:24:05Z LOG  checkpoint complete: wrote 3 buffers
```

## Notes

- `--follow` et `--since` peuvent être combinés : `pilot logs app -f --since 5m` diffuse depuis 5 minutes en arrière
- Pour les environnements distants, le flux SSH est maintenu ouvert pendant toute la durée de `--follow`
- En mode MCP, les logs ne sont pas diffusés sur stdout afin de ne pas corrompre le pipe JSON-RPC : ils sont retournés comme chaîne de caractères dans la réponse de l'outil
