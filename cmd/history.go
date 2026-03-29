package cmd

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/kaal/internal/config"
	kaalenv "github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/internal/providers/vps"
	"github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show deployment history for the active environment",
	RunE:  runHistory,
}

func runHistory(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := kaalenv.Active(currentEnv)
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml", activeEnv)
	}
	if envCfg.Target == "" {
		return fmt.Errorf("kaal history only applies to remote environments (no target for %q)", activeEnv)
	}

	provider, err := runtime.NewProvider(cfg, envCfg.Target)
	if err != nil {
		return err
	}

	// ReadHistory is VPS-specific — assert the capability.
	type historian interface {
		ReadHistory(ctx context.Context) ([]vps.DeployRecord, error)
	}
	h, ok := provider.(historian)
	if !ok {
		return fmt.Errorf("target %q does not support deployment history yet", envCfg.Target)
	}

	records, err := h.ReadHistory(cmd.Context())
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}

	if len(records) == 0 {
		ui.Dim("No deployments recorded yet.")
		return nil
	}

	fmt.Printf("\nDeployment history — %s → %s\n\n", activeEnv, envCfg.Target)
	fmt.Printf("  %-10s  %-10s  %-20s  %s\n", "STATUS", "TAG", "DATE", "NOTE")
	fmt.Printf("  %-10s  %-10s  %-20s  %s\n", "──────", "───", "────", "────")

	for _, r := range records {
		status := ui.GreenText("✓ ok")
		if !r.OK {
			status = ui.RedText("✗ failed")
		}
		at := r.At.Local().Format("2006-01-02 15:04:05")
		fmt.Printf("  %-10s  %-10s  %-20s  %s\n", status, r.Tag, at, r.Message)
	}
	fmt.Printf("\n  %d deployment(s) recorded\n\n", len(records))
	return nil
}
