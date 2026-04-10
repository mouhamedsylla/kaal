// Package sync implements the pilot sync command logic.
package sync

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot sync behaviour.
type Options struct {
	Env    string
	Target string // override target from pilot.yaml
}

// Run executes pilot sync: copy pilot.yaml + compose files to the remote target.
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
			"no deploy target for environment %q\n  pilot sync only applies to remote environments",
			activeEnv,
		)
	}

	target := cfg.Targets[targetName]
	ui.Info(fmt.Sprintf("Syncing config to %s (%s@%s)", targetName, target.User, target.Host))

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
	}

	if err := provider.Sync(ctx, activeEnv); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Config synced to %s", targetName))
	ui.Dim("  pilot.yaml, compose files, env files and bind-mount config files copied to ~/pilot/")
	fmt.Println()

	return nil
}
