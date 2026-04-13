package cmd

import (
	"os"

	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/local"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/scaffold"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [project-name]",
	Short: "Initialize pilot in a new or existing project",
	Long: `Launch an interactive wizard to describe your infrastructure.
Generates pilot.yaml — the single source of truth for all environments.

Works on a fresh directory or an existing project (pilot detects your stack).
Does NOT generate Dockerfiles — pilot up handles that at runtime.`,
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

	// Attempt registry authentication right after init.
	// Non-blocking — failure is a warning, not an error.
	cfg, err := config.Load(".")
	if err != nil || cfg.Registry.Provider == "" {
		return nil
	}

	// Load .env.local into the process environment so Login() can read the
	// credentials that were just written by the wizard.
	if vars, err := local.ListFile(".env.local"); err == nil {
		for k, v := range vars {
			if os.Getenv(k) == "" { // don't override vars already in the shell
				_ = os.Setenv(k, v)
			}
		}
	}

	reg, err := runtime.NewRegistryProvider(cfg)
	if err != nil {
		return nil
	}

	ui.Info("Authenticating to " + cfg.Registry.Provider + "...")
	if err := reg.Login(cmd.Context()); err != nil {
		ui.Warn("Registry login failed (you can run it manually later): " + err.Error())
	} else {
		ui.Success("Logged in to " + cfg.Registry.Provider)
	}

	return nil
}
