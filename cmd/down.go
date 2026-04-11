package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/app/up"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
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

func runDown(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := env.Active(currentEnv)

	provider, err := runtime.NewExecutionProvider(cfg, activeEnv)
	if err != nil {
		ui.Fatal(err)
	}

	uc := up.NewDown(provider)
	out, err := uc.Execute(cmd.Context(), up.DownInput{
		Env:    activeEnv,
		Config: cfg,
	})
	if err != nil {
		ui.Fatal(err)
	}

	ui.Success(fmt.Sprintf("Environment %q stopped", out.Env))
	return nil
}
