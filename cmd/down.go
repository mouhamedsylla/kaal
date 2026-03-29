package cmd

import (
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the local environment",
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolP("volumes", "v", false, "also remove named volumes")
}

func runDown(cmd *cobra.Command, args []string) error {
	// Implementation: orchestrator.New(cfg, env).Down() — to be wired up
	return nil
}
