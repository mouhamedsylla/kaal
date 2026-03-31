package cmd

import (
	"fmt"

	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
}

var envUseCmd = &cobra.Command{
	Use:   "use <env>",
	Short: "Switch the active environment",
	Long: `Switch the active environment. Writes .pilot-current-env at the project root.

All subsequent commands (up, down, logs, status, deploy...) will use this
environment unless overridden with --env.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runEnvUse,
}

var envCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the active environment",
	RunE:  runEnvCurrent,
}

func init() {
	envCmd.AddCommand(envUseCmd, envCurrentCmd)
}

func runEnvUse(_ *cobra.Command, args []string) error {
	env := args[0]
	if err := pilotenv.Use(env); err != nil {
		ui.Fatal(err)
	}
	ui.Success(fmt.Sprintf("Active environment → %s", env))
	ui.Dim(fmt.Sprintf("  Written to %s", pilotenv.StateFilePath()))
	return nil
}

func runEnvCurrent(_ *cobra.Command, _ []string) error {
	fmt.Println(pilotenv.Current())
	return nil
}
