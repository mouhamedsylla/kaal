# pilot sync

Copie les fichiers de configuration locaux vers le VPS sans effectuer de redéploiement complet.

```
pilot sync [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement cible (défaut : env actif) |

## Ce qui est synchronisé

### Fichiers plats → `~/pilot/`

Ces fichiers sont copiés directement à la racine du répertoire de travail pilot sur le VPS :

| Fichier local | Destination VPS |
|---|---|
| `pilot.yaml` | `~/pilot/pilot.yaml` |
| `docker-compose.<env>.yml` | `~/pilot/docker-compose.<env>.yml` |
| Fichier env (`environments.<env>.env_file`) | `~/pilot/.env.<env>` |

### Fichiers bind-mount → `~/pilot/<chemin-relatif>/`

pilot analyse les fichiers `docker-compose` pour détecter tous les montages en bind-mount locaux (sources relatives) et les copie en préservant la structure de répertoires :

```yaml
# docker-compose.prod.yml
volumes:
  - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf
  - ./certs/fullchain.pem:/etc/ssl/certs/fullchain.pem
```

Résultat sur le VPS :
```
~/pilot/nginx/prod.conf
~/pilot/certs/fullchain.pem
```

## Rechargement automatique de nginx

Après la synchronisation, pilot détecte les services dont l'image contient `nginx` et dont les fichiers de configuration ont été modifiés. Si de tels services sont en cours d'exécution, pilot exécute automatiquement :

```
docker compose exec -T <service> nginx -s reload
```

Ce rechargement est **sans interruption** : les connexions actives sont préservées, aucun redémarrage de conteneur n'est nécessaire.

Si le conteneur nginx n'est pas encore démarré, pilot affiche un avertissement non-fatal :

```
⚠  nginx non démarré (proxy) : démarrer avec : pilot deploy --env prod
```

## Exemple de sortie

```
→ Syncing files to vps-prod (1.2.3.4)

  ✓  pilot.yaml
  ✓  docker-compose.prod.yml
  ✓  .env.prod
  ✓  nginx/prod.conf
  ✓  nginx reloaded (proxy)

✓ Sync complete
```

## pilot sync vs pilot push + pilot deploy

| Action | Commande recommandée |
|---|---|
| Modification de la config nginx | `pilot sync` |
| Mise à jour du fichier `.env` | `pilot sync` (puis `docker compose restart` si les vars sont lues au démarrage) |
| Mise à jour de `pilot.yaml` | `pilot sync` |
| Ajout d'un certificat TLS | `pilot sync` |
| Modification du code applicatif | `pilot push` + `pilot deploy` |
| Modification du `Dockerfile` | `pilot push` + `pilot deploy` |
| Mise à jour de `package.json` / `go.mod` | `pilot push` + `pilot deploy` |

`pilot sync` est conçu pour les changements de configuration qui n'impliquent pas de rebuilder l'image Docker. Il est rapide, atomique, et évite une interruption de service inutile.

## Notes

- La synchronisation utilise la connexion SSH définie dans `targets.<name>` de `pilot.yaml`
- Les fichiers sont transférés via SFTP (même connexion SSH, pas de `rsync` requis)
- Si un fichier bind-mount référencé dans le compose n'existe pas localement, pilot affiche une erreur et interrompt la synchronisation
