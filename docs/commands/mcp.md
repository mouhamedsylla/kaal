# kaal mcp

Démarre le serveur MCP (Model Context Protocol) pour l'intégration avec les agents IA.

```
kaal mcp serve
```

## Transport

JSON-RPC 2.0 sur stdin/stdout : pas de port réseau, pas de processus séparé. kaal lit les requêtes depuis stdin et écrit les réponses sur stdout. L'éditeur ou l'agent IA gère le cycle de vie du processus.

## Configuration

Créer un fichier `.mcp.json` à la racine du projet :

```json
{
  "mcpServers": {
    "kaal": {
      "command": "kaal",
      "args": ["mcp", "serve"],
      "cwd": "${workspaceFolder}"
    }
  }
}
```

Cette configuration est compatible avec Claude Desktop, Claude Code, et tout client MCP standard.

## Important : stdout et mode MCP

kaal détecte si stdout est un terminal réel (`ui.IsTerminal()`). En mode MCP, stdout est un pipe JSON-RPC : **diffuser la sortie Docker ou SSH sur stdout corromprait le protocole**.

En mode MCP, kaal :
- Ne diffuse jamais la sortie des commandes Docker ou SSH sur stdout
- Retourne les résultats sous forme de chaîne structurée dans la réponse JSON-RPC
- Écrit les logs de débogage sur stderr uniquement

## Outils MCP exposés

| Outil | Description |
|-------|-------------|
| `kaal_context` | Lit et retourne le contenu de `kaal.yaml` et l'environnement actif |
| `kaal_generate_dockerfile` | Génère un `Dockerfile` adapté au stack détecté |
| `kaal_generate_compose` | Génère un fichier `docker-compose.<env>.yml` |
| `kaal_init` | Initialise un nouveau projet kaal (scaffold complet) |
| `kaal_env_switch` | Change l'environnement actif (équivalent de `kaal env use`) |
| `kaal_up` | Démarre l'environnement local |
| `kaal_down` | Arrête l'environnement local |
| `kaal_push` | Build et push l'image vers le registry |
| `kaal_deploy` | Déploie sur la cible distante |
| `kaal_rollback` | Revient au déploiement précédent ou à une version spécifique |
| `kaal_sync` | Synchronise les fichiers de config locaux vers le VPS |
| `kaal_status` | Retourne l'état des services (local ou distant) |
| `kaal_logs` | Retourne les logs d'un ou plusieurs services |
| `kaal_config_get` | Lit une valeur dans `kaal.yaml` |
| `kaal_config_set` | Écrit une valeur dans `kaal.yaml` |
| `kaal_preflight` | Exécute les vérifications prérequis et retourne le résultat structuré |
| `kaal_setup` | Prépare un VPS vierge (groupe docker, répertoire kaal) |
| `kaal_secrets_inject` | Injecte les secrets dans le fichier env local |

## Utilisation par les agents

Le champ `fix_type` retourné par `kaal_preflight` indique à l'agent ce qu'il peut corriger autonomement (`FixAgent`) ou ce qui requiert une intervention humaine (`FixHuman`). Un agent bien conçu suit cette séquence :

1. `kaal_context` : lire la configuration existante
2. `kaal_preflight` : identifier les bloquants
3. Traiter tous les `FixAgent` automatiquement
4. Demander à l'humain de résoudre les `FixHuman`
5. `kaal_push` puis `kaal_deploy`

## Références

- Schémas complets des outils et protocole JSON-RPC : [docs/internals/mcp-server.md](../internals/mcp-server.md)
- Séquence recommandée pour les agents : [docs/workflows/ai-agent.md](../workflows/ai-agent.md)
