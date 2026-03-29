// Package scaffold implements kaal init — project initialization and file generation.
package scaffold

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Run is the main entrypoint for kaal init.
// It handles prompts (unless --yes is set), generates files, and prints a summary.
func Run(name string, flags Flags) error {
	var opts Options
	var err error

	if flags.Yes {
		opts = buildDefaultOptions(name, flags)
	} else {
		opts, err = RunPrompt(name)
		if err != nil {
			return err
		}
		// Apply CLI flag overrides on top of prompt results
		applyFlagOverrides(&opts, flags)
	}

	opts.applyDefaults()

	// Check destination doesn't already have kaal.yaml
	kaalYAML := filepath.Join(opts.OutputDir, "kaal.yaml")
	if _, err := os.Stat(kaalYAML); err == nil {
		return fmt.Errorf("%s already exists in %s — run 'kaal init' in an empty directory", "kaal.yaml", opts.OutputDir)
	}

	ui.Info(fmt.Sprintf("Scaffolding %s (%s) in ./%s", opts.Name, opts.Stack, opts.OutputDir))

	if err := Generate(opts); err != nil {
		return fmt.Errorf("scaffold failed: %w", err)
	}

	printSummary(opts)
	return nil
}

// Flags mirrors the CLI flags for kaal init.
type Flags struct {
	Stack    string
	Registry string
	Yes      bool
}

func buildDefaultOptions(name string, flags Flags) Options {
	if name == "" {
		name = "my-app"
	}
	stack := flags.Stack
	if stack == "" {
		stack = "go"
	}
	registry := flags.Registry
	if registry == "" {
		registry = "ghcr"
	}
	return Options{
		Name:         name,
		Stack:        stack,
		Registry:     registry,
		Environments: []string{"dev", "staging", "prod"},
	}
}

func applyFlagOverrides(opts *Options, flags Flags) {
	if flags.Stack != "" {
		opts.Stack = flags.Stack
	}
	if flags.Registry != "" {
		opts.Registry = flags.Registry
	}
}

func printSummary(opts Options) {
	fmt.Println()
	ui.Success(fmt.Sprintf("Project %q created in ./%s", opts.Name, opts.OutputDir))
	fmt.Println()

	files := []string{
		"kaal.yaml",
		"Dockerfile",
		"docker-compose.dev.yml",
		"docker-compose.prod.yml",
		".env.dev",
		".gitignore",
	}
	for _, f := range files {
		ui.Dim(fmt.Sprintf("  + %s/%s", opts.OutputDir, f))
	}

	fmt.Println()
	ui.Bold("Next steps:")
	ui.Dim(fmt.Sprintf("  cd %s", opts.OutputDir))
	ui.Dim("  # Edit kaal.yaml — fill in your registry image and target hosts")
	ui.Dim("  kaal up")
}
