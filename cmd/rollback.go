package cmd

import (
	"github.com/mouhamedsylla/pilot/internal/rollback"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back to the previous (or specified) deployment",
	Long: `Roll back services on the remote target to a previous version.

Without --version, rolls back to the version deployed just before the current one.
pilot tracks the last two deployed tags on the remote in ~/.pilot/<project>/state.

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
	target, _ := cmd.Flags().GetString("target")

	if err := rollback.Run(cmd.Context(), rollback.Options{
		Env:     currentEnv,
		Version: version,
		Target:  target,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
