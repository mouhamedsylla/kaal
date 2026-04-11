// Package logs implements the pilot logs use case.
//
// LogsUseCase streams log lines from either the local runtime (ExecutionProvider)
// or the remote deployment target (DeployProvider). cmd/ selects the right
// provider based on the env config.
package logs

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Input is the data required to stream logs.
type Input struct {
	Env     string
	Service string // empty = all services
	Follow  bool
	Since   string
	Lines   int
	Config  *config.Config
}

// Output is the result of a successful logs call.
type Output struct {
	Lines <-chan string
}

// LogsUseCase streams log output from local or remote providers.
// Exactly one of local or remote must be non-nil.
type LogsUseCase struct {
	local  domain.ExecutionProvider
	remote domain.DeployProvider
}

// New constructs a LogsUseCase for a local environment.
func New(local domain.ExecutionProvider) *LogsUseCase {
	return &LogsUseCase{local: local}
}

// NewRemote constructs a LogsUseCase for a remote environment.
func NewRemote(remote domain.DeployProvider) *LogsUseCase {
	return &LogsUseCase{remote: remote}
}

// Execute starts streaming logs and returns a channel of log lines.
// The channel is closed when streaming ends or ctx is cancelled.
func (uc *LogsUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if _, ok := in.Config.Environments[in.Env]; !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	opts := domain.LogOptions{
		Follow: in.Follow,
		Since:  in.Since,
		Lines:  in.Lines,
	}

	if uc.remote != nil {
		ch, err := uc.remote.Logs(ctx, in.Env, in.Service, opts)
		if err != nil {
			return Output{}, fmt.Errorf("remote logs: %w", err)
		}
		return Output{Lines: ch}, nil
	}

	ch, err := uc.local.Logs(ctx, in.Env, in.Service, opts)
	if err != nil {
		return Output{}, fmt.Errorf("logs: %w\n  Is the environment running? Try 'pilot up'", err)
	}
	return Output{Lines: ch}, nil
}
