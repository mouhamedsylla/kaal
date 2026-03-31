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
                              le déploie à distance (SSH + docker compose)
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

Lorsque `pilot up` trouve des fichiers manquants, il ne se contente pas d'échouer : il construit un prompt structuré avec le contexte complet de votre projet et indique à l'agent exactement ce qu'il faut générer et avec quelles contraintes (build multi-étapes, utilisateur non-root, healthchecks, gestion VITE_*, injection env_file…).

Lorsque `pilot preflight` s'exécute avant un déploiement, il renvoie un plan d'action structuré en JSON que l'agent suit étape par étape : actions humaines d'abord, puis actions de l'agent, puis l'objectif de déploiement.

Lorsque `pilot_context` est appelé via MCP, l'agent reçoit tout : la stack, les services, les fichiers existants, les fichiers manquants, les chemins env_file, les cibles non configurées : une image complète, pas un résumé.

**L'agent ne devine pas. pilot lui dit exactement ce qui existe, ce qui manque et quoi faire.**

### 4. Zéro friction du local vers la prod

```bash
pilot up              # local
pilot push            # build
pilot deploy --env prod   # prod
```

Trois commandes. Les mêmes commandes depuis votre terminal, depuis la CI, depuis votre agent IA via MCP.

### 5. Automated ops, not documented ops

Most tools document what you need to do manually. pilot does it for you:

| Problem | Manual approach | pilot |
|---|---|---|
| Built ARM image on Mac, VPS is AMD64 | Add `--platform linux/amd64` to every build | Auto-detects Apple Silicon, builds linux/amd64 by default |
| Vite vars not showing in prod | Manually add `ARG`/`ENV` to Dockerfile | Auto-injects `VITE_*` vars, patches Dockerfile transparently |
| nginx config missing on VPS | `scp nginx/prod.conf deploy@host:~/pilot/nginx/` | `pilot sync` scans bind-mounts in compose file and copies them |
| `.env.prod` not on VPS | Manual `scp` | `pilot sync` copies all `env_file` declared in pilot.yaml |
| Deploy user not in docker group | SSH + `sudo usermod -aG docker deploy` | `pilot setup --env prod` |
| Wrong working dir on VPS | Debug compose errors | pilot always uses `~/pilot/docker-compose.<env>.yml` |

---

## The preflight system

Before pushing or deploying, `pilot preflight` runs a structured checklist and returns an action plan:

```
pilot preflight --target deploy

✓ pilot_yaml
✓ registry_image
✓ dockerfile
✓ docker_daemon
✓ registry_creds
✓ compose_file
✓ target_host
✓ ssh_key
✓ vps_connectivity
✓ vps_docker_group
✓ vps_env_file
✓ All checks passed : ready to deploy
```

When something is wrong, the report tells exactly who needs to act:
- `[HUMAN]` : you must do this (provide credentials, add SSH key, open firewall port)
- `[AGENT]` : the AI agent can call a pilot tool to fix this automatically

The agent calls `pilot_preflight` first, follows `next_steps[]` in order, and only asks you when human action is genuinely required : not for things it can handle itself.

---

## The AI-agent deploy workflow

In a project with Claude Code and `.mcp.json`:

```
You:    "Les tests passent, déploie en prod"

Agent:  pilot_preflight → all_ok: false
          [HUMAN] registry_creds: export GITHUB_TOKEN=...
        → waits for you

You:    (set the token)

Agent:  pilot_preflight → all_ok: true
        pilot_push → image built and pushed
        pilot_deploy → synced + deployed
        pilot_status → reports service health

You:    ✓ Done. No terminal opened.
```

The agent knows your infrastructure through `pilot.yaml`. It knows what's deployed through `pilot_status`. It knows what's broken through `pilot_preflight`. It knows how to fix things through `pilot_setup`, `pilot_sync`, and the generate tools. You stay in the chat.

---

## What pilot is not

- **Not a wrapper around Docker** : pilot supports compose, k3d (local Kubernetes), Lima (lightweight VMs), and cloud providers. Docker compose is the default implementation.
- **Not a CI tool** : pilot provides the primitives (`push`, `deploy`, `rollback`). GitHub Actions or GitLab CI sequence them.
- **Not a template engine** : Dockerfiles and compose files are generated by your AI agent reading your actual project, not by static templates.
- **Not Terraform** : pilot manages your application and its direct dependencies. Infrastructure provisioning (VMs, VPCs, managed databases) is Terraform or Pulumi's job.
- **Not opinionated about your stack** : Go, Node, Python, Rust, Java. VPS, AWS, GCP. GHCR, Docker Hub, custom registry. pilot abstracts the differences through provider interfaces.

---

## Project lifecycle

```
pilot init my-app
    └─► pilot.yaml + .mcp.json created
        Wizard: services, environments, VPS host, registry

─── first run ───

pilot up
    └─► Missing files detected
        Agent receives full context via pilot_context (MCP)
        or pilot context (paste into any AI chat)
        Agent generates: Dockerfile + docker-compose.dev.yml
        Files written to project root

pilot up
    └─► docker compose up
        Services running locally

─── development cycle ───

pilot push
    └─► VITE_* vars auto-detected from .env.dev
        linux/amd64 image built
        Image pushed to registry

pilot deploy --env prod
    └─► pilot sync: compose + env + nginx/prod.conf + ...
        docker pull on VPS
        docker compose up -d
        ✓ Deployed

─── something breaks ───

pilot rollback --env prod
    └─► Reads prev-tag from VPS state
        Restarts with previous image

─── infrastructure change ───

Edit pilot.yaml (add a service, change a port)
    └─► Agent calls pilot_context
        Regenerates docker-compose files
        pilot sync + pilot deploy
        VPS updated
```
