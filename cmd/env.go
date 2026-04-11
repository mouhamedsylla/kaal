package cmd

import (
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/envdiff"
	"github.com/mouhamedsylla/pilot/internal/config"
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

var envDiffCmd = &cobra.Command{
	Use:   "diff <env1> <env2>",
	Short: "Compare two environments and report divergences",
	Long: `Show variables, ports, and services that differ between two environments.

Helps catch the classic "works in dev, breaks in prod" issues before they happen.

Examples:
  pilot env diff dev prod
  pilot env diff staging prod`,
	Args: cobra.ExactArgs(2),
	RunE: runEnvDiff,
}

func init() {
	envCmd.AddCommand(envUseCmd, envCurrentCmd, envDiffCmd)
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

func runEnvDiff(_ *cobra.Command, args []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	uc := envdiff.New()
	out, err := uc.Execute(envdiff.Input{
		EnvA:   args[0],
		EnvB:   args[1],
		Config: cfg,
	})
	if err != nil {
		return err
	}

	fmt.Printf("\nenv diff  %s  ↔  %s\n\n", out.EnvA, out.EnvB)

	if !out.HasDiff() {
		ui.Success("  No divergences found — environments are in sync.")
		fmt.Println()
		return nil
	}

	// ── variables ─────────────────────────────────────────────────────────────
	if len(out.OnlyInA)+len(out.OnlyInB)+len(out.EmptyInA)+len(out.EmptyInB) > 0 {
		ui.Dim("  Variables")
		ui.Dim("  " + strings.Repeat("─", 50))
		for _, k := range out.OnlyInA {
			fmt.Printf("  %-40s  only in %s\n", k, out.EnvA)
		}
		for _, k := range out.OnlyInB {
			fmt.Printf("  %-40s  only in %s\n", k, out.EnvB)
		}
		for _, k := range out.EmptyInA {
			fmt.Printf("  %-40s  empty in %s\n", k, out.EnvA)
		}
		for _, k := range out.EmptyInB {
			fmt.Printf("  %-40s  empty in %s\n", k, out.EnvB)
		}
		fmt.Println()
	}

	// ── ports ─────────────────────────────────────────────────────────────────
	if len(out.PortDiffs) > 0 {
		ui.Dim("  Ports")
		ui.Dim("  " + strings.Repeat("─", 50))
		ui.Dim(fmt.Sprintf("  %-20s  %-12s  %-12s", "SERVICE", out.EnvA, out.EnvB))
		for _, d := range out.PortDiffs {
			portA := d.PortA
			if portA == "" {
				portA = "—"
			}
			portB := d.PortB
			if portB == "" {
				portB = "—"
			}
			fmt.Printf("  %-20s  %-12s  %-12s\n", d.Service, portA, portB)
		}
		fmt.Println()
	}

	// ── services ──────────────────────────────────────────────────────────────
	if len(out.ServicesOnlyInA)+len(out.ServicesOnlyInB) > 0 {
		ui.Dim("  Services")
		ui.Dim("  " + strings.Repeat("─", 50))
		for _, s := range out.ServicesOnlyInA {
			fmt.Printf("  %-30s  only in %s compose file\n", s, out.EnvA)
		}
		for _, s := range out.ServicesOnlyInB {
			fmt.Printf("  %-30s  only in %s compose file\n", s, out.EnvB)
		}
		fmt.Println()
	}

	total := len(out.OnlyInA) + len(out.OnlyInB) + len(out.EmptyInA) + len(out.EmptyInB) +
		len(out.PortDiffs) + len(out.ServicesOnlyInA) + len(out.ServicesOnlyInB)
	ui.Warn(fmt.Sprintf("  %d divergence(s) found", total))
	fmt.Println()
	return nil
}
