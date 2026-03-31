package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/mouhamedsylla/kaal/internal/version"
	"github.com/mouhamedsylla/kaal/pkg/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update kaal to the latest version",
	Long: `Check for a new version of kaal on GitHub and update if one is available.

Update strategy (in order):
  1. If 'go' is available → go install github.com/mouhamedsylla/kaal@latest
  2. Otherwise           → re-run the official install script

Use --check to only show the latest version without updating.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().Bool("check", false, "only check for a new version, do not update")
	updateCmd.Flags().Bool("force", false, "update even if already on the latest version")
	rootCmd.AddCommand(updateCmd)
}

const githubRepo = "mouhamedsylla/kaal"

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")

	current := version.Version

	// Fetch latest release from GitHub.
	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("could not reach GitHub to check for updates: %w\n  Check your internet connection or visit https://github.com/%s/releases", err, githubRepo)
	}

	ui.Info(fmt.Sprintf("Current version : %s", current))
	ui.Info(fmt.Sprintf("Latest version  : %s", latest.TagName))

	upToDate := current == latest.TagName || (current != "dev" && current >= latest.TagName)

	if upToDate && !force {
		ui.Success("Already on the latest version.")
		return nil
	}

	if current == "dev" {
		ui.Dim("  (running a local dev build — updating to latest release)")
	}

	if checkOnly {
		ui.Info(fmt.Sprintf("New version available: %s", latest.TagName))
		ui.Dim(fmt.Sprintf("  Release notes: %s", latest.HTMLURL))
		ui.Dim("  Run 'kaal update' to install it.")
		return nil
	}

	// Choose update strategy.
	if goPath, err := exec.LookPath("go"); err == nil {
		return updateViaGo(cmd.Context(), goPath, latest.TagName)
	}

	return updateViaScript(cmd.Context(), latest.TagName)
}

// updateViaGo runs: go install github.com/mouhamedsylla/kaal@<tag>
func updateViaGo(ctx context.Context, goPath, tag string) error {
	pkg := fmt.Sprintf("github.com/%s@%s", githubRepo, tag)
	ui.Info(fmt.Sprintf("Installing via go: %s", pkg))

	goCmd := exec.CommandContext(ctx, goPath, "install", pkg)
	goCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr

	if err := goCmd.Run(); err != nil {
		return fmt.Errorf("go install failed: %w\n  Try manually: go install %s", err, pkg)
	}

	ui.Success(fmt.Sprintf("kaal updated to %s", tag))
	ui.Dim("  Restart your shell or run: hash -r")
	return nil
}

// updateViaScript re-runs the official install.sh.
func updateViaScript(ctx context.Context, tag string) error {
	scriptURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/install.sh", githubRepo)
	ui.Info(fmt.Sprintf("Go not found — installing via script: %s", scriptURL))

	// Detect shell.
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	curlPath, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf(
			"neither 'go' nor 'curl' found — cannot auto-update\n"+
				"  Install manually: https://github.com/%s/releases/tag/%s",
			githubRepo, tag,
		)
	}

	// curl -fsSL <url> | sh
	curlCmd := exec.CommandContext(ctx, curlPath, "-fsSL", scriptURL)
	shellCmd := exec.CommandContext(ctx, shell)

	pipe, err := curlCmd.StdoutPipe()
	if err != nil {
		return err
	}
	shellCmd.Stdin = pipe
	shellCmd.Stdout = os.Stdout
	shellCmd.Stderr = os.Stderr

	if err := curlCmd.Start(); err != nil {
		return fmt.Errorf("curl failed: %w", err)
	}
	if err := shellCmd.Run(); err != nil {
		return fmt.Errorf("install script failed: %w", err)
	}
	if err := curlCmd.Wait(); err != nil {
		return fmt.Errorf("curl failed: %w", err)
	}

	ui.Success(fmt.Sprintf("kaal updated to %s", tag))
	return nil
}

// fetchLatestRelease calls the GitHub Releases API.
func fetchLatestRelease() (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("kaal/%s (%s/%s)", version.Version, runtime.GOOS, runtime.GOARCH))

	// Use GITHUB_TOKEN if available to avoid rate limiting.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases found yet at github.com/%s", githubRepo)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}

	// Normalize tag: some repos use "v0.1.0", others "0.1.0".
	rel.TagName = strings.TrimSpace(rel.TagName)
	return &rel, nil
}
