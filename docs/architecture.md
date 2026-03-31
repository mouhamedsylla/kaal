# Architecture du code

## Vue d'ensemble

```
pilot/
├── main.go                   # Entrée : appelle cmd.Execute()
├── cmd/                      # Commandes Cobra (1 fichier = 1 commande)
├── internal/                 # Logique métier (non exportée)
│   ├── config/               # Parse et valide pilot.yaml
│   ├── context/              # Collecte le contexte projet pour les agents AI
│   ├── env/                  # Gestion de l'environnement actif
│   ├── scaffold/             # pilot init : wizard TUI + génération pilot.yaml
│   ├── up/                   # Logique pilot up / pilot down
│   ├── composer/             # Génération docker-compose (utilisé par le MCP)
│   ├── orchestrator/         # Interface + implémentations (compose, k8s)
│   ├── providers/            # Interface + implémentations (vps, aws, gcp...)
│   ├── registry/             # Interface + implémentations (ghcr, dockerhub...)
│   ├── secrets/              # Interface + implémentations (local, aws_sm, gcp_sm)
│   ├── runtime/              # Factory package : instancie les bonnes implémentations
│   └── mcp/                  # Serveur MCP JSON-RPC 2.0 stdio
└── pkg/                      # Packages réutilisables (exportés)
    ├── ui/                   # Spinner, couleurs, output JSON
    └── ssh/                  # Client SSH
```

---

## Principe fondamental : l'interface comme contrat

Chaque couche extensible est définie par une **interface Go** dans son package racine. Toutes les implémentations respectent ce contrat. Les stubs retournent `fmt.Errorf("xxx: not yet implemented")`.

```
interface          implémentations
──────────         ───────────────────────────────────────
Orchestrator   →   compose/ | k3d/ | k8s/
Provider       →   vps/ | aws/ | gcp/ | azure/ | do/
Registry       →   ghcr/ | dockerhub/ | custom/ | ecr/ | gcr/ | acr/
SecretManager  →   local/ | aws_sm/ | gcp_sm/
```

### Pourquoi ce pattern ?

1. **Extensibilité** : ajouter un nouveau provider = implémenter l'interface, pas modifier le code existant.
2. **Testabilité** : on peut mocker n'importe quelle interface.
3. **Stubs propres** : les fonctionnalités non encore implémentées retournent une erreur explicite plutôt qu'un `nil` silencieux.

---

## Le problème des imports circulaires : et sa solution

En Go, si `orchestrator` importe `orchestrator/compose` ET `orchestrator/compose` importe `orchestrator`, c'est une dépendance cyclique → erreur de compilation.

**Solution : `internal/runtime`**

```
                 ┌─────────────────────────────────┐
                 │         internal/runtime         │
                 │  (seul package qui importe tout) │
                 └──────┬──────────────┬────────────┘
                        │              │
               ┌────────▼──────┐  ┌───▼────────────────┐
               │  orchestrator │  │  orchestrator/compose│
               │  (interface)  │  │  (implémentation)    │
               └───────────────┘  └─────────────────────┘
```

`internal/runtime/runtime.go` contient les factories :
```go
func NewOrchestrator(cfg *config.Config, env string) (orchestrator.Orchestrator, error)
func NewProvider(cfg *config.Config, targetName string) (providers.Provider, error)
func NewRegistry(cfg *config.Config) (registry.Registry, error)
func NewSecretManager(provider string) (secrets.SecretManager, error)
```

Les commandes (`cmd/`) importent `internal/runtime` → `runtime` importe les interfaces + les implémentations → pas de cycle.

---

## Flux de données : `pilot up`

```
cmd/up.go
  └─► up.Run(ctx, opts)
        ├─► config.Load(".")          // Parse pilot.yaml
        ├─► env.Active(opts.Env)      // Résout l'env actif (.pilot-current-env)
        ├─► pilotctx.Collect(env)      // Collecte le contexte projet complet
        │
        ├─► [fichiers manquants ?]
        │     └─► missingFilesError() // Affiche le prompt agent, STOP
        │
        └─► runtime.NewOrchestrator() // Instancie compose/ ou k3d/
              └─► orch.Up(ctx, env, services)
                    └─► docker compose up -d
```

## Flux de données : `pilot deploy`

```
cmd/deploy.go
  └─► deploy.Run(ctx, opts)
        ├─► config.Load(".")
        ├─► env.Active(opts.Env)
        ├─► runtime.NewProvider(cfg, targetName)  // Instancie vps/ ou aws/
        │
        ├─► registry.NewRegistry(cfg)
        │     └─► registry.Pull(tag)              // S'assure que l'image existe
        │
        └─► provider.Deploy(ctx, opts)
              ├─► SSH sur le VPS
              ├─► Copie docker-compose.prod.yml
              ├─► docker compose pull
              └─► docker compose up -d
```

## Flux de données : MCP agent workflow

