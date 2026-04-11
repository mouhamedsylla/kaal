// Package status implements the pilot status use case.
//
// StatusUseCase queries either the local runtime (ExecutionProvider) or the
// remote deployment target (DeployProvider), depending on which one is
// injected. cmd/ selects the right provider based on the env config.
package status

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Input is the data required to query service status.
type Input struct {
	Env    string
	Config *config.Config
}

// Output is the result of a status query.
type Output struct {
	Env      string
	Remote   bool   // true when querying a remote target
	Target   string // non-empty when Remote is true
	Host     string // non-empty when Remote is true
	Statuses []domain.ServiceStatus
}

// StatusUseCase queries service status from local or remote providers.
// Exactly one of local or remote must be non-nil.
type StatusUseCase struct {
	local  domain.ExecutionProvider // nil for remote envs
	remote domain.DeployProvider   // nil for local envs
}

// New constructs a StatusUseCase for a local environment.
func New(local domain.ExecutionProvider) *StatusUseCase {
	return &StatusUseCase{local: local}
}

// NewRemote constructs a StatusUseCase for a remote environment.
func NewRemote(remote domain.DeployProvider) *StatusUseCase {
	return &StatusUseCase{remote: remote}
}

// Execute queries service status.
func (uc *StatusUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if _, ok := in.Config.Environments[in.Env]; !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	if uc.remote != nil {
		return uc.remoteStatus(ctx, in)
	}
	return uc.localStatus(ctx, in)
}

func (uc *StatusUseCase) localStatus(ctx context.Context, in Input) (Output, error) {
	statuses, err := uc.local.Status(ctx, in.Env)
	if err != nil {
		return Output{}, fmt.Errorf("local status: %w\n  Is the environment running? Try 'pilot up'", err)
	}
	return Output{Env: in.Env, Remote: false, Statuses: statuses}, nil
}

func (uc *StatusUseCase) remoteStatus(ctx context.Context, in Input) (Output, error) {
	envCfg := in.Config.Environments[in.Env]
	targetName := envCfg.Target

	statuses, err := uc.remote.Status(ctx, in.Env)
	if err != nil {
		return Output{}, fmt.Errorf("remote status: %w", err)
	}

	targetHost := ""
	if t, ok := in.Config.Targets[targetName]; ok {
		targetHost = t.Host
	}

	return Output{
		Env:      in.Env,
		Remote:   true,
		Target:   targetName,
		Host:     targetHost,
		Statuses: statuses,
	}, nil
}
