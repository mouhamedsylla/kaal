package cmd

import (
	"fmt"

	kaalenv "github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
}

var envUseCmd = &cobra.Command{
	Use:   "use <env>",
	Short: "Switch the active environment",
	Long: `Switch the active environment. Writes .kaal-current-env at the project root.

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
	if err := kaalenv.Use(env); err != nil {
		ui.Fatal(err)
	}
	ui.Success(fmt.Sprintf("Active environment → %s", env))
	ui.Dim(fmt.Sprintf("  Written to %s", kaalenv.StateFilePath()))
	return nil
}

func runEnvCurrent(_ *cobra.Command, _ []string) error {
	fmt.Println(kaalenv.Current())
	return nil
}
