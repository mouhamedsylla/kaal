# kaal sync

Copie les fichiers de configuration locaux vers le VPS sans effectuer de redéploiement complet.

```
kaal sync [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--env`, `-e` | Environnement cible (défaut : env actif) |

## Ce qui est synchronisé

### Fichiers plats → `~/kaal/`

Ces fichiers sont copiés directement à la racine du répertoire de travail kaal sur le VPS :

| Fichier local | Destination VPS |
|---|---|
| `kaal.yaml` | `~/kaal/kaal.yaml` |
| `docker-compose.<env>.yml` | `~/kaal/docker-compose.<env>.yml` |
| Fichier env (`environments.<env>.env_file`) | `~/kaal/.env.<env>` |

### Fichiers bind-mount → `~/kaal/<chemin-relatif>/`

kaal analyse les fichiers `docker-compose` pour détecter tous les montages en bind-mount locaux (sources relatives) et les copie en préservant la structure de répertoires :

```yaml
# docker-compose.prod.yml
volumes:
  - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf
  - ./certs/fullchain.pem:/etc/ssl/certs/fullchain.pem
```

Résultat sur le VPS :
```
~/kaal/nginx/prod.conf
~/kaal/certs/fullchain.pem
```

## Rechargement automatique de nginx

Après la synchronisation, kaal détecte les services dont l'image contient `nginx` et dont les fichiers de configuration ont été modifiés. Si de tels services sont en cours d'exécution, kaal exécute automatiquement :

```
docker compose exec -T <service> nginx -s reload
```

Ce rechargement est **sans interruption** : les connexions actives sont préservées, aucun redémarrage de conteneur n'est nécessaire.

Si le conteneur nginx n'est pas encore démarré, kaal affiche un avertissement non-fatal :

```
⚠  nginx non démarré (proxy) : démarrer avec : kaal deploy --env prod
```

## Exemple de sortie

```
→ Syncing files to vps-prod (1.2.3.4)

  ✓  kaal.yaml
  ✓  docker-compose.prod.yml
  ✓  .env.prod
  ✓  nginx/prod.conf
  ✓  nginx reloaded (proxy)

✓ Sync complete
```

## kaal sync vs kaal push + kaal deploy

| Action | Commande recommandée |
|---|---|
| Modification de la config nginx | `kaal sync` |
| Mise à jour du fichier `.env` | `kaal sync` (puis `docker compose restart` si les vars sont lues au démarrage) |
| Mise à jour de `kaal.yaml` | `kaal sync` |
| Ajout d'un certificat TLS | `kaal sync` |
| Modification du code applicatif | `kaal push` + `kaal deploy` |
| Modification du `Dockerfile` | `kaal push` + `kaal deploy` |
| Mise à jour de `package.json` / `go.mod` | `kaal push` + `kaal deploy` |

`kaal sync` est conçu pour les changements de configuration qui n'impliquent pas de rebuilder l'image Docker. Il est rapide, atomique, et évite une interruption de service inutile.

## Notes

- La synchronisation utilise la connexion SSH définie dans `targets.<name>` de `kaal.yaml`
- Les fichiers sont transférés via SFTP (même connexion SSH, pas de `rsync` requis)
- Si un fichier bind-mount référencé dans le compose n'existe pas localement, kaal affiche une erreur et interrompt la synchronisation
