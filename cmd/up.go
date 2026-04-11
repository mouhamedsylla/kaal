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

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the local environment",
	Long: `Start services for the active environment.

pilot up reads pilot.yaml and starts all services via Docker Compose.
If the compose file is missing, pilot tells you exactly what to ask your
AI agent to generate.`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().BoolP("build", "b", false, "force image rebuild before starting")
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := env.Active(currentEnv)

	provider, err := runtime.NewExecutionProvider(cfg, activeEnv)
	if err != nil {
		ui.Fatal(err)
	}

	uc := up.New(provider)
	out, err := uc.Execute(cmd.Context(), up.Input{
		Env:      activeEnv,
		Services: args,
		Config:   cfg,
	})
	if err != nil {
		ui.Fatal(err)
	}

	if out.IsRemoteEnv {
		ui.Warn(fmt.Sprintf(
			"Environment %q is configured for remote deployment (target: %s)",
			activeEnv, out.TargetName,
		))
		ui.Dim("  Running it locally requires the image to already exist in the registry.")
		ui.Dim(fmt.Sprintf("  If you haven't pushed yet: pilot push --env %s", activeEnv))
		ui.Dim("  To develop locally, use the dev environment: pilot env use dev && pilot up")
		fmt.Println()
	}

	if out.MissingEnvFile != "" {
		ui.Warn(fmt.Sprintf("%s not found — services may fail to start without required variables", out.MissingEnvFile))
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Environment %q is up", activeEnv))
	printServiceURLs(cfg)
	return nil
}


func printServiceURLs(cfg *config.Config) {
	fmt.Println()
	for name, svc := range cfg.Services {
		if svc.Port > 0 {
			ui.Dim(fmt.Sprintf("  %-14s http://localhost:%d", name, svc.Port))
		}
	}
	fmt.Println()
	ui.Dim("pilot logs --follow   stream logs")
	ui.Dim("pilot down            stop services")
	ui.Dim("pilot status          inspect services")
}
