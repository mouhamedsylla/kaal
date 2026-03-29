package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/up"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [services...]",
	Short: "Start the local environment",
	Long: `Start services for the active environment.

kaal up reads kaal.yaml and:
  1. Generates a Dockerfile if none exists (based on detected stack)
  2. Generates docker-compose.<env>.yml if none exists
  3. Starts all services via Docker Compose

Generated files are placed at the project root. Commit them if they look
right, or delete them to let kaal regenerate on the next run.`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().BoolP("build", "b", false, "force image rebuild before starting")
}

func runUp(cmd *cobra.Command, args []string) error {
	build, _ := cmd.Flags().GetBool("build")

	if err := up.Run(cmd.Context(), up.Options{
		Env:      currentEnv,
		Services: args,
		Build:    build,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
