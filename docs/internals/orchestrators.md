# Internals : Orchestrateurs

Les orchestrateurs sont responsables de démarrer, arrêter et surveiller les services **localement**.

---

## Interface

```go
// internal/orchestrator/orchestrator.go
type Orchestrator interface {
    Up(ctx context.Context, env string, services []string) error
    Down(ctx context.Context, env string) error
    Logs(ctx context.Context, service string, opts LogOptions) (<-chan string, error)
    Status(ctx context.Context) ([]ServiceStatus, error)
}
```

---

## Implémentations

### `compose` — Docker Compose (implémenté)

Fichier : `internal/orchestrator/compose/compose.go`

Convention de nommage des fichiers : `docker-compose.<env>.yml`

```go
func (c *Compose) Up(ctx context.Context, env string, services []string) error {
    args := []string{"compose", "-f", composeFileForEnv(env), "up", "-d"}
    args = append(args, services...)
    return exec.CommandContext(ctx, "docker", args...).Run()
}
```

Prérequis : `docker` CLI installé avec le plugin `compose`.

### `k3d` — Kubernetes local (stub)

Fichier : `internal/orchestrator/k8s/k8s.go`

k3d crée un cluster Kubernetes local dans Docker. Idéal pour tester en conditions réelles de prod quand la prod tourne sur k8s.

```yaml
# kaal.yaml
environments:
  test:
    runtime: k3d
```

Roadmap :
- Créer/supprimer un cluster k3d au `Up`/`Down`
- Générer les manifests Kubernetes depuis `kaal.yaml` (ou déléguer à l'agent)
- Exposer les services localement via port-forward

### `lima` — VMs légères (stub)

Lima crée des VMs Linux légères sur macOS. Permet de simuler une vraie VM locale.

```yaml
environments:
  dev-vm:
    runtime: lima
```

Roadmap :
- Créer une VM Lima avec la config définie dans `kaal.yaml`
- Injecter Docker dans la VM
- Copier et démarrer les services dans la VM

---

## Factory

```go
// internal/runtime/runtime.go
func NewOrchestrator(cfg *config.Config, env string) (orchestrator.Orchestrator, error) {
    envCfg := cfg.Environments[env]
    switch envCfg.Runtime {
    case config.RuntimeCompose, "":
        return compose.New(cfg), nil
    case config.RuntimeK3d:
        return k8s.New(cfg), nil
    case config.RuntimeLima:
        return nil, fmt.Errorf("lima: not yet implemented")
    default:
        return nil, fmt.Errorf("unknown runtime %q", envCfg.Runtime)
    }
}
```

---

## Ajouter un nouveau runtime

1. Créer `internal/orchestrator/<runtime>/` avec un fichier `<runtime>.go`
2. Implémenter l'interface `Orchestrator`
3. Ajouter une constante dans `internal/config/types.go` : `RuntimeXxx = "xxx"`
4. Ajouter le cas dans le switch de `internal/runtime/runtime.go`

Exemple minimal :

```go
package myvms

import (
    "context"
    "fmt"
    "github.com/mouhamedsylla/kaal/internal/config"
    "github.com/mouhamedsylla/kaal/internal/orchestrator"
)

type MyVMs struct{ cfg *config.Config }

func New(cfg *config.Config) orchestrator.Orchestrator {
    return &MyVMs{cfg: cfg}
}

func (m *MyVMs) Up(ctx context.Context, env string, services []string) error {
    return fmt.Errorf("myvms: not yet implemented")
}
// ... Down, Logs, Status
```
