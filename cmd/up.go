package cmd

import (
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the local environment",
	Long:  `Starts services via Docker Compose (or k8s) for the active environment.`,
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolP("build", "b", false, "force rebuild images before starting")
	upCmd.Flags().BoolP("detach", "d", true, "run in background (default true)")
}

func runUp(cmd *cobra.Command, args []string) error {
	// Implementation: orchestrator.New(cfg, env).Up() — to be wired up
	return nil
}
