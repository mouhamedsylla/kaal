package cmd

import (
	"fmt"

	kaalctx "github.com/mouhamedsylla/kaal/internal/context"
	kaalenv "github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print the full project context for AI agents",
	Long: `Print the complete project context as a ready-to-use AI agent prompt.

Includes: kaal.yaml, file tree, detected stack, existing infra files,
service definitions, and explicit instructions for what needs to be generated.

Paste this into any AI chat, or use 'kaal mcp serve' for automatic context
delivery via the MCP protocol (Claude Code, Cursor, etc.).`,
	RunE: runContext,
}

var contextSummaryFlag bool

func init() {
	contextCmd.Flags().BoolVar(&contextSummaryFlag, "summary", false, "print a short summary instead of the full agent prompt")
}

func runContext(cmd *cobra.Command, _ []string) error {
	activeEnv := kaalenv.Active(currentEnv)

	projCtx, err := kaalctx.Collect(activeEnv)
	if err != nil {
		ui.Fatal(err)
	}

	if contextSummaryFlag {
		fmt.Print(projCtx.Summary())
		return nil
	}

	fmt.Print(projCtx.AgentPrompt())
	return nil
}
