package do

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Provider is a stub — DigitalOcean support is not yet implemented.
type Provider struct {
	cfg    *config.Config
	target config.Target
}

func New(cfg *config.Config, target config.Target) *Provider {
	return &Provider{cfg: cfg, target: target}
}

func (p *Provider) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error {
	return fmt.Errorf("digitalocean provider: not yet implemented")
}

func (p *Provider) Sync(_ context.Context, _ string) error {
	return fmt.Errorf("digitalocean provider: not yet implemented")
}

func (p *Provider) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, fmt.Errorf("digitalocean provider: not yet implemented")
}

func (p *Provider) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return "", fmt.Errorf("digitalocean provider: not yet implemented")
}

func (p *Provider) Logs(_ context.Context, _ string, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, fmt.Errorf("digitalocean provider: not yet implemented")
}
