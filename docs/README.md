# pilot : Documentation

> Dev Environment as Code : de l'init au déploiement, sans friction.

## Navigation

### Comprendre pilot
- [Concepts & Philosophie](concepts.md) : Pourquoi pilot existe, le modèle mental, la valeur ajoutée
- [Architecture du code](architecture.md) : Packages, interfaces, hexagonal architecture, pipeline de déploiement
- [Modèle de résilience](resilience.md) : Taxonomie TypeA/B/C/D, machine à états, pilot.lock
- [pilot.yaml : Référence complète](pilot-yaml.md) : Schéma, champs, exemples

### Commandes
- [pilot up / down](commands/up-down.md) : Démarrer / arrêter l'environnement local
- [pilot env](commands/env.md) : Gérer les environnements (use, current, diff)
- [pilot context](commands/context.md) : Exporter le contexte pour un agent AI
- [pilot push](commands/push.md) : Build + push de l'image Docker (auto-inject VITE_*, platform detection)
- [pilot preflight](commands/preflight.md) : Vérifier les prérequis + générer pilot.lock
- [pilot plan](commands/plan.md) : Afficher le plan d'exécution sans déployer
- [pilot deploy](commands/deploy.md) : Déployer sur une cible distante (pipeline 8 étapes, hooks, migrations, LIFO compensation)
- [pilot sync](commands/sync.md) : Synchroniser les fichiers de config vers le VPS sans redéployer
- [pilot rollback](commands/rollback.md) : Revenir à la version précédente
- [pilot resume](commands/resume.md) : Reprendre une opération suspendue (TypeC)
- [pilot diagnose](commands/diagnose.md) : Snapshot complet du système (Docker, SSH, ports, git)
- [pilot status](commands/status.md) : État complet du projet et des services
- [pilot logs](commands/logs.md) : Consulter les logs d'un service
- [pilot setup](commands/setup.md) : Préparer un VPS pour le déploiement
- [pilot history](commands/history.md) : Historique des déploiements
- [pilot mcp](commands/mcp.md) : Démarrer le serveur MCP (JSON-RPC 2.0 stdio)

### Workflows complets
- [Développement local](workflows/local-dev.md) : De `pilot init` à `pilot up`
- [Intégration Agent AI](workflows/ai-agent.md) : Générer l'infra et déployer avec Claude/Cursor
- [Déploiement VPS](workflows/deploy-vps.md) : Preflight, push, plan, deploy, rollback
- [CI/CD](workflows/ci-cd.md) : pilot dans GitHub Actions / GitLab CI

### Troubleshooting
- [Variables d'environnement vides dans le container](troubleshooting/env-vars-empty-in-container.md) : ARG/ENV pattern, priorité des vars, fixes et protections intégrées

### Internals (pour contribuer)
- [Moteur de contexte](internals/context-engine.md) : Comment `internal/mcp/context` collecte les informations
- [ExecutionProvider](internals/orchestrators.md) : compose, k8s — runtime local
- [DeployProvider](internals/providers.md) : VPS/SSH (interface, Sync, hooks, migrations, erreurs TypeC/D)
- [Serveur MCP](internals/mcp-server.md) : Protocole JSON-RPC 2.0, outils implémentés
