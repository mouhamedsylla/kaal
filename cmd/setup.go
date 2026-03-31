package cmd

import (
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/providers/vps"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Prepare the remote VPS for pilot deployments",
	Long: `Run one-time setup tasks on the target VPS:
  - Add the deploy user to the docker group
  - Verify docker is accessible without sudo

Requires password-less sudo on the VPS (standard on Hetzner, DigitalOcean, OVH cloud-init).`,
	RunE: runSetup,
}

func init() {
	setupCmd.Flags().Bool("fix-docker", true, "add deploy user to the docker group (default: true)")
}

func runSetup(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(currentEnv)

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}

	targetName := envCfg.Target
	if targetName == "" {
		return fmt.Errorf(
			"no deploy target for environment %q\n  Add: environments.%s.target: <target-name>",
			activeEnv, activeEnv,
		)
	}

	target, ok := cfg.Targets[targetName]
	if !ok {
		return fmt.Errorf("target %q not defined in pilot.yaml", targetName)
	}
	if target.Host == "" {
		return fmt.Errorf(
			"target %q has no host configured\n  Edit pilot.yaml:\n    targets:\n      %s:\n        host: \"YOUR_VPS_IP\"",
			targetName, targetName,
		)
	}
	if target.Type != "vps" && target.Type != "hetzner" {
		return fmt.Errorf("pilot setup only supports vps/hetzner targets (got %q)", target.Type)
	}

	ui.Info(fmt.Sprintf("Setting up %s (%s@%s)", targetName, target.User, target.Host))

	provider := vps.New(cfg, target)

	fixDocker, _ := cmd.Flags().GetBool("fix-docker")
	if fixDocker {
		ui.Info("Adding deploy user to docker group...")
		if err := provider.SetupDockerGroup(cmd.Context()); err != nil {
			return fmt.Errorf("docker setup: %w", err)
		}
		ui.Success(fmt.Sprintf("User %q added to docker group", target.User))
		fmt.Println()
		ui.Dim("  Group changes take effect on the next SSH connection.")
		ui.Dim("  Run pilot deploy — it opens a fresh SSH session automatically.")
		fmt.Println()
	}

	return nil
}
