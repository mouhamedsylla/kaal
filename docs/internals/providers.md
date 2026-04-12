# Internals : Déploiement distant (DeployProvider)

Les `DeployProvider` gèrent le **déploiement sur les cibles distantes** : VPS, cloud, etc.
Ils implémentent également deux interfaces optionnelles pour les hooks et migrations.

---

## Interfaces

Toutes définies dans `internal/domain/ports.go`.

### `DeployProvider` (obligatoire)

```go
type DeployProvider interface {
    Sync(ctx context.Context, env string) error
    Deploy(ctx context.Context, env string, opts DeployOptions) error
    Rollback(ctx context.Context, env string, toTag string) (restoredTag string, err error)
    Status(ctx context.Context, env string) ([]ServiceStatus, error)
    Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}
```

### `HookRunner` (optionnel : pre/post deploy)

```go
type HookRunner interface {
    RunHooks(ctx context.Context, commands []string) error
}
```

### `MigrationRunner` (optionnel : migrations schema)

```go
type MigrationRunner interface {
    RunMigrations(ctx context.Context, cfg MigrationConfig) error
    RollbackMigrations(ctx context.Context, cfg MigrationConfig) error
}
```

`HookRunner` et `MigrationRunner` sont des interfaces optionnelles : si le provider ne les implémente pas (ou si le factory retourne `nil`), les étapes correspondantes sont silencieusement ignorées dans le pipeline de déploiement.

---

## Implémentations

### `vps` : SSH + Docker Compose (implémenté)

Fichier : `internal/adapters/vps/ssh.go`

Utilise `pkg/ssh` pour se connecter et exécuter des commandes à distance.
Implémente `DeployProvider`, `HookRunner` et `MigrationRunner`.

#### Répertoire de travail distant

Toutes les commandes utilisent `~/pilot/` comme répertoire de travail :

```go
func remoteComposeFile(env string) string {
    return fmt.Sprintf("~/pilot/docker-compose.%s.yml", env)
}
```

#### Flux de `Deploy`

Le pipeline complet est orchestré par `app/deploy.DeployUseCase`, pas par le provider lui-même. Le provider expose les briques primitives :

1. `Sync(env)` : copie les fichiers vers `~/pilot/` sur le VPS
2. `RunHooks(ctx, commands)` : exécute des commandes SSH arbitraires
3. `RunMigrations(ctx, tool, command)` : exécute la commande de migration via SSH
4. `Deploy(ctx, env, opts)` : `docker pull` + `docker compose up -d`
5. `Rollback(ctx, env, toTag)` : restaure un tag précédent

#### Erreurs structurées (taxonomie TypeA/B/C/D)

`vps.Provider` retourne des `*domain/errors.PilotError` typés :

| Situation | Type | Code |
|-----------|------|------|
| Connexion SSH échoue | D | `PILOT-SSH-001` |
| `docker pull` échoue | D | `PILOT-DEPLOY-002` |
| User pas dans le groupe docker | **C** | `PILOT-DEPLOY-003` |
| `docker compose up` échoue | D | `PILOT-DEPLOY-004` |
| Rollback image échoue | D | `PILOT-DEPLOY-005` |
| Rollback migration échoue | D | `PILOT-DEPLOY-006` |

Pour PILOT-DEPLOY-003 (TypeC), le provider génère automatiquement deux options :

```
→ [0] pilot setup --env <env>   (automatique)
  [1] ssh <user>@<host> 'sudo usermod -aG docker <user>'   (manuel)
```

pilot suspend l'opération et attend un choix. `pilot resume --answer 0` reprend depuis le début.

#### Flux de `Sync`

`Sync(env)` collecte et copie :

1. Tous les compose files déclarés dans `pilot.yaml`
2. Tous les env files (`environments.<env>.env_file`)
3. Tous les fichiers bind-mount détectés dans les compose files

```go
// internal/adapters/vps/ssh.go
func parseComposeMounts(composeFile string) ([]string, error)
```

