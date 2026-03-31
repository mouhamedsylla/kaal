// Package status implements the pilot status command logic.
package status

import (
	"context"
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot status behaviour.
type Options struct {
	Env     string
	JSONOut bool
}

// serviceRow is a display-agnostic summary of one service.
type serviceRow struct {
	Name   string
	State  string
	Health string
	Info   string // ports (local) or image version (remote)
}

// Run executes pilot status.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}

	fmt.Println()
	ui.Bold(fmt.Sprintf("Environment: %s", activeEnv))
	fmt.Println()

	if envCfg.Target != "" {
		return runRemoteStatus(ctx, cfg, activeEnv, envCfg.Target, opts)
	}
	return runLocalStatus(ctx, cfg, activeEnv, opts)
}

func runLocalStatus(ctx context.Context, cfg *config.Config, activeEnv string, opts Options) error {
	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return err
	}

	statuses, err := orch.Status(ctx)
	if err != nil {
		return fmt.Errorf("local status: %w\n  Is the environment running? Try 'pilot up'", err)
	}

	if len(statuses) == 0 {
		ui.Dim("  No services running — try 'pilot up'")
		fmt.Println()
		return nil
	}

	var rows []serviceRow
	for _, s := range statuses {
		ports := strings.Join(s.Ports, ", ")
		if ports == "" {
			ports = "—"
		}
		rows = append(rows, serviceRow{
			Name:   s.Name,
			State:  s.State,
			Health: s.Health,
			Info:   ports,
		})
	}

	if opts.JSONOut {
		return ui.JSON(statuses)
	}
	printTable(rows, "PORTS")
	return nil
}

func runRemoteStatus(ctx context.Context, cfg *config.Config, activeEnv, targetName string, opts Options) error {
	target := cfg.Targets[targetName]
	ui.Dim(fmt.Sprintf("Target: %s (%s@%s)", targetName, target.User, target.Host))
	fmt.Println()

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
	}

	statuses, err := provider.Status(ctx, activeEnv)
	if err != nil {
		return fmt.Errorf("remote status: %w", err)
	}

	if len(statuses) == 0 {
		ui.Dim("  No services found — have you deployed? Try 'pilot deploy'")
		fmt.Println()
		return nil
	}

	var rows []serviceRow
	for _, s := range statuses {
		rows = append(rows, serviceRow{
			Name:   s.Name,
			State:  s.State,
			Health: s.Health,
			Info:   s.Version,
		})
	}

	if opts.JSONOut {
		return ui.JSON(statuses)
	}
	printTable(rows, "VERSION")
	return nil
}

func printTable(rows []serviceRow, infoHeader string) {
	ui.Dim(fmt.Sprintf("  %-20s %-12s %-12s %s", "SERVICE", "STATE", "HEALTH", infoHeader))
	ui.Dim("  " + strings.Repeat("─", 62))
	for _, r := range rows {
		health := r.Health
		if health == "" {
			health = "—"
		}
		info := r.Info
		if info == "" {
			info = "—"
		}
		line := fmt.Sprintf("  %-20s %-12s %-12s %s", r.Name, r.State, health, info)
		switch {
		case r.State == "running" && r.Health != "unhealthy":
			ui.Success(line)
		case r.State == "exited" || r.Health == "unhealthy":
			ui.Error(line)
		default:
			ui.Warn(line)
		}
	}
	fmt.Println()
}
