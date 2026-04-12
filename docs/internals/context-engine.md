# Internals : Moteur de contexte

`internal/mcp/context` est le package qui collecte une photo complète du projet à un instant T. C'est la pièce centrale qui permet aux agents AI de comprendre le projet sans avoir besoin de lire chaque fichier manuellement.

---

## Pourquoi ce package existe

Un agent AI a besoin de contexte pour générer de bons fichiers d'infra. Sans contexte :
- Il génère un Dockerfile Go générique, pas optimisé pour ton projet
- Il ne sait pas quelle version de Go tu utilises
- Il ne sait pas si tu as un Dockerfile existant à respecter
- Il ne connaît pas tes services

Avec `pilot context`, l'agent reçoit tout ce dont il a besoin en un seul appel.

---

## `ProjectContext`

```go
// internal/mcp/context/context.go
type ProjectContext struct {
    // Source de vérité
    KaalYAML string  // contenu brut de pilot.yaml (champ JSON: "pilot_yaml")

    // Détection automatique
    Stack             string  // "go", "node", "python", "rust", "java"
    LanguageVersion   string  // "1.23", "20", "3.12"
    IsExistingProject bool    // true si des fichiers de code existent déjà

    // Structure du projet
    FileTree string     // Arbre (3 niveaux max, bruit filtré)
    KeyFiles []string   // go.mod, package.json, Cargo.toml, etc.

    // Infra existante
    ExistingDockerfiles  []string
    ExistingComposeFiles []string
    ExistingEnvFiles     []string

    // Ce qui manque (pour l'env actif)
    MissingDockerfile bool
    MissingCompose    bool
    MissingComposeEnvs []string  // tous les envs configurés sans compose
    ActiveEnv          string

    // Config parsée (accès structuré)
    Config *config.Config
}
```

---

## `Collect(activeEnv string)`

Fonction principale. Collecte tout depuis le répertoire courant.

```go
projCtx, err := context.Collect("dev")
```

**Ce que Collect fait :**

1. `config.Load(".")` : parse pilot.yaml
2. `scaffold.Detect(".")` : détecte le stack depuis les fichiers manifestes
3. Construit le `FileTree` en ignorant : `.git`, `node_modules`, `vendor`, `.cache`, `dist`, `build`, `__pycache__`
4. Cherche les fichiers clés : `go.mod`, `package.json`, `Cargo.toml`, `requirements.txt`, `pyproject.toml`, `pom.xml`, `build.gradle`, `Makefile`
5. Liste les Dockerfiles et compose files existants
6. Détermine `MissingDockerfile` et `MissingCompose` pour l'env actif
7. Détermine `MissingComposeEnvs` : **tous** les environnements configurés sans leur fichier compose (pas seulement l'env actif)

---

## `AgentPrompt()`

Génère un document Markdown structuré prêt à être consommé par n'importe quel LLM.

Structure du document généré :
```
## pilot.yaml
[contenu brut]

## Project structure
[arbre de fichiers]

## Key files detected
[liste]

## Existing Dockerfiles
[contenu de chaque Dockerfile existant]

## Stack
- Language: go 1.23
- Active environment: dev

## Services defined in pilot.yaml
[YAML des services]

## What is needed
- Dockerfile is missing...
- docker-compose.dev.yml is missing...
```

---

## `Summary()`

Version courte pour l'affichage terminal (`pilot context --summary`).

```
Project:  taskflow
Stack:    go 1.23
Env:      dev

Services:
  api          type=app        port=8080
  db           type=postgres
  cache        type=redis

Dockerfiles: Dockerfile
Compose:     docker-compose.dev.yml
```

---

## Filtrage du FileTree

```go
var skipDirs = map[string]bool{
    ".git": true, "node_modules": true, "vendor": true,
    ".cache": true, "dist": true, "build": true, "__pycache__": true,
}
```

Les fichiers cachés (`.`) à la racine sont ignorés, sauf `.env.example` : car `.env.example` documente les variables requises et est utile pour l'agent.

---

## Champs JSON retournés par `pilot_context` (MCP)

| Champ JSON | Source | Description |
|------------|--------|-------------|
| `pilot_yaml` | `KaalYAML` | Contenu brut de pilot.yaml |
| `stack` | `Stack` | Langage détecté |
| `language_version` | `LanguageVersion` | Version du langage |
| `is_existing_project` | `IsExistingProject` | True si code existant |
| `file_tree` | `FileTree` | Arbre de fichiers filtré |
| `key_files` | `KeyFiles` | Fichiers manifestes détectés |
| `existing_dockerfiles` | `ExistingDockerfiles` | Dockerfiles présents |
| `existing_compose_files` | `ExistingComposeFiles` | Composes présents |
| `existing_env_files` | `ExistingEnvFiles` | Fichiers .env présents |
| `missing_dockerfile` | `MissingDockerfile` | Dockerfile manquant ? |
| `missing_compose` | `MissingCompose` | Compose manquant (env actif) ? |
| `missing_compose_envs` | `MissingComposeEnvs` | **Liste de tous les envs** sans compose |
| `active_env` | `ActiveEnv` | Environnement actif |
| `agent_prompt` | `AgentPrompt()` | Prompt Markdown complet |
| `services` | `Config.Services` | Services de pilot.yaml |
| `environments` | `Config.Environments` | Environnements de pilot.yaml |

`missing_compose_envs` permet à l'agent de générer **tous** les composes manquants en un seul passage, pas seulement celui de l'env actif.

---

## Utilisation dans l'écosystème pilot

| Consommateur | Usage |
|--------------|-------|
| `pilot up` | Vérifie `MissingDockerfile` et `MissingCompose` (env actif) + staleness |
| `pilot context` | Affiche `AgentPrompt()` ou `Summary()` |
| MCP `pilot_context` | Retourne le contexte en JSON pour l'agent |
| MCP `pilot_up` | Passe le contexte à l'agent si fichiers manquants |

### Services managés dans `AgentPrompt()`

Quand des services ont `hosting: managed`, `AgentPrompt()` inclut une section
**CRITICAL** que l'agent doit respecter :

```markdown
## CRITICAL : Managed services (DO NOT generate Docker Compose blocks)

The following services are externally hosted. Do NOT add them as services
in docker-compose. They are provided by external cloud providers and
your application connects to them via environment variables only.

| Service | Type     | Provider   | Env vars needed                    |
|---------|----------|------------|------------------------------------|
| db      | postgres | neon       | DATABASE_URL                       |
| cache   | redis    | upstash    | UPSTASH_REDIS_REST_URL, ...        |
```

C'est la garantie que l'agent ne génère pas accidentellement un container postgres
alors que le projet utilise Neon.
