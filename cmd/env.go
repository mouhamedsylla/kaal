package cmd

import (
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
}

var envUseCmd = &cobra.Command{
	Use:   "use <env>",
	Short: "Switch the active environment (dev, staging, prod)",
	Args:  cobra.ExactArgs(1),
	RunE:  runEnvUse,
}

func init() {
	envCmd.AddCommand(envUseCmd)
}

func runEnvUse(cmd *cobra.Command, args []string) error {
	// Implementation: write .kaal-current-env — to be wired up
	return nil
}
