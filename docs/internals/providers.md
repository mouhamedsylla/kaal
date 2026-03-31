# Internals : Providers

Les providers gèrent le **déploiement sur les cibles distantes** : VPS, cloud, etc.

---

## Interface

```go
// internal/providers/provider.go
type Provider interface {
    Deploy(ctx context.Context, opts DeployOptions) error
    Sync(ctx context.Context, env string) error
    Status(ctx context.Context, env string) ([]ServiceStatus, error)
    Logs(ctx context.Context, service, env string, lines int, since string) (string, error)
    Rollback(ctx context.Context, env, version string) error
    Setup(ctx context.Context, env string) error
}
```

`DeployOptions` :

```go
type DeployOptions struct {
    Env string
    Tag string
}
```

---

## Implémentations

### `vps` : SSH + Docker Compose (implémenté)

Fichier : `internal/providers/vps/ssh.go`

Utilise `pkg/ssh` pour se connecter et exécuter des commandes à distance.

#### Flux de Deploy

1. Connexion SSH (`host:port`, `user`, clé privée)
2. `Sync(env)` : copie tous les fichiers vers `~/pilot/` sur le VPS
3. `docker pull <image>:<tag>` sur le VPS
4. `IMAGE_TAG=<tag> docker compose -f ~/pilot/docker-compose.<env>.yml up -d --remove-orphans`
5. Sauvegarde le tag dans `~/.pilot/<project>/current-tag`

#### Répertoire de travail distant

Toutes les commandes utilisent `~/pilot/` comme répertoire de travail :

```go
func remoteComposeFile(env string) string {
    return fmt.Sprintf("~/pilot/docker-compose.%s.yml", env)
}
```

Jamais de chemins relatifs, jamais de commandes dans le home directory racine.

#### Flux de Sync

`Sync(env)` collecte et copie :

1. **Tous les compose files** déclarés dans `pilot.yaml` pour tous les environnements
2. **Tous les env files** (`environments.<env>.env_file`) pour tous les environnements
3. **Tous les fichiers bind-mount** détectés dans chaque compose file

```go
func parseComposeMounts(composeFile string) ([]string, error)
```

Cette fonction parse le YAML du compose file et extrait les sources locales des volumes (syntaxe courte `./path:container` et syntaxe longue `source: ./path`). Seuls les chemins relatifs (`./ ou ../`) sont retenus : les volumes Docker nommés sont ignorés.

Exemple : si `docker-compose.prod.yml` contient :
```yaml
volumes:
  - ./nginx/prod.conf:/etc/nginx/conf.d/default.conf:ro
```

pilot copie `./nginx/prod.conf` vers `~/pilot/nginx/prod.conf` sur le VPS, en préservant la structure de répertoires. Docker compose trouve le fichier exactement là où il l'attend.

#### Flux de Rollback

1. Lit le tag précédent depuis `~/.pilot/<project>/prev-tag` sur le VPS
2. `IMAGE_TAG=<prev-tag> docker compose -f ~/pilot/docker-compose.<env>.yml up -d`
3. Avec `--version` : utilise directement le tag spécifié

#### Setup (Docker group)

```bash
sudo usermod -aG docker <user>
```

Exécuté via SSH avec sudo. Nécessite que le user ait les droits sudo sur le VPS.

### `aws` : AWS ECS/EKS (stub)

### `gcp` : GCP Cloud Run/GKE (stub)

### `azure` : Azure Container Apps/AKS (stub)

### `do` : DigitalOcean (stub)

---

## Factory

```go
// internal/runtime/runtime.go
func NewProvider(cfg *config.Config, targetName string) (providers.Provider, error) {
    target, ok := cfg.Targets[targetName]
    if !ok {
        return nil, fmt.Errorf("target %q not found in pilot.yaml", targetName)
    }

    switch target.Type {
    case "vps":
        return vps.New(cfg, target), nil
    case "aws":
        return aws.New(target), nil
    // ...
    }
}
```

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

- `CopyFiles` : transfert SCP de plusieurs fichiers, `map[localPath]remotePath`
- `CopyFileTo` : copie un fichier vers un chemin distant exact, crée les répertoires parents avec `mkdir -p`

`CopyFileTo` est utilisé par `Sync` pour les bind-mounts afin de garantir que le fichier distant est bien un fichier (et non un répertoire que Docker créerait silencieusement).

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
    "github.com/mouhamedsylla/pilot/internal/config"
    "github.com/mouhamedsylla/pilot/internal/providers"
)

type Hetzner struct {
    cfg    *config.Config
    target config.Target
}

func New(cfg *config.Config, target config.Target) providers.Provider {
    return &Hetzner{cfg: cfg, target: target}
}

func (h *Hetzner) Deploy(ctx context.Context, opts providers.DeployOptions) error {
    return fmt.Errorf("hetzner: not yet implemented")
}

func (h *Hetzner) Sync(ctx context.Context, env string) error {
    return fmt.Errorf("hetzner: not yet implemented")
}
// ...
```
