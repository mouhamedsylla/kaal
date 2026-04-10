package cmd

import (
	"fmt"

	pilotctx "github.com/mouhamedsylla/pilot/internal/mcp/context"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/mcp"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server commands",
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server (JSON-RPC 2.0 over stdio)",
	Long: `Starts pilot as an MCP server. AI clients (Claude Code, Cursor) connect
via stdio transport. Add .mcp.json to your project root to enable it.`,
	RunE: runMCPServe,
}

var mcpContextCmd = &cobra.Command{
	Use:   "context",
	Short: "Print the full project context for AI agents",
	Long: `Print the complete project context as a ready-to-use AI agent prompt.

Includes: pilot.yaml, file tree, detected stack, existing infra files,
service definitions, and explicit instructions for what needs to be generated.

Use 'pilot mcp serve' for automatic context delivery via the MCP protocol
(Claude Code, Cursor, etc.).`,
	RunE: runMCPContext,
}

var mcpContextSummaryFlag bool

func init() {
	mcpContextCmd.Flags().BoolVar(&mcpContextSummaryFlag, "summary", false, "print a short summary instead of the full agent prompt")
	mcpCmd.AddCommand(mcpServeCmd, mcpContextCmd)
}

func runMCPServe(cmd *cobra.Command, _ []string) error {
	return mcp.NewServer().Serve(cmd.Context())
}

func runMCPContext(_ *cobra.Command, _ []string) error {
	activeEnv := pilotenv.Active(currentEnv)
	projCtx, err := pilotctx.Collect(activeEnv)
	if err != nil {
		ui.Fatal(err)
	}
	if mcpContextSummaryFlag {
		fmt.Print(projCtx.Summary())
		return nil
	}
	fmt.Print(projCtx.AgentPrompt())
	return nil
}
