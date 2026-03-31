# kaal : Documentation

> Dev Environment as Code : de l'init au déploiement, sans friction.

## Navigation

### Comprendre kaal
- [Concepts & Philosophie](concepts.md) : Pourquoi kaal existe, le modèle mental, la valeur ajoutée
- [Architecture du code](architecture.md) : Packages, interfaces, patterns Go
- [kaal.yaml : Référence complète](kaal-yaml.md) : Schéma, champs, exemples

### Commandes
- [kaal up / down](commands/up-down.md) : Démarrer / arrêter l'environnement local
- [kaal context](commands/context.md) : Exporter le contexte pour un agent AI
- [kaal push](commands/push.md) : Build + push de l'image Docker (auto-inject VITE_*, platform detection)
- [kaal deploy](commands/deploy.md) : Déployer sur une cible distante (sync automatique, rollback auto)
- [kaal sync](commands/sync.md) : Synchroniser les fichiers de config vers le VPS sans redéployer
- [kaal rollback](commands/rollback.md) : Revenir à la version précédente
- [kaal status](commands/status.md) : État complet du projet et des services
- [kaal logs](commands/logs.md) : Consulter les logs d'un service
- [kaal env](commands/env.md) : Gérer les environnements (switch, liste)
- [kaal setup](commands/setup.md) : Préparer un VPS pour le déploiement
- [kaal history](commands/history.md) : Historique des déploiements
- [kaal mcp](commands/mcp.md) : Démarrer le serveur MCP (JSON-RPC 2.0 stdio)
- [kaal preflight](commands/preflight.md) : Vérifier tous les prérequis avant de déployer

### Workflows complets
- [Développement local](workflows/local-dev.md) : De `kaal init` à `kaal up`
- [Intégration Agent AI](workflows/ai-agent.md) : Générer l'infra et déployer avec Claude/Cursor
- [Déploiement VPS](workflows/deploy-vps.md) : Preflight, push, deploy, rollback
- [CI/CD](workflows/ci-cd.md) : kaal dans GitHub Actions / GitLab CI

### Troubleshooting
- [Variables d'environnement vides dans le container](troubleshooting/env-vars-empty-in-container.md) : ARG/ENV pattern, priorité des vars, fixes et protections intégrées

### Internals (pour contribuer)
- [Moteur de contexte](internals/context-engine.md) : Comment `internal/context` collecte les informations
- [Orchestrateurs](internals/orchestrators.md) : compose, k3d, k8s
- [Providers](internals/providers.md) : VPS/SSH (interface, Sync, CopyFileTo, bind-mounts)
- [Serveur MCP](internals/mcp-server.md) : Protocole JSON-RPC 2.0, outils implémentés