```
Agent AI (Claude / Cursor)
  │
  ├─► tools/call: pilot_context
  │     └─► handlers.HandleContext()
  │           └─► pilotctx.Collect()
  │                 └─► {pilot_yaml, file_tree, stack, missing_files, agent_prompt}
  │
  ├─► [Agent génère le contenu]
  │
  ├─► tools/call: pilot_generate_dockerfile
  │     └─► handlers.HandleGenerateDockerfile()
  │           └─► os.WriteFile("Dockerfile", content)
  │
  ├─► tools/call: pilot_generate_compose
  │     └─► handlers.HandleGenerateCompose()
  │           └─► os.WriteFile("docker-compose.dev.yml", content)
  │
  └─► tools/call: pilot_up
        └─► up.Run(ctx, opts)
              └─► docker compose up -d
```

---

## Packages détaillés

### `internal/config`

Responsabilité : lire, parser, valider `pilot.yaml`.

```
config/
├── types.go     # Structs Go miroir du schéma YAML
├── loader.go    # Load(dir) : remonte les répertoires pour trouver pilot.yaml
└── validator.go # Validation des contraintes métier
```

`config.Load(".")` remonte jusqu'à la racine pour trouver `pilot.yaml` : tu peux lancer `pilot up` depuis n'importe quel sous-dossier du projet.

### `internal/context`

Responsabilité : assembler une photo complète du projet à un instant T pour les agents AI.

```go
type ProjectContext struct {
    PilotYAML             string        // Contenu brut de pilot.yaml
    Stack, LanguageVersion string      // Détectés ou déclarés
    FileTree             string        // Arbre de fichiers (3 niveaux)
    KeyFiles             []string      // go.mod, package.json, etc.
    ExistingDockerfiles  []string
    ExistingComposeFiles []string
    MissingDockerfile    bool
    MissingCompose       bool
    ActiveEnv            string
    Config               *config.Config
}
```

`AgentPrompt()` génère un document Markdown structuré que n'importe quel LLM peut comprendre.

### `internal/orchestrator`

Interface :
```go
type Orchestrator interface {
    Up(ctx context.Context, env string, services []string) error
    Down(ctx context.Context, env string) error
    Logs(ctx context.Context, service string, opts LogOptions) (<-chan string, error)
    Status(ctx context.Context) ([]ServiceStatus, error)
}
```

Implémentations :
- `compose/` : Docker Compose (implémenté)
- `k8s/` : Kubernetes (stub)

Convention de nommage des fichiers : `docker-compose.<env>.yml`

### `internal/providers`

Interface :
```go
type Provider interface {
    Deploy(ctx context.Context, opts DeployOptions) error
    Sync(ctx context.Context, files []string) error
    Status(ctx context.Context) ([]ServiceStatus, error)
    Rollback(ctx context.Context, version string) error
}
```

Implémentations :
- `vps/` : SSH + docker-compose (implémenté)
- `aws/`, `gcp/`, `azure/`, `do/` : stubs

### `internal/registry`

Interface :
```go
type Registry interface {
    Login(ctx context.Context) error
    Build(ctx context.Context, tag, dockerfile string) error
    Push(ctx context.Context, tag string) error
    Pull(ctx context.Context, tag string) error
}
```

Implémentations :
- `ghcr/` : GitHub Container Registry (implémenté)
- `dockerhub/` : Docker Hub (implémenté)
- `custom/` : Registry custom avec auth (implémenté)
- `ecr/`, `gcr/`, `acr/` : stubs

### `internal/mcp`

Serveur JSON-RPC 2.0 sur stdin/stdout. Pas de port réseau, pas de processus séparé.

```
mcp/
├── server.go      # Boucle de lecture + dispatch JSON-RPC
├── tools.go       # Définitions des outils (Tool, InputSchema, Property)
├── handlers.go    # Wiring des handlers
└── handlers/
    ├── stub.go    # Stub pour outils non implémentés
    └── context.go # HandleContext, HandleGenerateDockerfile, HandleGenerateCompose
```

### `pkg/ui`

Toutes les sorties utilisateur passent par ce package : jamais de `fmt.Println` direct dans `internal/`.

```go
ui.Success("message")  // vert
ui.Error("message")    // rouge
ui.Warn("message")     // jaune
ui.Info("message")     // bleu
ui.Dim("message")      // grisé
ui.Bold("message")     // gras
ui.Fatal(err)          // affiche l'erreur + os.Exit(1)
ui.JSON(v)             // sérialise en JSON (pour --json)
```

---

## Conventions de code

| Convention | Règle |
|------------|-------|
| Pas de `panic` | Toujours retourner une `error` |
| Context | Toutes les fonctions I/O acceptent `context.Context` en premier argument |
| Stubs | `fmt.Errorf("xxx: not yet implemented")`, jamais `nil` silencieux |
| Factories | `New(cfg, ...)` dans leur package |
| Output | Tout passe par `pkg/ui` |
| JSON output | Respecter `--json` avec `ui.JSON()` |
| Git | Feature branches (`feat/xxx`, `docs/xxx`), merge `--no-ff` dans `main` |
