package cmd

import (
	"github.com/mouhamedsylla/kaal/internal/push"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Build and push the Docker image to the configured registry",
	Long: `Build the Docker image and push it to the registry configured in kaal.yaml.

Tag defaults to the current git short SHA. The same tag is then passed to
'kaal deploy' to deploy that exact image version.`,
	RunE: runPush,
}

func init() {
	pushCmd.Flags().StringP("tag", "t", "", "image tag (default: git short SHA)")
	pushCmd.Flags().BoolP("no-cache", "n", false, "disable Docker build cache")
	pushCmd.Flags().StringSlice("platform", []string{}, "target platforms (e.g. linux/amd64,linux/arm64)")
}

func runPush(cmd *cobra.Command, _ []string) error {
	tag, _ := cmd.Flags().GetString("tag")
	noCache, _ := cmd.Flags().GetBool("no-cache")
	platforms, _ := cmd.Flags().GetStringSlice("platform")

	if err := push.Run(cmd.Context(), push.Options{
		Env:       currentEnv,
		Tag:       tag,
		NoCache:   noCache,
		Platforms: platforms,
	}); err != nil {
		ui.Fatal(err)
	}
	return nil
}
