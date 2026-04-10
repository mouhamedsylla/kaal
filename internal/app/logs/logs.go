// Package logs implements the pilot logs command logic.
package logs

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/orchestrator"
	"github.com/mouhamedsylla/pilot/internal/providers"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot logs behaviour.
type Options struct {
	Env     string
	Service string // empty = all services
	Follow  bool
	Since   string
	Lines   int
}

// Run executes pilot logs — streams to stdout until the channel closes or ctx is cancelled.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}

	if envCfg.Target != "" {
		return runRemoteLogs(ctx, cfg, activeEnv, envCfg.Target, opts)
	}
	return runLocalLogs(ctx, cfg, activeEnv, opts)
}

func runLocalLogs(ctx context.Context, cfg *config.Config, activeEnv string, opts Options) error {
	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return err
	}

	if opts.Service != "" {
		ui.Dim(fmt.Sprintf("Logs: %s/%s", activeEnv, opts.Service))
	} else {
		ui.Dim(fmt.Sprintf("Logs: %s (all services)", activeEnv))
	}
	fmt.Println()

	ch, err := orch.Logs(ctx, opts.Service, orchestrator.LogOptions{
		Follow: opts.Follow,
		Since:  opts.Since,
		Lines:  opts.Lines,
	})
	if err != nil {
		return fmt.Errorf("logs: %w\n  Is the environment running? Try 'pilot up'", err)
	}

	stream(ch)
	return nil
}

func runRemoteLogs(ctx context.Context, cfg *config.Config, activeEnv, targetName string, opts Options) error {
	target := cfg.Targets[targetName]

	if opts.Service != "" {
		ui.Dim(fmt.Sprintf("Logs: %s/%s → %s (%s)", activeEnv, opts.Service, targetName, target.Host))
	} else {
		ui.Dim(fmt.Sprintf("Logs: %s (all) → %s (%s)", activeEnv, targetName, target.Host))
	}
	fmt.Println()

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
	}

	ch, err := provider.Logs(ctx, activeEnv, providers.LogOptions{
		Service: opts.Service,
		Follow:  opts.Follow,
		Since:   opts.Since,
		Lines:   opts.Lines,
	})
	if err != nil {
		return fmt.Errorf("remote logs: %w", err)
	}

	stream(ch)
	return nil
}

// stream drains a log channel to stdout until it closes.
func stream(ch <-chan string) {
	for line := range ch {
		fmt.Println(line)
	}
}
