# Internals : Providers

Les providers gèrent le **déploiement sur les cibles distantes** — VPS, cloud, etc.

---

## Interface

```go
// internal/providers/provider.go
type Provider interface {
    Deploy(ctx context.Context, opts DeployOptions) error
    Sync(ctx context.Context, files []string) error
    Status(ctx context.Context) ([]ServiceStatus, error)
    Rollback(ctx context.Context, version string) error
}
```

---

## Implémentations

### `vps` — SSH + Docker Compose (implémenté)

Fichier : `internal/providers/vps/ssh.go`

Utilise `pkg/ssh` pour se connecter et exécuter des commandes à distance.

**Flux de Deploy :**
1. Connexion SSH (`host:port`, `user`, clé privée)
2. `Sync()` — copie `docker-compose.<env>.yml` et `.env.<env>` si présent
3. `docker compose -f docker-compose.<env>.yml pull`
4. `docker compose -f docker-compose.<env>.yml up -d`

**Flux de Rollback :**
1. Lit le tag précédent depuis `~/.kaal/<project>/last-tag` sur le VPS
2. Met à jour le compose file avec le tag précédent
3. `docker compose up -d`

**`Sync` — fichiers copiés :**
- `docker-compose.<env>.yml` (toujours)
- `.env.<env>` (si présent localement)
- Tout fichier passé en paramètre

### `aws` — AWS ECS/EKS (stub)

### `gcp` — GCP Cloud Run/GKE (stub)

### `azure` — Azure Container Apps/AKS (stub)

### `do` — DigitalOcean (stub)

---

## Factory

```go
// internal/runtime/runtime.go
func NewProvider(cfg *config.Config, targetName string) (providers.Provider, error) {
    target, ok := cfg.Targets[targetName]
    if !ok {
        return nil, fmt.Errorf("target %q not found in kaal.yaml", targetName)
    }

    switch target.Type {
    case "vps":
        return vps.New(target), nil
    case "aws":
        return aws.New(target), nil
    // ...
    }
}
```

---

## `pkg/ssh` — Client SSH

Le client SSH bas niveau utilisé par le provider VPS.

```go
// pkg/ssh/client.go
type Client struct { /* connexion SSH */ }

func New(host, user, keyPath string, port int) (*Client, error)
func (c *Client) Run(ctx context.Context, cmd string) (string, error)
func (c *Client) CopyFiles(ctx context.Context, files map[string]string) error
// files = map[localPath]remotePath
```

`CopyFiles` utilise SCP pour transférer les fichiers.

---

## Ajouter un nouveau provider

1. Créer `internal/providers/<name>/<name>.go`
2. Implémenter l'interface `Provider`
3. Ajouter le cas dans `internal/runtime/runtime.go`

```go
// internal/providers/hetzner/hetzner.go
package hetzner

import (
    "context"
    "fmt"
    "github.com/mouhamedsylla/kaal/internal/config"
    "github.com/mouhamedsylla/kaal/internal/providers"
)

type Hetzner struct{ target config.Target }

func New(target config.Target) providers.Provider {
    return &Hetzner{target: target}
}

func (h *Hetzner) Deploy(ctx context.Context, opts providers.DeployOptions) error {
    return fmt.Errorf("hetzner: not yet implemented")
}
// ...
```
