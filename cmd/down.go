package cmd

import (
	"github.com/mouhamedsylla/pilot/internal/up"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop the local environment",
	Long:  `Stop and remove containers for the active environment.`,
	RunE:  runDown,
}

func init() {
	downCmd.Flags().BoolP("volumes", "v", false, "also remove named volumes (destroys data)")
}

func runDown(cmd *cobra.Command, args []string) error {
	volumes, _ := cmd.Flags().GetBool("volumes")

	if err := up.RunDown(cmd.Context(), up.DownOptions{
		Env:     currentEnv,
		Volumes: volumes,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
