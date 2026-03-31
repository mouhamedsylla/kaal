// Package rollback implements the pilot rollback command logic.
package rollback

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot rollback behaviour.
type Options struct {
	Env     string
	Version string // empty = previous deployment (read from VPS state)
	Target  string // override target from pilot.yaml
}

// Run executes pilot rollback.
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

	targetName := opts.Target
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return fmt.Errorf(
			"no deploy target for environment %q\n  pilot rollback only applies to remote environments",
			activeEnv,
		)
	}

	target := cfg.Targets[targetName]

	if opts.Version != "" {
		ui.Info(fmt.Sprintf("Rolling back %s → %s (tag: %s)", activeEnv, targetName, opts.Version))
	} else {
		ui.Info(fmt.Sprintf("Rolling back %s → %s (previous deployment)", activeEnv, targetName))
	}

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
	}

	resolvedTag, err := provider.Rollback(ctx, activeEnv, opts.Version)
	if err != nil {
		return fmt.Errorf("rollback: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Rolled back to %s:%s → %s (%s)", cfg.Registry.Image, resolvedTag, targetName, target.Host))
	fmt.Println()
	ui.Dim(fmt.Sprintf("  pilot status --env %s", activeEnv))
	ui.Dim(fmt.Sprintf("  pilot logs --env %s --follow", activeEnv))
	fmt.Println()

	return nil
}
