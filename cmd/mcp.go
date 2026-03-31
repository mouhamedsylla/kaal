package cmd

import (
	"github.com/mouhamedsylla/pilot/internal/mcp"
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

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, _ []string) error {
	return mcp.NewServer().Serve(cmd.Context())
}
