package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/adapters/vps"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/app/status"
	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
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

	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := pilotenv.Active(currentEnv)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		ui.Fatal(fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv))
	}

	fmt.Println()
	ui.Bold(fmt.Sprintf("Environment: %s", activeEnv))
	fmt.Println()

	in := status.Input{Env: activeEnv, Config: cfg}

	var out status.Output
	if envCfg.Target != "" {
		provider, pErr := runtime.NewDeployProvider(cfg, envCfg.Target)
		if pErr != nil {
			ui.Fatal(pErr)
		}
		target := cfg.Targets[envCfg.Target]
		ui.Dim(fmt.Sprintf("Target: %s (%s@%s)", envCfg.Target, target.User, target.Host))
		fmt.Println()
		uc := status.NewRemote(provider)
		out, err = uc.Execute(cmd.Context(), in)
	} else {
		provider, pErr := runtime.NewExecutionProvider(cfg, activeEnv)
		if pErr != nil {
			ui.Fatal(pErr)
		}
		uc := status.New(provider)
		out, err = uc.Execute(cmd.Context(), in)
	}
	if err != nil {
		ui.Fatal(err)
	}

	if len(out.Statuses) == 0 {
		if out.Remote {
			ui.Dim("  No services found — have you deployed? Try 'pilot deploy'")
		} else {
			ui.Dim("  No services running — try 'pilot up'")
		}
		fmt.Println()
		return nil
	}

	if jsonOutput {
		return ui.JSON(out.Statuses)
	}

	if out.Remote {
		printStatusTable(out.Statuses, "VERSION")
	} else {
		printStatusTable(out.Statuses, "PORTS")
	}
	return nil
}

func printStatusTable(statuses []domain.ServiceStatus, infoHeader string) {
	ui.Dim(fmt.Sprintf("  %-20s %-12s %-12s %s", "SERVICE", "STATE", "HEALTH", infoHeader))
	ui.Dim("  " + strings.Repeat("─", 62))
	for _, s := range statuses {
		health := s.Health
		if health == "" {
			health = "—"
		}
		line := fmt.Sprintf("  %-20s %-12s %-12s", s.Name, s.State, health)
		switch {
		case s.State == "running" && s.Health != "unhealthy":
			ui.Success(line)
		case s.State == "exited" || s.Health == "unhealthy":
			ui.Error(line)
		default:
			ui.Warn(line)
		}
	}
	fmt.Println()
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

	provider, err := runtime.NewDeployProvider(cfg, envCfg.Target)
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
