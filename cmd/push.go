package cmd

import (
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Build and push the Docker image to the configured registry",
	RunE:  runPush,
}

func init() {
	pushCmd.Flags().StringP("tag", "t", "", "image tag (default: git short SHA)")
	pushCmd.Flags().BoolP("no-cache", "n", false, "disable Docker build cache")
	pushCmd.Flags().StringSlice("platform", []string{}, "target platforms (e.g. linux/amd64,linux/arm64)")
}

func runPush(cmd *cobra.Command, args []string) error {
	// Implementation: registry.New(cfg).Login() + Build() + Push() — to be wired up
	return nil
}
