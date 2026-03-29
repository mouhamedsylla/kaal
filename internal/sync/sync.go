// Package sync implements the kaal sync command logic.
package sync

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Options controls kaal sync behaviour.
type Options struct {
	Env    string
	Target string // override target from kaal.yaml
}

// Run executes kaal sync: copy kaal.yaml + compose files to the remote target.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml", activeEnv)
	}

	targetName := opts.Target
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return fmt.Errorf(
			"no deploy target for environment %q\n  kaal sync only applies to remote environments",
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
	ui.Dim("  kaal.yaml and docker-compose files copied to ~/kaal/ on the remote")
	fmt.Println()

	return nil
}
