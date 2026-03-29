package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/deploy"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to the target environment (VPS or cloud)",
	Long: `Sync the compose file, pull the image, and restart services on the remote target.

The target is read from kaal.yaml (environments.<env>.target).
Use 'kaal push' first to build and push the image, then 'kaal deploy' to
deploy that exact version. The same image can be deployed multiple times
(e.g. staging then prod) without rebuilding.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringP("tag", "t", "", "image tag to deploy (default: git short SHA)")
	deployCmd.Flags().String("target", "", "override target from kaal.yaml")
	deployCmd.Flags().StringP("strategy", "s", "rolling", "deployment strategy (rolling)")
	deployCmd.Flags().Bool("dry-run", false, "show what would happen without executing")
}

func runDeploy(cmd *cobra.Command, _ []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	target, _ := cmd.Flags().GetString("target")
	strategy, _ := cmd.Flags().GetString("strategy")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if err := deploy.Run(cmd.Context(), deploy.Options{
		Env:      currentEnv,
		Tag:      tag,
		Target:   target,
		Strategy: strategy,
		DryRun:   dryRun,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
