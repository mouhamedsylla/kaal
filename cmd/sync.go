package cmd

import (
	"github.com/mouhamedsylla/pilot/internal/sync"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local config to the remote target",
	Long: `Copy pilot.yaml and docker-compose files to the remote VPS or cluster.

Useful when you've updated pilot.yaml or a compose file and want to push the
changes without triggering a full redeploy. Idempotent — safe to run anytime.

Note: pilot deploy already runs sync as its first step.`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().String("target", "", "override target from pilot.yaml")
}

func runSync(cmd *cobra.Command, _ []string) error {
	target, _ := cmd.Flags().GetString("target")

	if err := sync.Run(cmd.Context(), sync.Options{
		Env:    currentEnv,
		Target: target,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
