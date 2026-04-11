package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/app/rollback"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to the previous (or specified) deployment",
	Long: `Roll back services on the remote target to a previous version.

Without --version, rolls back to the version deployed just before the current one.

Examples:
  pilot rollback                          # back to previous deployment
  pilot rollback --env prod               # rollback prod explicitly
  pilot rollback --version v1.1.0         # rollback to a specific tag`,
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().StringP("version", "v", "", "specific tag to roll back to (default: previous deployment)")
	rollbackCmd.Flags().String("target", "", "override target from pilot.yaml")
}

func runRollback(cmd *cobra.Command, _ []string) error {
	version, _ := cmd.Flags().GetString("version")
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
			"no deploy target for environment %q\n  pilot rollback only applies to remote environments",
			activeEnv,
		))
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		ui.Fatal(err)
	}

	uc := rollback.New(provider)
	out, err := uc.Execute(cmd.Context(), rollback.Input{
		Env:        activeEnv,
		Version:    version,
		TargetName: targetOverride,
		Config:     cfg,
	})
	if err != nil {
		ui.Fatal(err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Rolled back to %s:%s → %s (%s)",
		cfg.Registry.Image, out.RestoredTag, out.TargetName, out.TargetHost))
	fmt.Println()
	ui.Dim(fmt.Sprintf("  pilot status --env %s", activeEnv))
	ui.Dim(fmt.Sprintf("  pilot logs --env %s --follow", activeEnv))
	fmt.Println()
	return nil
}