Parse le YAML du compose file et extrait les sources locales des volumes (syntaxe courte `./path:container` et longue `source: ./path`). Seuls les chemins relatifs sont retenus.

Exemple : si `docker-compose.prod.yml` contient :
```yaml
volumes:
  - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf:ro
```

pilot copie `./nginx/prod.conf` → `~/pilot/nginx/prod.conf` sur le VPS, en préservant la structure. Docker compose trouve le fichier exactement là où il l'attend.

#### Output distant

Toutes les sorties SSH sont préfixées `  │ ` pour distinguer l'output remote du terminal local :

```
  │  Step 1/5 : FROM golang:1.23-alpine
  │  Step 2/5 : WORKDIR /app
```

### `aws` / `gcp` / `azure` / `do` : Cloud providers (stubs)

Fichiers : `internal/adapters/{aws,gcp,azure,do}/`

Retournent `fmt.Errorf("xxx: not yet implemented")`.

---

## Factory

```go
// internal/app/runtime/runtime.go
func NewDeployProvider(cfg *config.Config, targetName string) (domain.DeployProvider, error)
func NewHookRunner(cfg *config.Config, targetName string) (domain.HookRunner, error)
func NewMigrationRunner(cfg *config.Config, targetName string) (domain.MigrationRunner, error)
```

`NewHookRunner` et `NewMigrationRunner` font une assertion de type duck-typée :
si le provider concret n'implémente pas `RunHooks` / `RunMigrations`, ils retournent `nil` sans erreur.

Les adapters runtime (`hookRunnerAdapter`, `migrationRunnerAdapter`) dans `runtime.go` servent de pont entre les signatures concrètes de `vps.Provider` et les interfaces `domain.HookRunner` / `domain.MigrationRunner`.

---

## `pkg/ssh` : Client SSH

Le client SSH bas niveau utilisé par le provider VPS.

```go
// pkg/ssh/client.go
type Client struct { /* connexion SSH */ }

func New(host, user, keyPath string, port int) (*Client, error)
func (c *Client) Run(ctx context.Context, cmd string) (string, error)
func (c *Client) CopyFiles(ctx context.Context, files map[string]string) error
func (c *Client) CopyFileTo(ctx context.Context, localPath, remotePath string) error
```

- `CopyFiles` : transfert SCP de plusieurs fichiers `map[localPath]remotePath`
- `CopyFileTo` : copie un fichier vers un chemin distant exact, crée les répertoires parents avec `mkdir -p`

---

## Ajouter un nouveau provider

1. Créer `internal/adapters/<name>/<name>.go`
2. Implémenter `domain.DeployProvider` (5 méthodes)
3. Ajouter le cas dans `runtime.newRawProvider()` pour le nouveau `target.Type`

Exemple minimal :

```go
// internal/adapters/hetzner/hetzner.go
package hetzner

import (
    "context"
    "fmt"
    domain "github.com/mouhamedsylla/pilot/internal/domain"
    "github.com/mouhamedsylla/pilot/internal/config"
)

type Hetzner struct {
    cfg    *config.Config
    target config.Target
}

func New(cfg *config.Config, target config.Target) domain.DeployProvider {
    return &Hetzner{cfg: cfg, target: target}
}

func (h *Hetzner) Deploy(ctx context.Context, env string, opts domain.DeployOptions) error {
    return fmt.Errorf("hetzner: not yet implemented")
}

func (h *Hetzner) Sync(ctx context.Context, env string) error {
    return fmt.Errorf("hetzner: not yet implemented")
}
// ... Rollback, Status, Logs
```

Ensuite dans `runtime.newRawProvider()` :
```go
case "hetzner":
    return hetzner.New(cfg, target), nil
```

C'est tout. Le pipeline de déploiement, le preflight, le rollback, le sync, le status et les logs fonctionnent immédiatement : ils opèrent sur l'interface, pas sur le type concret.
