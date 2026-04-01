package cmd

import (
	"fmt"
	"os"

	"github.com/mouhamedsylla/pilot/internal/piloterr"
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
	// Display the mascot banner when pilot is called with no subcommand.
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner("")
		return nil
	},
}

// Execute is the entrypoint called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(piloterr.ExitCode(err))
	}
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
