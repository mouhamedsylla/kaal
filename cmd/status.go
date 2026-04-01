package cmd

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/providers/vps"
	"github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/internal/status"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the state of services (local or remote based on active env)",
	Long: `Show the runtime state of all services for the active environment.

For local environments (no target): queries docker compose ps.
For remote environments (with a target): connects via SSH and queries the VPS.

Use --history to show deployment history instead of current state.`,
	RunE: runStatus,
}

var statusHistoryFlag bool

func init() {
	statusCmd.Flags().BoolVar(&statusHistoryFlag, "history", false, "show deployment history instead of current state")
}

func runStatus(cmd *cobra.Command, _ []string) error {
	if statusHistoryFlag {
		return runStatusHistory(cmd.Context())
	}
	if err := status.Run(cmd.Context(), status.Options{
		Env:     currentEnv,
		JSONOut: jsonOutput,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}

func runStatusHistory(ctx context.Context) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := pilotenv.Active(currentEnv)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}
	if envCfg.Target == "" {
		return fmt.Errorf("pilot status --history only applies to remote environments (no target for %q)", activeEnv)
	}

	provider, err := runtime.NewProvider(cfg, envCfg.Target)
	if err != nil {
		return err
	}

	type historian interface {
		ReadHistory(ctx context.Context) ([]vps.DeployRecord, error)
	}
	h, ok := provider.(historian)
	if !ok {
		return fmt.Errorf("target %q does not support deployment history yet", envCfg.Target)
	}

	records, err := h.ReadHistory(ctx)
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}

	if len(records) == 0 {
		ui.Dim("No deployments recorded yet.")
		return nil
	}

	fmt.Printf("\nDeployment history — %s → %s\n\n", activeEnv, envCfg.Target)
	fmt.Printf("  %-10s  %-10s  %-20s  %s\n", "STATUS", "TAG", "DATE", "NOTE")
	fmt.Printf("  %-10s  %-10s  %-20s  %s\n", "──────", "───", "────", "────")
	for _, r := range records {
		s := ui.GreenText("✓ ok")
		if !r.OK {
			s = ui.RedText("✗ failed")
		}
		at := r.At.Local().Format("2006-01-02 15:04:05")
		fmt.Printf("  %-10s  %-10s  %-20s  %s\n", s, r.Tag, at, r.Message)
	}
	fmt.Printf("\n  %d deployment(s) recorded\n\n", len(records))
	return nil
}
