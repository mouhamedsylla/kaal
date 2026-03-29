// Package runtime is the only package that imports both interface packages
// and their implementations. It wires everything together based on kaal.yaml.
// cmd/ packages import runtime — never the implementation packages directly.
package runtime

import (
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/orchestrator"
	"github.com/mouhamedsylla/kaal/internal/orchestrator/compose"
	"github.com/mouhamedsylla/kaal/internal/orchestrator/k8s"
	"github.com/mouhamedsylla/kaal/internal/providers"
	"github.com/mouhamedsylla/kaal/internal/providers/aws"
	"github.com/mouhamedsylla/kaal/internal/providers/azure"
	"github.com/mouhamedsylla/kaal/internal/providers/do"
	"github.com/mouhamedsylla/kaal/internal/providers/gcp"
	"github.com/mouhamedsylla/kaal/internal/providers/vps"
	"github.com/mouhamedsylla/kaal/internal/registry"
	"github.com/mouhamedsylla/kaal/internal/registry/acr"
	"github.com/mouhamedsylla/kaal/internal/registry/custom"
	"github.com/mouhamedsylla/kaal/internal/registry/dockerhub"
	"github.com/mouhamedsylla/kaal/internal/registry/ecr"
	"github.com/mouhamedsylla/kaal/internal/registry/gcr"
	"github.com/mouhamedsylla/kaal/internal/registry/ghcr"
	"github.com/mouhamedsylla/kaal/internal/secrets"
	"github.com/mouhamedsylla/kaal/internal/secrets/aws_sm"
	"github.com/mouhamedsylla/kaal/internal/secrets/gcp_sm"
	"github.com/mouhamedsylla/kaal/internal/secrets/local"
)

// NewOrchestrator returns the correct Orchestrator for the given environment.
func NewOrchestrator(cfg *config.Config, env string) (orchestrator.Orchestrator, error) {
	envCfg, ok := cfg.Environments[env]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined in kaal.yaml", env)
	}
	otype := envCfg.Orchestrator
	if otype == "" {
		otype = "compose"
	}
	switch otype {
	case "compose":
		return compose.New(cfg, env), nil
	case "k8s":
		return k8s.New(cfg, env), nil
	default:
		return nil, fmt.Errorf("unknown orchestrator %q", otype)
	}
}

// NewProvider returns the correct Provider for the given target name.
func NewProvider(cfg *config.Config, targetName string) (providers.Provider, error) {
	target, ok := cfg.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("target %q not defined in kaal.yaml", targetName)
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

// NewRegistry returns the correct Registry based on kaal.yaml registry config.
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
