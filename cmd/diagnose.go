package cmd

import (
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/diagnose"
	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Snapshot of the full system state",
	Long: `Run all diagnostic checks and print a structured report.

Checks: Docker, compose files, .env files, declared ports, registry
reachability, SSH key + VPS connectivity, git state, pending suspensions.

Useful before a deploy or when debugging an unexpected failure.`,
	RunE: runDiagnose,
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}

func runDiagnose(cmd *cobra.Command, _ []string) error {
	cfg, _ := config.Load(".") // best-effort — diagnose works even without pilot.yaml

	activeEnv := pilotenv.Active(currentEnv)

	uc := diagnose.New()
	report := uc.Execute(cmd.Context(), cfg, activeEnv)

	printDiagnoseReport(report)
	return nil
}

func printDiagnoseReport(r diagnose.Report) {
	fmt.Println()
	fmt.Printf("  pilot diagnose  (env: %s)\n\n", r.ActiveEnv)

	// Group by category
	categories := []string{"System", "Project", "Ports", "Registry", "SSH", "Git"}
	byCategory := map[string][]diagnose.Check{}
	for _, c := range r.Checks {
		byCategory[c.Category] = append(byCategory[c.Category], c)
	}

	for _, cat := range categories {
		checks, ok := byCategory[cat]
		if !ok || len(checks) == 0 {
			continue
		}
		fmt.Printf("  ─── %s %s\n", cat, strings.Repeat("─", max(0, 40-len(cat))))
		for _, c := range checks {
			if c.OK {
				line := fmt.Sprintf("  ✓  %-30s", c.Name)
				if c.Value != "" {
					line += "  " + c.Value
				}
				ui.Success(line)
			} else {
				line := fmt.Sprintf("  ✗  %-30s", c.Name)
				if c.Issue != "" {
					line += "  " + c.Issue
				}
				ui.Error(line)
			}
		}
		fmt.Println()
	}

	// Pending suspension
	if r.Suspended != nil {
		fmt.Printf("  ─── Pending choice %s\n", strings.Repeat("─", 20))
		ui.Warn(fmt.Sprintf("  ⚠  [%s] %s", r.Suspended.ErrorCode, r.Suspended.Command))
		ui.Dim(fmt.Sprintf("     Suspended: %s", r.Suspended.Since.Local().Format("2006-01-02 15:04:05")))
		ui.Dim("     Run: pilot resume [--answer <option>]")
		fmt.Println()
	}

	// Summary
	ok, fail := 0, 0
	for _, c := range r.Checks {
		if c.OK {
			ok++
		} else {
			fail++
		}
	}
	if fail == 0 {
		ui.Success(fmt.Sprintf("  All %d checks passed", ok))
	} else {
		ui.Warn(fmt.Sprintf("  %d/%d checks passed — %d issue(s) found", ok, ok+fail, fail))
	}
	fmt.Println()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
