// Package runtime is the wiring layer: it reads pilot.yaml and constructs the
// correct domain port implementations. cmd/ and mcp/ import runtime — never
// the adapter packages directly (except for VPS-specific features like history).
package runtime

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/adapters/aws"
	"github.com/mouhamedsylla/pilot/internal/adapters/azure"
	"github.com/mouhamedsylla/pilot/internal/adapters/compose"
	"github.com/mouhamedsylla/pilot/internal/adapters/do"
	"github.com/mouhamedsylla/pilot/internal/adapters/gcp"
	"github.com/mouhamedsylla/pilot/internal/adapters/k8s"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/acr"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/custom"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/dockerhub"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/ecr"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/gcr"
	"github.com/mouhamedsylla/pilot/internal/adapters/registry/ghcr"
	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/aws_sm"
	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/gcp_sm"
	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/local"
	"github.com/mouhamedsylla/pilot/internal/adapters/vps"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// NewExecutionProvider returns the local runtime (compose, k8s) for the given env.
// The returned value directly implements domain.ExecutionProvider.
func NewExecutionProvider(cfg *config.Config, env string) (domain.ExecutionProvider, error) {
	envCfg, ok := cfg.Environments[env]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined in pilot.yaml", env)
	}
	rt := envCfg.Runtime
	if rt == "" {
		rt = config.RuntimeCompose
	}
	switch rt {
	case config.RuntimeCompose:
		return compose.New(cfg, env), nil
	case config.RuntimeK3d, config.RuntimeLima:
		return k8s.New(cfg, env), nil
	default:
		return nil, fmt.Errorf("unknown runtime %q", rt)
	}
}

// NewDeployProvider returns the deployment target for the given target name.
// The returned value directly implements domain.DeployProvider.
func NewDeployProvider(cfg *config.Config, targetName string) (domain.DeployProvider, error) {
	p, err := newRawProvider(cfg, targetName)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewRegistryProvider returns the registry backend declared in pilot.yaml.
// The returned value directly implements domain.RegistryProvider.
func NewRegistryProvider(cfg *config.Config) (domain.RegistryProvider, error) {
	r := cfg.Registry
	switch r.Provider {
	case "ghcr":
		return ghcr.New(r.Image), nil
	case "dockerhub":
		return dockerhub.New(r.Image), nil
	case "ecr":
		return ecr.New(r.Image, r.URL), nil
	case "gcr":
		return gcr.New(r.Image), nil
	case "acr":
		return acr.New(r.Image, r.URL), nil
	case "custom":
		return custom.New(r.Image, r.URL), nil
	default:
		return nil, fmt.Errorf("unknown registry provider %q", r.Provider)
	}
}

// NewSecretManager returns the secret backend for the given provider name.
func NewSecretManager(provider string) (domain.SecretManager, error) {
	switch provider {
	case "local", "":
		return local.New(), nil
	case "aws_sm":
		return aws_sm.New(), nil
	case "gcp_sm":
		return gcp_sm.New(), nil
	default:
		return nil, fmt.Errorf("unknown secret manager provider %q", provider)
	}
}

// hookRunnerIface matches the RunHooks method on *vps.Provider.
type hookRunnerIface interface {
	RunHooks(ctx context.Context, commands []string) error
}

// migrationRunnerIface matches the RunMigrations/RollbackMigrations methods on *vps.Provider.
type migrationRunnerIface interface {
	RunMigrations(ctx context.Context, tool, command string) error
	RollbackMigrations(ctx context.Context, tool, rollbackCommand string) error
}

// hookRunnerAdapter bridges vps.Provider → domain.HookRunner.
type hookRunnerAdapter struct{ inner hookRunnerIface }

func (a *hookRunnerAdapter) RunHooks(ctx context.Context, commands []string) error {
	return a.inner.RunHooks(ctx, commands)
}

// migrationRunnerAdapter bridges vps.Provider → domain.MigrationRunner.
type migrationRunnerAdapter struct{ inner migrationRunnerIface }

func (a *migrationRunnerAdapter) RunMigrations(ctx context.Context, cfg domain.MigrationConfig) error {
	return a.inner.RunMigrations(ctx, cfg.Tool, cfg.Command)
}

func (a *migrationRunnerAdapter) RollbackMigrations(ctx context.Context, cfg domain.MigrationConfig) error {
	return a.inner.RollbackMigrations(ctx, cfg.Tool, cfg.RollbackCommand)
}

// NewHookRunner returns a domain.HookRunner for the given target, or nil if unsupported.
func NewHookRunner(cfg *config.Config, targetName string) (domain.HookRunner, error) {
	p, err := newRawProvider(cfg, targetName)
	if err != nil {
		return nil, err
	}
	hr, ok := p.(hookRunnerIface)
	if !ok {
		return nil, nil // hooks silently skipped for non-VPS targets
	}
	return &hookRunnerAdapter{inner: hr}, nil
}

// NewMigrationRunner returns a domain.MigrationRunner for the given target, or nil if unsupported.
func NewMigrationRunner(cfg *config.Config, targetName string) (domain.MigrationRunner, error) {
	p, err := newRawProvider(cfg, targetName)
	if err != nil {
		return nil, err
	}
	mr, ok := p.(migrationRunnerIface)
	if !ok {
		return nil, nil // migrations silently skipped for non-VPS targets
	}
	return &migrationRunnerAdapter{inner: mr}, nil
}

// newRawProvider instantiates the concrete provider for the given target.
// Returns domain.DeployProvider (all providers implement it directly).
func newRawProvider(cfg *config.Config, targetName string) (domain.DeployProvider, error) {
	target, ok := cfg.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("target %q not defined in pilot.yaml", targetName)
	}
	switch target.Type {
	case "vps", "hetzner":
		return vps.New(cfg, target), nil
	case "aws":
		return aws.New(cfg, target), nil
	case "gcp":
		return gcp.New(cfg, target), nil
	case "azure":
		return azure.New(cfg, target), nil
	case "do":
		return do.New(cfg, target), nil
	default:
		return nil, fmt.Errorf("unknown provider type %q", target.Type)
	}
}
