# kaal status

Affiche l'état de tous les services en cours d'exécution pour un environnement donné.

```
kaal status [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement à inspecter (défaut : env actif) |
| `--json` | Sortie JSON structurée |

## Comportement

**Environnement local** : exécute `docker compose ps` localement à partir du fichier `docker-compose.<env>.yml`.

**Environnement distant** (avec une cible `target` configurée) : se connecte au VPS via SSH et exécute `docker compose ps` depuis `~/kaal/`.

## Colonnes de sortie

| Colonne | Description |
|---------|-------------|
| `SERVICE` | Nom du service tel que défini dans le compose |
| `STATE` | État du conteneur : `running`, `exited`, `restarting` |
| `HEALTH` | Résultat du healthcheck : `healthy`, `unhealthy`, ou `-` |
| `PORTS` | Ports publiquement exposés (via `ports:`) |

**Colonne PORTS** : seuls les services utilisant `ports:` dans le compose affichent leurs ports. Les services qui n'utilisent que `expose:` (visibles uniquement sur le réseau Docker interne) affichent `-`.

**Colonne HEALTH** : affiche `-` pour les services sans directive `healthcheck` définie dans le compose ou le Dockerfile.

## Exemple de sortie

```
→ Status prod (vps-prod · 1.2.3.4)

  SERVICE    STATE      HEALTH     PORTS
  ─────────────────────────────────────────────────
  app        running    healthy    -
  proxy      running    :          0.0.0.0:80, 0.0.0.0:443
  db         running    healthy    -
  redis      running    :          -
  worker     exited     :          -
```

### Sortie JSON (`--json`)

```json
{
  "env": "prod",
  "target": "vps-prod",
  "host": "1.2.3.4",
  "services": [
    {
      "name": "app",
      "state": "running",
      "health": "healthy",
      "ports": []
    },
    {
      "name": "proxy",
      "state": "running",
      "health": null,
      "ports": ["0.0.0.0:80", "0.0.0.0:443"]
    },
    {
      "name": "worker",
      "state": "exited",
      "health": null,
      "ports": []
    }
  ]
}
```

## Interprétation des états

| STATE | HEALTH | Signification |
|-------|--------|---------------|
| `running` | `healthy` | Nominal : le service fonctionne et répond aux healthchecks |
| `running` | `-` | En cours d'exécution, mais sans healthcheck configuré : pas nécessairement un problème |
| `running` | `unhealthy` | Le conteneur tourne, mais le healthcheck échoue : investiguer avec `kaal logs <service>` |
| `exited` | `-` | Le conteneur s'est arrêté : vérifier les logs pour identifier la cause |
| `restarting` | `-` | Le conteneur redémarre en boucle : crash loop probable, voir `kaal logs <service>` |
