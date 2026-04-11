package cmd

import (
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/resume"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a suspended operation after resolving a TypeC error",
	Long: `Resume the last operation that was suspended because pilot needed a choice.

When pilot encounters a situation with multiple valid paths (TypeC error), it
saves the context to .pilot/suspended.json and waits. After you take action,
run "pilot resume" to retry. Use --answer to pick a specific option.

Examples:
  pilot resume                    # show suspended state and use recommended fix
  pilot resume --answer 0         # pick option 0
  pilot resume --answer "pilot setup --env prod"`,
	RunE: runResume,
}

func init() {
	resumeCmd.Flags().StringP("answer", "a", "", "pick an option by index or exact text (default: recommended)")
}

func runResume(cmd *cobra.Command, _ []string) error {
	answer, _ := cmd.Flags().GetString("answer")

	uc := resume.New()
	out, op, err := uc.Resolve(cmd.Context(), resume.Input{Answer: answer})
	if err != nil {
		return err
	}

	fmt.Println()
	ui.Info(fmt.Sprintf("Resuming: %s  [%s]", op.Command, op.ErrorCode))
	ui.Dim(fmt.Sprintf("  Suspended: %s", op.SuspendedAt.Local().Format("2006-01-02 15:04:05")))
	ui.Dim(fmt.Sprintf("  Action:    %s", out.AppliedOption))
	fmt.Println()

	// Extract the base command (before flags/args) to decide what to re-run.
	baseCommand := strings.Fields(op.Command)[0]

	switch baseCommand {
	case "deploy":
		ui.Info("Re-running: pilot deploy")
		// Re-dispatch to the deploy command with the original args.
		return runDeployResume(cmd, op)
	default:
		ui.Warn(fmt.Sprintf("Resume not yet implemented for %q — re-run manually:", baseCommand))
		ui.Dim(fmt.Sprintf("  pilot %s", op.Command))
		_ = resume.ClearSuspension()
	}

	return nil
}

// runDeployResume re-runs pilot deploy with the args stored in the suspended op.
func runDeployResume(cmd *cobra.Command, op *resume.SuspendedOp) error {
	// Reconstruct flags from saved args.
	if env, ok := op.Args["env"]; ok && currentEnv == "" {
		currentEnv = env
	}

	// Clear suspension before re-running — if it fails again, it will re-save.
	if err := resume.ClearSuspension(); err != nil {
		ui.Warn(fmt.Sprintf("Could not clear suspension file: %v", err))
	}

	// Re-use the deploy command's RunE with the saved tag.
	tag := op.Args["tag"]
	dryRun := op.Args["dry_run"] == "true"
	skipLock := op.Args["skip_lock"] == "true"

	// Temporarily override deploy flags from suspended args.
	_ = deployCmd.Flags().Set("tag", tag)
	if dryRun {
		_ = deployCmd.Flags().Set("dry-run", "true")
	}
	if skipLock {
		_ = deployCmd.Flags().Set("skip-lock", "true")
	}

	return deployCmd.RunE(deployCmd, []string{})
}
