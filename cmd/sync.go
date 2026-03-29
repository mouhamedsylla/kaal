package cmd

import (
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local config to the remote target",
	Long:  `Copies kaal.yaml and compose files to the remote VPS or cluster. Idempotent.`,
	RunE:  runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	// Implementation: providers.New(cfg, target).Sync() — to be wired up
	return nil
}
