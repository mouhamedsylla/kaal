package cmd

import (
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to the target environment (VPS or cloud)",
	Long: `Builds and pushes the image, then deploys to the configured target.
Uses SSH + docker compose for VPS targets.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringP("tag", "t", "", "image tag to deploy (default: git short SHA)")
	deployCmd.Flags().StringP("target", "", "", "override target from kaal.yaml")
	deployCmd.Flags().StringP("strategy", "s", "rolling", "deployment strategy (rolling, blue-green, canary)")
	deployCmd.Flags().Bool("dry-run", false, "show what would happen without executing")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Implementation: providers.New(cfg, target).Deploy() — to be wired up
	return nil
}
