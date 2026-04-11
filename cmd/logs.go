package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	pilotLogs "github.com/mouhamedsylla/pilot/internal/app/logs"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Stream service logs (local or remote based on active env)",
	Long: `Stream log output for a service or all services.

For local environments: streams docker compose logs.
For remote environments: connects via SSH and streams docker compose logs from the VPS.

Examples:
  pilot logs                      # all services, last 100 lines
  pilot logs api                  # api service only
  pilot logs api --follow         # stream in real time
  pilot logs --env prod --follow  # prod logs via SSH`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "stream logs in real time")
	logsCmd.Flags().String("since", "", "show logs since duration or timestamp (e.g. 5m, 1h)")
	logsCmd.Flags().IntP("lines", "n", 100, "number of lines to show")
}

func runLogs(cmd *cobra.Command, args []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	since, _ := cmd.Flags().GetString("since")
	lines, _ := cmd.Flags().GetInt("lines")

	service := ""
	if len(args) > 0 {
		service = args[0]
	}

	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := pilotenv.Active(currentEnv)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		ui.Fatal(fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv))
	}

	in := pilotLogs.Input{
		Env:     activeEnv,
		Service: service,
		Follow:  follow,
		Since:   since,
		Lines:   lines,
		Config:  cfg,
	}

	var uc *pilotLogs.LogsUseCase
	if envCfg.Target != "" {
		target := cfg.Targets[envCfg.Target]
		if service != "" {
			ui.Dim(fmt.Sprintf("Logs: %s/%s → %s (%s)", activeEnv, service, envCfg.Target, target.Host))
		} else {
			ui.Dim(fmt.Sprintf("Logs: %s (all) → %s (%s)", activeEnv, envCfg.Target, target.Host))
		}
		provider, pErr := runtime.NewDeployProvider(cfg, envCfg.Target)
		if pErr != nil {
			ui.Fatal(pErr)
		}
		uc = pilotLogs.NewRemote(provider)
	} else {
		if service != "" {
			ui.Dim(fmt.Sprintf("Logs: %s/%s", activeEnv, service))
		} else {
			ui.Dim(fmt.Sprintf("Logs: %s (all services)", activeEnv))
		}
		provider, pErr := runtime.NewExecutionProvider(cfg, activeEnv)
		if pErr != nil {
			ui.Fatal(pErr)
		}
		uc = pilotLogs.New(provider)
	}
	fmt.Println()

	out, err := uc.Execute(cmd.Context(), in)
	if err != nil {
		ui.Fatal(err)
	}

	for line := range out.Lines {
		fmt.Println(line)
	}
	return nil
}
