package k8s

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Orchestrator is a stub — Kubernetes support is not yet implemented.
type Orchestrator struct {
	cfg *config.Config
	env string
}

func New(cfg *config.Config, env string) *Orchestrator {
	return &Orchestrator{cfg: cfg, env: env}
}

func (o *Orchestrator) Up(_ context.Context, _ string, _ []string) error {
	return fmt.Errorf("k8s orchestrator: not yet implemented")
}

func (o *Orchestrator) Down(_ context.Context, _ string) error {
	return fmt.Errorf("k8s orchestrator: not yet implemented")
}

func (o *Orchestrator) Logs(_ context.Context, _ string, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, fmt.Errorf("k8s orchestrator: not yet implemented")
}

func (o *Orchestrator) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, fmt.Errorf("k8s orchestrator: not yet implemented")
}
