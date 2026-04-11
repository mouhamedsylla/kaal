package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	pilotErr "github.com/mouhamedsylla/pilot/internal/domain/errors"
	"github.com/mouhamedsylla/pilot/internal/version"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	currentEnv string
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "pilot",
	Short: "Dev Environment as Code — from local to production in one command",
	Long: `pilot is a terminal-first, opinionated, AI-native CLI that takes your project
from initialization to production deployment, ensuring local and remote environments
are identical across any cloud provider or bare-metal VPS.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner("")
		return nil
	},
}

// Execute is the entrypoint called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		exitCode := handleError(err)
		os.Exit(exitCode)
	}
}

// handleError displays the error according to its type and returns the exit code.
func handleError(err error) int {
	var pe *pilotErr.PilotError
	if errors.As(err, &pe) {
		switch pe.Type {
		case pilotErr.TypeC:
			printTypeC(pe)
		case pilotErr.TypeD:
			printTypeD(pe)
		default:
			ui.Error(pe.Error())
		}
		return pe.Exit
	}
	// Plain error — print as-is.
	ui.Error(err.Error())
	return pilotErr.ExitGeneral
}

// printTypeC displays a TypeC error: pilot suspends and presents options.
func printTypeC(pe *pilotErr.PilotError) {
	fmt.Println()
	ui.Warn(fmt.Sprintf("⚠  %s", pe.Message))
	fmt.Println()

	if len(pe.Options) > 0 {
		ui.Dim("  Possible actions:")
		for i, opt := range pe.Options {
			marker := "  "
			if opt == pe.Recommended || (pe.Recommended == "" && i == 0) {
				marker = "→ "
			}
			fmt.Printf("  %s[%d] %s\n", marker, i, opt)
		}
		fmt.Println()
		ui.Dim("  After taking action, run:")
		ui.Dim("    pilot resume")
	}

	if pe.AppliesTo != "" {
		fmt.Println()
		ui.Dim(fmt.Sprintf("  Affects: %s", pe.AppliesTo))
	}
	fmt.Println()
}

// printTypeD displays a TypeD error: stop with exact instructions.
func printTypeD(pe *pilotErr.PilotError) {
	fmt.Println()
	ui.Error(fmt.Sprintf("✗  %s", pe.Message))
	fmt.Println()

	if pe.Instructions != "" {
		ui.Dim("  What to do:")
		for _, line := range strings.Split(pe.Instructions, "\n") {
			fmt.Printf("  %s\n", line)
		}
	}

	if pe.Cause != nil && pe.Instructions == "" {
		ui.Dim(fmt.Sprintf("  Cause: %v", pe.Cause))
	}
	fmt.Println()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: pilot.yaml in cwd or parent)")
	rootCmd.PersistentFlags().StringVarP(&currentEnv, "env", "e", "", "target environment (dev, staging, prod)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format (for machine consumption)")

	// setup is absorbed into preflight --fix (Phase 3) — hidden in the meantime.
	setupCmd.Hidden = true

	rootCmd.AddCommand(
		initCmd,
		envCmd,
		upCmd,
		downCmd,
		pushCmd,
		deployCmd,
		rollbackCmd,
		syncCmd,
		statusCmd,
		logsCmd,
		secretsCmd,
		preflightCmd,
		setupCmd,
		resumeCmd,
		mcpCmd,
		versionCmd,
	)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print pilot version information",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Println(version.String())
	},
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
	viper.AutomaticEnv()
}
