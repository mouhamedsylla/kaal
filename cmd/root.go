package cmd

import (
	"os"

	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile    string
	currentEnv string
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:   "kaal",
	Short: "Dev Environment as Code — from local to production in one command",
	Long: `kaal is a terminal-first, opinionated, AI-native CLI that takes your project
from initialization to production deployment, ensuring local and remote environments
are identical across any cloud provider or bare-metal VPS.`,
	// Display the mascot banner when kaal is called with no subcommand.
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.PrintBanner("")
		return nil
	},
}

// Execute is the entrypoint called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: kaal.yaml in cwd or parent)")
	rootCmd.PersistentFlags().StringVarP(&currentEnv, "env", "e", "", "target environment (dev, staging, prod)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format (for machine consumption)")

	rootCmd.AddCommand(
		initCmd,
		contextCmd,
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
		mcpCmd,
	)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
	viper.AutomaticEnv()
}
