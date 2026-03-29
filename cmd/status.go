package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/status"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the state of services (local or remote based on active env)",
	Long: `Show the runtime state of all services for the active environment.

For local environments (no target): queries docker compose ps.
For remote environments (with a target): connects via SSH and queries the VPS.`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, _ []string) error {
	if err := status.Run(cmd.Context(), status.Options{
		Env:     currentEnv,
		JSONOut: jsonOutput,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
