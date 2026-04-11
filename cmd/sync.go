package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	pilotSync "github.com/mouhamedsylla/pilot/internal/app/sync"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local config to the remote target",
	Long: `Copy pilot.yaml and docker-compose files to the remote VPS or cluster.

Useful when you've updated pilot.yaml or a compose file and want to push the
changes without triggering a full redeploy. Idempotent — safe to run anytime.

Note: pilot deploy already runs sync as its first step.`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().String("target", "", "override target from pilot.yaml")
}

func runSync(cmd *cobra.Command, _ []string) error {
	targetOverride, _ := cmd.Flags().GetString("target")

	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := env.Active(currentEnv)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		ui.Fatal(fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv))
	}

	targetName := targetOverride
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		ui.Fatal(fmt.Errorf(
			"no deploy target for environment %q\n  pilot sync only applies to remote environments",
			activeEnv,
		))
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		ui.Fatal(err)
	}

	target := cfg.Targets[targetName]
	ui.Info(fmt.Sprintf("Syncing config to %s (%s@%s)", targetName, target.User, target.Host))

	uc := pilotSync.New(provider)
	out, err := uc.Execute(cmd.Context(), pilotSync.Input{
		Env:            activeEnv,
		TargetOverride: targetOverride,
		Config:         cfg,
	})
	if err != nil {
		ui.Fatal(err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Config synced to %s", out.TargetName))
	ui.Dim("  pilot.yaml, compose files, env files copied to ~/pilot/")
	fmt.Println()
	return nil
}
