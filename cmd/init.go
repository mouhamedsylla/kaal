package cmd

import (
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize a new kaal project",
	Long: `Scaffold a new project with kaal.yaml, Dockerfiles, docker-compose files,
and environment configs. Asks 4 questions then generates everything.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("stack", "s", "", "project stack (go, node, python, rust)")
	initCmd.Flags().StringP("registry", "r", "", "registry provider (ghcr, dockerhub, custom)")
	initCmd.Flags().BoolP("yes", "y", false, "accept all defaults without prompts")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Implementation in internal/scaffold — to be wired up
	return nil
}
