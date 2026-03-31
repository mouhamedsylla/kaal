# kaal setup

Prépare un VPS vierge pour les déploiements kaal.

```
kaal setup [flags]
```

## Flags

| Flag | Description |
|------|-------------|
| `--fix-docker` | Ajoute l'utilisateur deploy au groupe `docker` (défaut : `true`) |
| `--env`, `-e` | Environnement dont la cible VPS sera configurée (défaut : env actif) |

## Ce que fait kaal setup

kaal se connecte au VPS via SSH et exécute les deux actions suivantes :

1. **Ajout au groupe docker** : ajoute l'utilisateur défini dans `targets.<name>.user` au groupe `docker` :
   ```
   sudo usermod -aG docker <user>
   ```

2. **Création du répertoire de travail** : crée `~/kaal/` sur le VPS si ce répertoire n'existe pas :
   ```
   mkdir -p ~/kaal
   ```

## Pourquoi c'est nécessaire

Par défaut, les commandes Docker requièrent les privilèges root ou l'appartenance au groupe `docker`. Sans cette configuration, `kaal deploy` échoue avec :

```
permission denied while trying to connect to the Docker daemon socket
```

kaal ne déploie jamais en tant que root. Il utilise l'utilisateur déclaré dans `kaal.yaml` (`target.user`) et requiert que cet utilisateur appartienne au groupe `docker`.

## Quand l'exécuter

- **Une seule fois par VPS**, après le premier accès SSH
- Habituellement suivi de `kaal preflight --target deploy` pour vérifier que tout est en ordre

kaal preflight détecte automatiquement si setup est nécessaire via la vérification `vps_docker_group`.

## Exemple de sortie

```
→ Setting up vps-prod (azureuser@1.2.3.4)

  ✓  azureuser ajouté au groupe docker
  ✓  ~/kaal/ créé

✓ Setup complete
→ Reconnectez-vous en SSH pour que les changements de groupe prennent effet
→ Ensuite : kaal preflight --target deploy --env prod
```

## Note importante : reconnexion SSH

L'appartenance à un groupe Unix n'est prise en compte qu'à l'ouverture d'une nouvelle session. Après `kaal setup`, la session SSH en cours ne bénéficie pas encore du groupe `docker`. kaal le signale explicitement.

Pour les déploiements automatisés (CI/CD), cette contrainte ne s'applique pas car chaque connexion SSH ouvre une nouvelle session.

## Prérequis

- Le VPS doit être accessible via SSH avec les paramètres définis dans `kaal.yaml` (`targets.<name>`)
- L'utilisateur doit avoir les droits `sudo` pour exécuter `usermod`
- Docker doit être préinstallé sur le VPS (kaal setup n'installe pas Docker)
