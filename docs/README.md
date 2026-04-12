# pilot : Documentation

> Dev Environment as Code : de l'init au déploiement, sans friction.

---

## Guides pratiques

Commence ici si tu découvres pilot.

- [Démarrer avec pilot](guide/getting-started.md) : wizard d'init, génération des fichiers d'infra, premier `pilot up`
- [Ajouter un service](guide/adding-services.md) : `pilot add` pour étendre l'infra après l'init (Redis, Neon, Cloudflare R2…)
- [Premier déploiement sur VPS](workflows/deploy-vps.md) : preflight, push, deploy, rollback

---

## Comprendre pilot

- [Concepts & Philosophie](concepts.md) : Pourquoi pilot existe, le modèle mental, la valeur ajoutée
- [Architecture du code](architecture.md) : Packages, interfaces, hexagonal architecture, pipeline de déploiement
- [Modèle de résilience](resilience.md) : Taxonomie TypeA/B/C/D, machine à états, pilot.lock

---

## Référence

- [pilot.yaml : Référence complète](reference/pilot-yaml.md) : Tous les champs : project, registry, services (hosting/provider/catalogue), environments, targets
- [Workflows complets](#workflows)
- [Toutes les commandes](#commandes)
- [Outils MCP](commands/mcp.md) : Liste des outils et protocole JSON-RPC

---

## Workflows complets

- [Développement local](workflows/local-dev.md) : De `pilot init` à `pilot up`
- [Intégration Agent IA](workflows/ai-agent.md) : Générer l'infra et déployer avec Claude/Cursor
- [Déploiement VPS](workflows/deploy-vps.md) : Preflight, push, plan, deploy, rollback
- [CI/CD](workflows/ci-cd.md) : pilot dans GitHub Actions / GitLab CI

---

## Commandes

### Infra locale

| Commande | Description |
|----------|-------------|
| [`pilot init`](commands/init.md) | Initialiser pilot dans un projet nouveau ou existant |
| [`pilot add`](commands/add.md) | Ajouter un service à un projet existant |
| [`pilot up` / `pilot down`](commands/up-down.md) | Démarrer / arrêter l'environnement local |
| [`pilot env`](commands/env.md) | Gérer les environnements (use, current, diff) |

### Déploiement

| Commande | Description |
|----------|-------------|
| [`pilot push`](commands/push.md) | Build + push de l'image Docker vers le registry |
| [`pilot deploy`](commands/deploy.md) | Déployer sur une cible distante (pipeline 8 étapes) |
| [`pilot rollback`](commands/rollback.md) | Revenir à la version précédente |
| [`pilot sync`](commands/sync.md) | Synchroniser les fichiers de config vers le VPS |
| [`pilot preflight`](commands/preflight.md) | Vérifier les prérequis + générer pilot.lock |
| [`pilot plan`](commands/plan.md) | Afficher le plan d'exécution sans déployer |
| [`pilot setup`](commands/setup.md) | Préparer un VPS vierge pour le déploiement |

### Observation

| Commande | Description |
|----------|-------------|
| [`pilot status`](commands/status.md) | État complet du projet et des services |
| [`pilot logs`](commands/logs.md) | Logs d'un service (local ou distant) |
| [`pilot diagnose`](commands/diagnose.md) | Snapshot complet du système |
| [`pilot history`](commands/history.md) | Historique des déploiements |

### Secrets

| Commande | Description |
|----------|-------------|
| [`pilot secrets`](commands/secrets.md) | Gérer les secrets (list, get, set, inject) |

### Agent IA & MCP

| Commande | Description |
|----------|-------------|
| [`pilot mcp serve`](commands/mcp.md) | Démarrer le serveur MCP (JSON-RPC 2.0 stdio) |
| [`pilot mcp context`](commands/context.md) | Afficher le contexte complet pour un agent AI |

### Divers

| Commande | Description |
|----------|-------------|
| [`pilot resume`](commands/resume.md) | Reprendre une opération suspendue (TypeC) |
| [`pilot update`](commands/update.md) | Mettre à jour pilot vers la dernière version |

---

## Troubleshooting

- [Variables d'environnement vides dans le container](troubleshooting/env-vars-empty-in-container.md) : ARG/ENV pattern, priorité des vars, fixes intégrés
- [Compose désynchronisé après `pilot add`](troubleshooting/stale-compose.md) : Staleness detection, comment régénérer

---

## Internals (pour contribuer)

- [Moteur de contexte](internals/context-engine.md) : `internal/mcp/context` : collecte du contexte, AgentPrompt, MissingComposeEnvs
- [ExecutionProvider](internals/orchestrators.md) : compose, k8s : runtime local
- [DeployProvider](internals/providers.md) : VPS/SSH (interface, Sync, hooks, migrations, erreurs TypeC/D)
- [Serveur MCP](internals/mcp-server.md) : Protocole JSON-RPC 2.0, outils implémentés
