# kaal — Documentation

> Dev Environment as Code — de l'init au déploiement, sans friction.

## Navigation

### Comprendre kaal
- [Concepts & Philosophie](concepts.md) — Pourquoi kaal existe, le modèle mental
- [Architecture du code](architecture.md) — Packages, interfaces, patterns Go
- [kaal.yaml — Référence complète](kaal-yaml.md) — Schéma, champs, exemples

### Commandes
- [kaal init](commands/init.md) — Initialiser un projet
- [kaal env](commands/env.md) — Gérer les environnements actifs
- [kaal up / down](commands/up-down.md) — Démarrer / arrêter l'environnement local
- [kaal context](commands/context.md) — Exporter le contexte pour un agent AI
- [kaal push](commands/push.md) — Build + push de l'image Docker
- [kaal deploy](commands/deploy.md) — Déployer sur une cible distante
- [kaal rollback](commands/rollback.md) — Revenir à une version précédente
- [kaal status](commands/status.md) — État des services local et distant
- [kaal logs](commands/logs.md) — Logs en streaming
- [kaal mcp serve](commands/mcp.md) — Serveur MCP pour agents AI

### Workflows complets
- [Développement local](workflows/local-dev.md) — De `kaal init` à `kaal up`
- [Intégration AI agent](workflows/ai-agent.md) — Générer l'infra avec Claude/Cursor
- [CI/CD](workflows/ci-cd.md) — kaal dans GitHub Actions / GitLab CI
- [Déploiement VPS](workflows/deploy-vps.md) — Push + deploy sur un VPS

### Internals (pour contribuer)
- [Moteur de contexte](internals/context-engine.md) — Comment `internal/context` collecte les informations
- [Orchestrateurs](internals/orchestrators.md) — compose, k3d, k8s
- [Providers](internals/providers.md) — VPS/SSH, AWS, GCP...
- [Serveur MCP](internals/mcp-server.md) — Protocole JSON-RPC 2.0, outils exposés
