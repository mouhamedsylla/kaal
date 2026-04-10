package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/app/deploy"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/gitutil"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to the target environment (VPS or cloud)",
	Long: `Sync the compose file, pull the image, and restart services on the remote target.

The target is read from pilot.yaml (environments.<env>.target).
Use 'pilot push' first to build and push the image, then 'pilot deploy' to
deploy that exact version. The same image can be deployed multiple times
(e.g. staging then prod) without rebuilding.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringP("tag", "t", "", "image tag to deploy (default: git short SHA)")
	deployCmd.Flags().String("target", "", "override target from pilot.yaml")
	deployCmd.Flags().StringP("strategy", "s", "rolling", "deployment strategy (rolling)")
	deployCmd.Flags().Bool("dry-run", false, "show what would happen without executing")
	deployCmd.Flags().Bool("no-rollback", false, "skip auto-rollback on healthcheck failure")
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	targetOverride, _ := cmd.Flags().GetString("target")
	strategy, _ := cmd.Flags().GetString("strategy")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if strategy != "" && strategy != "rolling" {
		ui.Warn(fmt.Sprintf("Strategy %q not yet implemented — falling back to rolling update", strategy))
	}

	if tag == "" {
		var err error
		if tag, err = gitutil.ShortSHA(); err != nil {
			ui.Fatal(err)
		}
	}

	activeEnv := env.Active(currentEnv)

	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	// Resolve target name.
	targetName := targetOverride
	if targetName == "" {
		if envCfg, ok := cfg.Environments[activeEnv]; ok {
			targetName = envCfg.Target
		}
	}
	if targetName == "" {
		ui.Fatal(fmt.Errorf(
			"no deploy target for environment %q — set environments.%s.target in pilot.yaml",
			activeEnv, activeEnv,
		))
	}

	// Wire provider (domain.DeployProvider).
	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		ui.Fatal(err)
	}

	// Wire secret manager (optional — satisfies domain.SecretManager).
	var secretRefs map[string]string
	var secrets domain.SecretManager
	if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.Secrets != nil && len(envCfg.Secrets.Refs) > 0 {
		secretRefs = envCfg.Secrets.Refs
		secrets, err = runtime.NewSecretManager(envCfg.Secrets.Provider)
		if err != nil {
			ui.Fatal(err)
		}
	}

	stateDir := filepath.Join(".", ".pilot")
	_ = os.MkdirAll(stateDir, 0755)

	uc := deploy.New(provider, secrets, stateDir)

	out, err := uc.Execute(cmd.Context(), deploy.Input{
		Env:        activeEnv,
		Tag:        tag,
		SecretRefs: secretRefs,
		DryRun:     dryRun,
	})
	if err != nil {
		ui.Fatal(err)
	}

	if dryRun {
		ui.Bold("Dry run — nothing was executed")
		for _, step := range out.DryRunPlan.Steps {
			ui.Dim("  → " + step)
		}
		return nil
	}

	ui.Success(fmt.Sprintf("Deployed %s:%s → %s", cfg.Registry.Image, out.Tag, targetName))
	for _, s := range out.Statuses {
		health := s.Health
		if health == "" {
			health = s.State
		}
		ui.Dim(fmt.Sprintf("  %-20s %s", s.Name, health))
	}
	fmt.Println()
	ui.Dim(fmt.Sprintf("  pilot logs --env %s --follow", activeEnv))
	ui.Dim(fmt.Sprintf("  pilot status --env %s", activeEnv))
	return nil
}
