package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mouhamedsylla/pilot/internal/app/push"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Build and push the Docker image to the configured registry",
	Long: `Build the Docker image and push it to the registry configured in pilot.yaml.

Tag defaults to the current git short SHA. The same tag is then passed to
'pilot deploy' to deploy that exact image version.`,
	RunE: runPush,
}

func init() {
	pushCmd.Flags().StringP("tag", "t", "", "image tag (default: git short SHA)")
	pushCmd.Flags().BoolP("no-cache", "n", false, "disable Docker build cache")
	pushCmd.Flags().StringSlice("platform", []string{}, "target platforms (e.g. linux/amd64,linux/arm64)")
	pushCmd.Flags().Bool("force", false, "skip compile-time var gap check")
}

func runPush(cmd *cobra.Command, _ []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	platforms, _ := cmd.Flags().GetStringSlice("platform")
	force, _ := cmd.Flags().GetBool("force")

	cfg, err := config.Load(".")
	if err != nil {
		ui.Fatal(err)
	}

	activeEnv := env.Active(currentEnv)

	provider, err := runtime.NewRegistryProvider(cfg)
	if err != nil {
		ui.Fatal(err)
	}

	uc := push.New(provider)

	if len(platforms) == 0 {
		platforms = nil // let the use case decide the default
	}

	out, err := uc.Execute(cmd.Context(), push.Input{
		Env:       activeEnv,
		Tag:       tag,
		NoCache:   noCache,
		Platforms: platforms,
		Force:     force,
		Config:    cfg,
	})
	if err != nil {
		ui.Fatal(err)
	}

	if out.ARMDetected {
		ui.Info("Detected macOS ARM64 — building for linux/amd64 (VPS target)")
		ui.Dim("  Pass --platform linux/arm64 if your VPS is ARM-based")
	}

	ui.Info(fmt.Sprintf("Building %s [%s]", out.Image, strings.Join(platforms, ",")))
	fmt.Println()
	ui.Success(fmt.Sprintf("Pushed %s", out.Image))
	fmt.Println()
	ui.Dim(fmt.Sprintf("  pilot deploy --env prod --tag %s", out.Tag))
	fmt.Println()
	return nil
}
