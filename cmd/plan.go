package cmd

import (
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/planview"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Show the execution plan for the next deploy without executing",
	Long: `Read pilot.lock and display the full execution plan for the next deploy:
ordered steps, migration tool/command, and the compensation plan (what pilot
would roll back in LIFO order on failure).

Nothing is executed. Requires a valid pilot.lock — run 'pilot preflight' first.

Examples:
  pilot plan
  pilot plan --env prod`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)
}

func runPlan(_ *cobra.Command, _ []string) error {
	activeEnv := pilotenv.Active(currentEnv)

	uc := planview.New()
	out, err := uc.Execute(planview.Input{
		Operation:  "deploy",
		ProjectDir: ".",
		Env:        activeEnv,
	})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("  Execution plan — pilot deploy --env %s\n\n", out.Env)

	if !out.LockFresh {
		ui.Warn("  ⚠  " + out.LockWarning)
		fmt.Println()
	}

	// Steps
	ui.Dim("  Steps")
	ui.Dim("  " + strings.Repeat("─", 50))
	for i, s := range out.Steps {
		rev := ""
		if s.Reversible {
			rev = "  (compensable)"
		}
		fmt.Printf("  [%d] %-16s %s%s\n", i+1, s.Name, s.Description, rev)
	}
	fmt.Println()

	// Compensation plan
	if len(out.Compensation) > 0 {
		ui.Dim("  Compensation plan  (LIFO — executed on failure)")
		ui.Dim("  " + strings.Repeat("─", 50))
		for i, c := range out.Compensation {
			fmt.Printf("  [%d] %-16s %s\n", i+1, c.Step, c.Command)
		}
		fmt.Println()
	} else {
		ui.Dim("  No compensation steps declared (reversible: false).")
		fmt.Println()
	}

	ui.Dim("  To execute this plan:  pilot deploy --env " + out.Env)
	ui.Dim("  To preview only:       pilot deploy --env " + out.Env + " --dry-run")
	fmt.Println()
	return nil
}
