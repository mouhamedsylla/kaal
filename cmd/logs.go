package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/logs"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [service]",
	Short: "Stream service logs (local or remote based on active env)",
	Long: `Stream log output for a service or all services.

For local environments: streams docker compose logs.
For remote environments: connects via SSH and streams docker compose logs from the VPS.

Examples:
  kaal logs                      # all services, last 100 lines
  kaal logs api                  # api service only
  kaal logs api --follow         # stream in real time
  kaal logs --env prod --follow  # prod logs via SSH`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLogs,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "stream logs in real time")
	logsCmd.Flags().String("since", "", "show logs since duration or timestamp (e.g. 5m, 1h, 2024-01-15T10:00:00)")
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

	if err := logs.Run(cmd.Context(), logs.Options{
		Env:     currentEnv,
		Service: service,
		Follow:  follow,
		Since:   since,
		Lines:   lines,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
