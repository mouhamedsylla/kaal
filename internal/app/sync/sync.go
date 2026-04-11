// Package sync implements the pilot sync use case.
package sync

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Input is the data required to sync config to a remote target.
type Input struct {
	Env            string
	TargetOverride string // override target from pilot.yaml; empty = use env config
	Config         *config.Config
}

// Output is the result of a successful sync.
type Output struct {
	TargetName string
	TargetHost string
}

// SyncUseCase copies config files to a remote target.
type SyncUseCase struct {
	provider domain.DeployProvider
}

// New constructs a SyncUseCase.
func New(provider domain.DeployProvider) *SyncUseCase {
	return &SyncUseCase{provider: provider}
}

// Execute runs pilot sync.
func (uc *SyncUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	envCfg, ok := in.Config.Environments[in.Env]
	if !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	targetName := in.TargetOverride
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return Output{}, fmt.Errorf(
			"no deploy target for environment %q\n  pilot sync only applies to remote environments",
			in.Env,
		)
	}

	if err := uc.provider.Sync(ctx, in.Env); err != nil {
		return Output{}, fmt.Errorf("sync: %w", err)
	}

	targetHost := ""
	if t, ok := in.Config.Targets[targetName]; ok {
		targetHost = t.Host
	}

	return Output{TargetName: targetName, TargetHost: targetHost}, nil
}
