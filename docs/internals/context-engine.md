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
    ActiveEnv         string

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

## Utilisation dans l'écosystème pilot

| Consommateur | Usage |
|--------------|-------|
| `pilot up` | Vérifie `MissingDockerfile` et `MissingCompose` |
| `pilot context` | Affiche `AgentPrompt()` ou `Summary()` |
| MCP `pilot_context` | Retourne le contexte en JSON pour l'agent |
| MCP `pilot_up` | Passe le contexte à l'agent si fichiers manquants |
