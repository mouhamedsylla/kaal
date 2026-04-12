# Concepts & Philosophie

## Le problème que résout pilot

Lorsque vous créez et lancez une application, trois mondes séparés doivent se mettre d'accord :

- **Vous** : vous savez ce dont votre application a besoin (une base de données, un cache, des limites de mémoire spécifiques, des variables secrètes).
- **Votre agent IA** : il écrit des Dockerfiles et des fichiers compose, mais uniquement s'il comprend exactement la structure de votre projet.
- **Votre environnement de production** : il exécute ce qui a été construit, avec ce qui a été configuré, au bon endroit.

Aujourd'hui, ces trois mondes s'éloignent constamment. Vous dites à l'IA "génère un Dockerfile pour mon application Go" et obtenez un modèle générique. Vous copiez-collez des variables d'environnement et en oubliez une. Vous construisez sur macOS ARM64 et déployez sur un VPS AMD64. Vous ajoutez un nouveau service localement et oubliez de synchroniser la configuration à distance.

**La réponse de pilot :** un unique `pilot.yaml` que les trois parties lisent. Vous l'écrivez une fois, tout le monde reste synchronisé.

---

## Le modèle central : pilot.yaml comme contrat partagé

```
pilot.yaml
    │
    ├── Vous l'écrivez      → décrit ce dont votre application a besoin
    │                         (services, environnements, cibles, registre)
    │
    ├── L'agent IA le lit   → comprend votre infrastructure exacte
    │                         génère le bon Dockerfile et les fichiers compose
    │                         sait quelles variables d'env sont liées à la compilation vs exécution
    │                         sait où déployer et avec quelles contraintes
    │
    └── pilot l'exécute      → le lance localement (docker compose)
                              le déploie à distance (SSH + pipeline 8 étapes)
                              gère les opérations fastidieuses automatiquement
```

Ce n'est pas seulement un fichier de configuration. C'est le **contrat** entre vous, vos outils et votre environnement de production. Tout le reste : Dockerfiles, fichiers compose, scripts CI : en découle.

---

## Principes

### 1. Décrire l'intention, pas l'implémentation

Vous déclarez **ce dont** votre application a besoin, pas **comment** la construire :

```yaml
services:
  app:
    type: app
    port: 8080
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
```

pilot (ou votre agent IA) génère l'implémentation. Si vous ajoutez un service, vous ajoutez une ligne. L'agent régénère le fichier compose. Aucune modification manuelle de YAML pour les volumes, les réseaux, ou les healthchecks.

### 2. Local = Production

Les environnements locaux simulent les contraintes de production :

```yaml
environments:
  dev:
    runtime: compose
    resources:
      cpus: "1"
      memory: 1G    # mêmes limites qu'en prod

  prod:
    target: vps-prod
    resources:
      cpus: "1"
      memory: 1G    # identiques
```

Si cela fonctionne dans ces contraintes localement, cela fonctionnera en production.

### 3. IA-native, pas IA-optionnelle

pilot est conçu pour fonctionner *avec* des agents IA, et non à côté d'eux après coup.

Lorsque `pilot up` trouve des fichiers manquants, il ne se contente pas d'échouer : il construit un prompt structuré avec le contexte complet de votre projet et indique à l'agent exactement ce qu'il faut générer et avec quelles contraintes (build multi-étapes, utilisateur non-root, healthchecks, gestion VITE_*, injection env_file...).

Lorsque `pilot preflight` s'exécute avant un déploiement, il retourne un plan d'action structuré en JSON que l'agent suit étape par étape : actions humaines d'abord, puis actions de l'agent.

Lorsque `pilot_context` est appelé via MCP, l'agent reçoit tout : la stack, les services, les fichiers existants, les fichiers manquants, les chemins env_file, les targets non configurés : une image complète, pas un résumé.

**L'agent ne devine pas. pilot lui dit exactement ce qui existe, ce qui manque et quoi faire.**

### 4. Zéro friction du local vers la prod

```bash
pilot up                  # local
pilot push                # build
pilot deploy --env prod   # prod
```

Trois commandes. Les mêmes depuis votre terminal, depuis la CI, depuis votre agent IA via MCP.

### 5. Automated ops, not documented ops

La plupart des outils documentent ce que vous devez faire manuellement. pilot le fait pour vous :

| Problème | Approche manuelle | pilot |
|---|---|---|
| Image ARM construite sur Mac, VPS en AMD64 | `--platform linux/amd64` à chaque build | Détecte Apple Silicon, build linux/amd64 par défaut |
| Variables Vite absentes en prod | Ajouter `ARG`/`ENV` dans le Dockerfile | Auto-injecte les `VITE_*`, patche le Dockerfile de façon transparente |
| Config nginx manquante sur le VPS | `scp nginx/prod.conf deploy@host:~/pilot/nginx/` | `pilot sync` scanne les bind-mounts du compose et les copie |
| `.env.prod` absent sur le VPS | `scp` manuel | `pilot sync` copie tous les `env_file` déclarés dans pilot.yaml |
| User deploy pas dans le groupe docker | SSH + `sudo usermod -aG docker deploy` | `pilot setup --env prod` |
| Mauvais répertoire de travail sur le VPS | Déboguer les erreurs compose | pilot utilise toujours `~/pilot/docker-compose.<env>.yml` |

---

## Le système preflight et pilot.lock

Avant de déployer, `pilot preflight --target deploy` fait deux choses :

