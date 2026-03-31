package cmd

import (
	"fmt"
	"sort"

	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/internal/secrets/local"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets for a pilot environment",
}

// pilot secrets list
var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret keys for the active environment",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(".")
		if err != nil {
			return err
		}
		activeEnv := pilotenv.Active(currentEnv)
		envFile := fmt.Sprintf(".env.%s", activeEnv)

		vars, err := local.ListFile(envFile)
		if err != nil {
			ui.Warn(fmt.Sprintf("No %s file found", envFile))
			return nil
		}

		// Also show refs declared in pilot.yaml for this env
		var refs map[string]string
		if e, ok := cfg.Environments[activeEnv]; ok && e.Secrets != nil {
			refs = e.Secrets.Refs
		}

		keys := make([]string, 0, len(vars))
		for k := range vars {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("\nSecrets for environment %q (%s)\n\n", activeEnv, envFile)
		for _, k := range keys {
			ref := ""
			if refs != nil {
				if r, ok := refs[k]; ok {
					ref = fmt.Sprintf("  ← %s", r)
				}
			}
			fmt.Printf("  %-30s %s\n", k, ref)
		}
		fmt.Println()
		return nil
	},
}

// pilot secrets get KEY
var secretsGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get the value of a secret from the active environment's .env file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeEnv := pilotenv.Active(currentEnv)
		envFile := fmt.Sprintf(".env.%s", activeEnv)

		vars, err := local.ListFile(envFile)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", envFile, err)
		}
		val, ok := vars[args[0]]
		if !ok {
			return fmt.Errorf("key %q not found in %s", args[0], envFile)
		}
		fmt.Println(val)
		return nil
	},
}

// pilot secrets set KEY VALUE
var secretsSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set or update a secret in the active environment's .env file",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		activeEnv := pilotenv.Active(currentEnv)
		envFile := fmt.Sprintf(".env.%s", activeEnv)
		key, value := args[0], args[1]

		if err := local.SetInFile(envFile, key, value); err != nil {
			return fmt.Errorf("cannot write %s: %w", envFile, err)
		}
		ui.Success(fmt.Sprintf("Set %s in %s", key, envFile))
		return nil
	},
}

// pilot secrets inject — resolves all secrets for the env and prints them
var secretsInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Resolve and display all secrets for the active environment",
	Long: `Resolves all secrets declared in pilot.yaml for the active environment
using the configured provider (local, aws_sm, gcp_sm).

Secrets are printed as KEY=VALUE pairs — pipe to a tool or use in scripts.
Values are never written to disk by this command.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		showValues, _ := cmd.Flags().GetBool("show-values")

		cfg, err := config.Load(".")
		if err != nil {
			return err
		}
		activeEnv := pilotenv.Active(currentEnv)
		envCfg, ok := cfg.Environments[activeEnv]
		if !ok {
			return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
		}

		provider := "local"
		var refs map[string]string
		if envCfg.Secrets != nil {
			if envCfg.Secrets.Provider != "" {
				provider = envCfg.Secrets.Provider
			}
			refs = envCfg.Secrets.Refs
		}

		sm, err := runtime.NewSecretManager(provider)
		if err != nil {
			return err
		}
		if refs == nil {
			refs = map[string]string{}
		}

		injected, err := sm.Inject(cmd.Context(), activeEnv, refs)
		if err != nil {
			return fmt.Errorf("inject secrets: %w", err)
		}

		keys := make([]string, 0, len(injected))
		for k := range injected {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fmt.Printf("\nResolved secrets — env: %s, provider: %s\n\n", activeEnv, provider)
		for _, k := range keys {
			if showValues {
				fmt.Printf("  %s=%s\n", k, injected[k])
			} else {
				fmt.Printf("  %s=<redacted>\n", k)
			}
		}
		fmt.Printf("\n  %d secret(s) resolved\n\n", len(keys))
		return nil
	},
}

func init() {
	secretsInjectCmd.Flags().Bool("show-values", false, "Print secret values (use with caution)")

	secretsCmd.AddCommand(secretsListCmd, secretsGetCmd, secretsSetCmd, secretsInjectCmd)
}
