# Internals : Exécution locale (ExecutionProvider)

Les `ExecutionProvider` gèrent le démarrage, l'arrêt et la surveillance des services **localement** (docker compose, k8s...).

---

## Interface

Définie dans `internal/domain/ports.go` : les adapters l'implémentent, le domain ne connaît rien d'eux.

```go
// internal/domain/ports.go
type ExecutionProvider interface {
    Up(ctx context.Context, env string, services []string) error
    Down(ctx context.Context, env string) error
    Status(ctx context.Context, env string) ([]ServiceStatus, error)
    Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}
```

---

## Implémentations

### `compose` : Docker Compose (implémenté)

Fichier : `internal/adapters/compose/compose.go`

Convention de nommage : `docker-compose.<env>.yml`

```go
func (c *Compose) Up(ctx context.Context, env string, services []string) error {
    args := []string{"compose", "-f", composeFileForEnv(env), "up", "-d"}
    args = append(args, services...)
    return exec.CommandContext(ctx, "docker", args...).Run()
}
```

`Status()` parse la sortie de `docker compose ps --json` via `internal/adapters/compose/parser.go`.

Prérequis : `docker` CLI installé avec le plugin `compose`.

### `k8s` : Kubernetes (stub)

Fichier : `internal/adapters/k8s/k8s.go`

Activé en déclarant `runtime: k3d` ou `runtime: lima` dans `pilot.yaml`.

```yaml
environments:
  test:
    runtime: k3d
```

Roadmap :
- Créer/supprimer un cluster k3d au `Up`/`Down`
- Générer les manifests Kubernetes depuis `pilot.yaml`
- Exposer les services localement via port-forward

---

## Factory

```go
// internal/app/runtime/runtime.go
func NewExecutionProvider(cfg *config.Config, env string) (domain.ExecutionProvider, error) {
    envCfg := cfg.Environments[env]
    switch envCfg.Runtime {
    case config.RuntimeCompose, "":
        return compose.New(cfg, env), nil
    case config.RuntimeK3d, config.RuntimeLima:
        return k8s.New(cfg, env), nil
    default:
        return nil, fmt.Errorf("unknown runtime %q", envCfg.Runtime)
    }
}
```

Les commandes `cmd/up.go`, `cmd/status.go`, `cmd/logs.go` appellent `runtime.NewExecutionProvider()` : jamais l'adapter directement.

---

## Ajouter un nouveau runtime

1. Créer `internal/adapters/<runtime>/<runtime>.go`
2. Implémenter `domain.ExecutionProvider` (4 méthodes)
3. Ajouter une constante dans `internal/config/types.go` : `RuntimeXxx = "xxx"`
4. Ajouter le cas dans `runtime.NewExecutionProvider()`

Exemple minimal :

```go
// internal/adapters/podman/podman.go
package podman

import (
    "context"
    "fmt"
    domain "github.com/mouhamedsylla/pilot/internal/domain"
    "github.com/mouhamedsylla/pilot/internal/config"
)

type Podman struct{ cfg *config.Config; env string }

func New(cfg *config.Config, env string) domain.ExecutionProvider {
    return &Podman{cfg: cfg, env: env}
}

func (p *Podman) Up(ctx context.Context, env string, services []string) error {
    return fmt.Errorf("podman: not yet implemented")
}
// ... Down, Status, Logs
```