**1. Vérifie** tous les prérequis (Docker, SSH, registry, clés, fichiers compose...)

```
✓ pilot_yaml            project: my-api
✓ dockerfile            Dockerfile
✓ registry_creds        GITHUB_TOKEN + GITHUB_ACTOR présents
✓ vps_connectivity      SSH OK (1.2.3.4:22)
✓ All checks passed : pilot.lock generated
```

**2. Génère `pilot.lock`** : le contrat signé du prochain déploiement :

```yaml
# pilot.lock : generated automatically, commit this file.
execution_plan:
  nodes_active: [preflight, migrations, deploy, post_hooks, healthcheck]
  migrations:
    tool: prisma
    command: npx prisma migrate deploy
    rollback_command: npx prisma migrate rollback
    reversible: true
project_hash: "abc123..."
```

`pilot.lock` doit être commité. `pilot deploy` vérifie que les sources n'ont pas changé depuis sa génération. Ce qui a été validé par l'équipe est ce qui s'exécute en production : jamais une inférence du moment.

---

## Le modèle de résilience

pilot ne laisse jamais l'utilisateur ni l'agent dans un état ambigu. Chaque erreur appartient à l'un des quatre types :

| Type | Situation | Qui agit | Comment |
|------|-----------|----------|---------|
| **A** | Déterministe, faible impact | pilot, silencieusement | auto-corrige, continue |
| **B** | Déterministe, impact visible | pilot, annoncé | auto-corrige, affiche ce qu'il a fait |
| **C** | Choix requis, options connues | humain ou agent | suspend, présente les options, attend |
| **D** | Choix requis, options inconnues | humain uniquement | arrête avec des instructions précises |

Quand une erreur TypeC survient (ex: user pas dans le groupe docker), pilot suspend l'opération et présente des choix numérotés. `pilot resume --answer 0` reprend depuis là où ça s'est arrêté.

Pour les agents IA, la même information arrive en JSON structuré avec `resume_with` pour reprendre automatiquement.

---

## Le workflow agent AI de déploiement

Dans un projet avec Claude Code et `.mcp.json` :

```
Vous :  "Les tests passent, déploie en prod"

Agent:  pilot_preflight → all_ok: false
          [HUMAN] registry_creds: export GITHUB_TOKEN=...
        → attend votre confirmation

Vous :  (configurez le token)

Agent:  pilot_preflight → all_ok: true : pilot.lock generated
        pilot_push → image built linux/amd64, pushed
        pilot_deploy → pipeline 8 étapes (hooks, migrations, deploy, healthcheck)
        pilot_status → tous les services healthy

Vous :  ✓ Terminé. Aucun terminal ouvert.
```

L'agent connaît votre infrastructure via `pilot.yaml`. Il sait ce qui est déployé via `pilot_status`. Il sait ce qui est cassé via `pilot_preflight`. Il sait comment réparer via `pilot_setup`, `pilot_sync`, et les outils de génération. Vous restez dans le chat.

---

## Ce que pilot n'est pas

- **Pas un wrapper autour de Docker** : pilot supporte compose, k3d (Kubernetes local), Lima (VMs légères), et les cloud providers. Docker compose est l'implémentation par défaut.
- **Pas un outil de CI** : pilot fournit les primitives (`push`, `deploy`, `rollback`). GitHub Actions ou GitLab CI les séquencent.
- **Pas un moteur de templates** : les Dockerfiles et compose files sont générés par votre agent IA qui lit votre projet réel : pas par des templates statiques.
- **Pas Terraform** : pilot gère votre application et ses dépendances directes. Le provisioning d'infrastructure (VMs, VPCs, bases de données managées) reste le travail de Terraform ou Pulumi.
- **Pas opinioné sur votre stack** : Go, Node, Python, Rust, Java. VPS, AWS, GCP. GHCR, Docker Hub, registry custom. pilot abstrait les différences via des interfaces provider.

---

## Cycle de vie du projet

```
pilot init my-app
    └─► pilot.yaml + .mcp.json créés
        Wizard : services, environnements, VPS host, registry

─── premier démarrage ───

pilot up
    └─► Fichiers manquants détectés
        L'agent reçoit le contexte complet via pilot_context (MCP)
        ou pilot context (paste dans n'importe quel AI chat)
        L'agent génère : Dockerfile + docker-compose.dev.yml
        Fichiers écrits à la racine du projet

pilot up
    └─► docker compose up -d
        Services en cours localement

─── cycle de développement ───

pilot preflight --target deploy --env prod
    └─► Vérifie 13 prérequis
        Génère pilot.lock → commiter

pilot push --env prod
    └─► Vars VITE_* auto-détectées depuis .env.prod
        Image linux/amd64 construite
        Image poussée vers le registry

pilot plan --env prod
    └─► Affiche le pipeline exact (sans exécuter)

pilot deploy --env prod
    └─► lock check → secrets → sync → hooks → migrations
        → deploy → hooks → healthcheck
        ✓ Déployé

─── quelque chose casse ───

pilot rollback --env prod
    └─► Lit prev-tag depuis l'état VPS
        Redémarre avec l'image précédente

─── changement d'infrastructure ───

Modifier pilot.yaml (ajout service, changement port)
    └─► L'agent appelle pilot_context
        Régénère les fichiers compose
        pilot preflight (nouveau pilot.lock)
        pilot sync + pilot deploy
        VPS mis à jour
```
