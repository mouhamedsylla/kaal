// Package runtime is the only package that imports both interface packages
// and their implementations. It wires everything together based on pilot.yaml.
// cmd/ packages import runtime — never the implementation packages directly.
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
	"github.com/mouhamedsylla/pilot/internal/orchestrator"
	"github.com/mouhamedsylla/pilot/internal/providers"
	"github.com/mouhamedsylla/pilot/internal/registry"
	"github.com/mouhamedsylla/pilot/internal/secrets"
)

// NewOrchestrator returns the correct Orchestrator for the given environment.
func NewOrchestrator(cfg *config.Config, env string) (orchestrator.Orchestrator, error) {
	envCfg, ok := cfg.Environments[env]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined in pilot.yaml", env)
	}
	runtime := envCfg.Runtime
	if runtime == "" {
		runtime = config.RuntimeCompose
	}
	switch runtime {
	case config.RuntimeCompose:
		return compose.New(cfg, env), nil
	case config.RuntimeK3d, config.RuntimeLima:
		return k8s.New(cfg, env), nil
	default:
		return nil, fmt.Errorf("unknown runtime %q", runtime)
	}
}

// NewProvider returns the correct Provider for the given target name.
func NewProvider(cfg *config.Config, targetName string) (providers.Provider, error) {
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

// NewRegistry returns the correct Registry based on pilot.yaml registry config.
func NewRegistry(cfg *config.Config) (registry.Registry, error) {
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

// NewDeployProvider wraps a Provider as a domain.DeployProvider.
// This adapter bridges the old providers.Provider interface to domain ports
// during the incremental migration to the hexagonal architecture.
func NewDeployProvider(cfg *config.Config, targetName string) (domain.DeployProvider, error) {
	p, err := NewProvider(cfg, targetName)
	if err != nil {
		return nil, err
	}
	return &deployProviderAdapter{inner: p}, nil
}

// deployProviderAdapter adapts providers.Provider → domain.DeployProvider.
type deployProviderAdapter struct{ inner providers.Provider }

func (a *deployProviderAdapter) Sync(ctx context.Context, env string) error {
	return a.inner.Sync(ctx, env)
}

func (a *deployProviderAdapter) Deploy(ctx context.Context, env string, opts domain.DeployOptions) error {
	return a.inner.Deploy(ctx, env, providers.DeployOptions{
		Tag:      opts.Tag,
		EnvFiles: opts.EnvFiles,
	})
}

func (a *deployProviderAdapter) Rollback(ctx context.Context, env string, tag string) (string, error) {
	return a.inner.Rollback(ctx, env, tag)
}

func (a *deployProviderAdapter) Status(ctx context.Context, env string) ([]domain.ServiceStatus, error) {
	raw, err := a.inner.Status(ctx, env)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ServiceStatus, len(raw))
	for i, s := range raw {
		out[i] = domain.ServiceStatus{Name: s.Name, State: s.State, Health: s.Health}
	}
	return out, nil
}

func (a *deployProviderAdapter) Logs(ctx context.Context, env string, service string, opts domain.LogOptions) (<-chan string, error) {
	return a.inner.Logs(ctx, env, providers.LogOptions{
		Service: service,
		Follow:  opts.Follow,
		Since:   opts.Since,
		Lines:   opts.Lines,
	})
}

// NewExecutionProvider wraps an Orchestrator as a domain.ExecutionProvider.
func NewExecutionProvider(cfg *config.Config, env string) (domain.ExecutionProvider, error) {
	orch, err := NewOrchestrator(cfg, env)
	if err != nil {
		return nil, err
	}
	return &executionProviderAdapter{inner: orch}, nil
}

// executionProviderAdapter adapts orchestrator.Orchestrator → domain.ExecutionProvider.
type executionProviderAdapter struct{ inner orchestrator.Orchestrator }

func (a *executionProviderAdapter) Up(ctx context.Context, env string, services []string) error {
	return a.inner.Up(ctx, env, services)
}

func (a *executionProviderAdapter) Down(ctx context.Context, env string) error {
	return a.inner.Down(ctx, env)
}

func (a *executionProviderAdapter) Status(ctx context.Context, _ string) ([]domain.ServiceStatus, error) {
	raw, err := a.inner.Status(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ServiceStatus, len(raw))
	for i, s := range raw {
		out[i] = domain.ServiceStatus{Name: s.Name, State: s.State, Health: s.Health}
	}
	return out, nil
}

func (a *executionProviderAdapter) Logs(ctx context.Context, _ string, service string, opts domain.LogOptions) (<-chan string, error) {
	return a.inner.Logs(ctx, service, orchestrator.LogOptions{
		Follow: opts.Follow,
		Since:  opts.Since,
		Lines:  opts.Lines,
	})
}

// NewRegistryProvider wraps a Registry as a domain.RegistryProvider.
func NewRegistryProvider(cfg *config.Config) (domain.RegistryProvider, error) {
	r, err := NewRegistry(cfg)
	if err != nil {
		return nil, err
	}
	return &registryProviderAdapter{inner: r}, nil
}

// registryProviderAdapter adapts registry.Registry → domain.RegistryProvider.
type registryProviderAdapter struct{ inner registry.Registry }

func (a *registryProviderAdapter) Login(ctx context.Context) error {
	return a.inner.Login(ctx)
}

func (a *registryProviderAdapter) Build(ctx context.Context, opts domain.BuildOptions) error {
	return a.inner.Build(ctx, registry.BuildOptions{
		Tag:        opts.Tag,
		Dockerfile: opts.Dockerfile,
		Context:    opts.Context,
		Platforms:  opts.Platforms,
		BuildArgs:  opts.BuildArgs,
		NoCache:    opts.NoCache,
	})
}

func (a *registryProviderAdapter) Push(ctx context.Context, tag string) error {
	return a.inner.Push(ctx, tag)
}

// NewSecretManager returns the correct SecretManager for the given provider name.
func NewSecretManager(provider string) (secrets.SecretManager, error) {
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
