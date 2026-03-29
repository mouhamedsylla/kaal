package cmd

import (
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the complete project state (local and remote)",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Implementation: orchestrator.Status() + provider.Status() — to be wired up
	return nil
}
