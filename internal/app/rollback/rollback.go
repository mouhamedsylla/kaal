// Package rollback implements the pilot rollback use case.
package rollback

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Input is the data required to roll back a deployment.
type Input struct {
	Env        string
	Version    string // explicit tag to roll back to; empty = previous deployment
	TargetName string // override target from pilot.yaml; empty = use env config
	Config     *config.Config
}

// Output is the result of a successful rollback.
type Output struct {
	RestoredTag string
	TargetName  string
	TargetHost  string
}

// RollbackUseCase rolls back to a previous deployment.
type RollbackUseCase struct {
	provider domain.DeployProvider
}

// New constructs a RollbackUseCase.
func New(provider domain.DeployProvider) *RollbackUseCase {
	return &RollbackUseCase{provider: provider}
}

// Execute runs the rollback.
func (uc *RollbackUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	envCfg, ok := in.Config.Environments[in.Env]
	if !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	targetName := in.TargetName
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return Output{}, fmt.Errorf(
			"no deploy target for environment %q\n  pilot rollback only applies to remote environments",
			in.Env,
		)
	}

	restoredTag, err := uc.provider.Rollback(ctx, in.Env, in.Version)
	if err != nil {
		return Output{}, fmt.Errorf("rollback: %w", err)
	}

	targetHost := ""
	if t, ok := in.Config.Targets[targetName]; ok {
		targetHost = t.Host
	}

	return Output{
		RestoredTag: restoredTag,
		TargetName:  targetName,
		TargetHost:  targetHost,
	}, nil
}
