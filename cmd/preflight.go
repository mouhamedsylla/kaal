package cmd

import (
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/preflight"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Verify all prerequisites before push or deploy",
	Long: `Run a complete pre-flight check for the target operation.

Returns a structured report telling you — and your AI agent — exactly what
needs to be fixed before proceeding:

  [HUMAN]  actions you must perform (set env vars, add SSH key, etc.)
  [AGENT]  actions your AI agent can perform automatically

Examples:
  kaal preflight --target push
  kaal preflight --target deploy --env prod`,
	RunE: runPreflight,
}

func init() {
	preflightCmd.Flags().StringP("target", "t", "deploy", "target operation: up | push | deploy")
}

func runPreflight(cmd *cobra.Command, _ []string) error {
	targetStr, _ := cmd.Flags().GetString("target")
	activeEnv := preflight.ActiveEnv(currentEnv)

	target := preflight.Target(targetStr)
	switch target {
	case preflight.TargetUp, preflight.TargetPush, preflight.TargetDeploy:
	default:
		return fmt.Errorf("unknown target %q — use: up | push | deploy", targetStr)
	}

	// When --target deploy is used without an explicit --env, the active env may be
	// a local environment (e.g. dev) with no deploy target. Auto-detect the first
	// remote env so the user doesn't have to know to pass --env prod.
	if target == preflight.TargetDeploy && currentEnv == "" {
		if detected := preflight.DetectRemoteEnv(activeEnv); detected != "" && detected != activeEnv {
			ui.Dim(fmt.Sprintf("  note: env %q has no deploy target — using %q instead (pass --env to override)", activeEnv, detected))
			fmt.Println()
			activeEnv = detected
		}
	}

	ui.Info(fmt.Sprintf("Pre-flight checks — target: %s → env: %s", target, activeEnv))
	fmt.Println()

	report, err := preflight.Run(cmd.Context(), target, activeEnv)
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Println(report.JSON())
		return nil
	}

	printPreflightReport(report)
	return nil
}

func printPreflightReport(r *preflight.Report) {
	for _, c := range r.Checks {
		switch c.Status {
		case preflight.StatusOK:
			ui.Success(fmt.Sprintf("%-20s %s", c.Name, c.Message))
		case preflight.StatusWarning:
			ui.Warn(fmt.Sprintf("%-20s %s", c.Name, c.Message))
		case preflight.StatusError:
			ui.Error(fmt.Sprintf("%-20s %s", c.Name, c.Message))
			if c.HumanInstruction != "" {
				for _, line := range splitLines(c.HumanInstruction) {
					ui.Dim("  " + line)
				}
			}
			if c.AgentTool != "" {
				ui.Dim(fmt.Sprintf("  → Agent: call %s", c.AgentTool))
			}
		case preflight.StatusSkipped:
			ui.Dim(fmt.Sprintf("  %-20s skipped", c.Name))
		}
	}

	fmt.Println()

	if r.AllOK {
		ui.Success(fmt.Sprintf("All checks passed — ready to %s", r.Target))
	} else {
		ui.Error(fmt.Sprintf("%d blocker(s) found", r.BlockerCount))
	}

	fmt.Println()
	ui.Bold("  Next steps:")
	for i, step := range r.NextSteps {
		ui.Dim(fmt.Sprintf("  %d. %s", i+1, step))
	}
	fmt.Println()
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
