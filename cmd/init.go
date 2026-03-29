package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/scaffold"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize kaal in a new or existing project",
	Long: `Launch an interactive wizard to describe your infrastructure.
Generates kaal.yaml — the single source of truth for all environments.

Works on a fresh directory or an existing project (kaal detects your stack).
Does NOT generate Dockerfiles — kaal up handles that at runtime.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("stack", "s", "", "stack override (go, node, python, rust, java)")
	initCmd.Flags().StringP("registry", "r", "", "registry provider (ghcr, dockerhub, custom)")
	initCmd.Flags().BoolP("yes", "y", false, "non-interactive — accept defaults (for CI / agents)")
}

func runInit(cmd *cobra.Command, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	stack, _ := cmd.Flags().GetString("stack")
	registry, _ := cmd.Flags().GetString("registry")
	yes, _ := cmd.Flags().GetBool("yes")

	if err := scaffold.Run(name, scaffold.Flags{
		Stack:    stack,
		Registry: registry,
		Yes:      yes,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
