package cmd

import (
	"fmt"

	pilotctx "github.com/mouhamedsylla/pilot/internal/context"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print the full project context for AI agents",
	Long: `Print the complete project context as a ready-to-use AI agent prompt.

Includes: pilot.yaml, file tree, detected stack, existing infra files,
service definitions, and explicit instructions for what needs to be generated.

Paste this into any AI chat, or use 'pilot mcp serve' for automatic context
delivery via the MCP protocol (Claude Code, Cursor, etc.).`,
	RunE: runContext,
}

var contextSummaryFlag bool

func init() {
	contextCmd.Flags().BoolVar(&contextSummaryFlag, "summary", false, "print a short summary instead of the full agent prompt")
}

func runContext(cmd *cobra.Command, _ []string) error {
	activeEnv := pilotenv.Active(currentEnv)

	projCtx, err := pilotctx.Collect(activeEnv)
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
