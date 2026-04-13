// Package scaffold implements pilot init — project initialization.
// It collects infrastructure intent via a TUI wizard and writes pilot.yaml.
// It does NOT generate Dockerfiles or compose files — those are generated
// at runtime by pilot up, based on what already exists in the project.
package scaffold

import (
	"fmt"
	"os"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/scaffold/tui"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Flags mirrors the CLI flags available on pilot init.
type Flags struct {
	Stack    string
	Registry string
	Yes      bool
}

// Run is the entrypoint for pilot init.
func Run(name string, flags Flags) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Detect existing project
	detected := Detect(dir)
	if name != "" {
		detected.Name = name
	}

	// Guard: refuse if pilot.yaml already exists (use --force to override later)
	if detected.HasKaalYAML {
		return fmt.Errorf(
			"pilot.yaml already exists in this directory\n" +
				"  Edit it directly, or delete it and re-run pilot init",
		)
	}

	var opts Options

	if flags.Yes {
		opts = defaultOptions(detected, flags)
	} else {
		opts, err = runWizard(detected, flags)
		if err != nil {
			return err
		}
	}

	opts.ApplyDefaults()

	ui.Info(fmt.Sprintf("Generating pilot.yaml for %q", opts.Name))

	if err := Generate(opts); err != nil {
		return fmt.Errorf("scaffold failed: %w", err)
	}

	printSummary(opts)
	return nil
}

func runWizard(detected DetectedProject, flags Flags) (Options, error) {
	result, err := tui.Run(tui.DetectedInfo{
		Name:            detected.Name,
		Stack:           detected.Stack,
		LanguageVersion: detected.LanguageVersion,
		IsExisting:      detected.IsExisting,
		Hints:           detected.Hints,
	})
	if err != nil {
		return Options{}, err
	}
	if result.Cancelled {
		return Options{}, fmt.Errorf("init cancelled")
	}

	opts := Options{
		Name:            result.Name,
		Stack:           result.Stack,
		LanguageVersion: result.LanguageVersion,
		Environments:    result.Environments,
		TargetType:      result.TargetType,
		TargetHost:      result.TargetHost,
		TargetUser:      result.TargetUser,
		TargetSSHKey:    result.TargetSSHKey,
		Registry:        result.Registry,
		RegistryImage:   result.RegistryImage,
		RegistryCreds:   result.RegistryCreds,
		OutputDir:       ".",
	}

	// Merge flag overrides.
	if flags.Stack != "" {
		opts.Stack = flags.Stack
	}
	if flags.Registry != "" {
		opts.Registry = flags.Registry
	}

	// Map wizard service choices to ServiceChoice, applying hosting + provider
	// decisions made in stepManagedServices.
	for _, svc := range result.Services {
		hosting := result.ServiceHosting[svc.Key]
		if hosting == "" {
			hosting = "container" // default for services not in the managed step
		}
		provider := result.ServiceProvider[svc.Key]
		if hosting == "container" {
			provider = "container"
		}

		opts.Services = append(opts.Services, ServiceChoice{
			Name:     svc.Key,
			Type:     svc.Type,
			Port:     defaultPortForService(svc.Key),
			Version:  defaultVersionForService(svc.Key),
			Hosting:  hosting,
			Provider: provider,
		})
	}

	return opts, nil
}

func defaultOptions(detected DetectedProject, flags Flags) Options {
	stack := flags.Stack
	if stack == "" {
		stack = detected.Stack
	}
	if stack == "" {
		stack = "go"
	}
	registry := flags.Registry
	if registry == "" {
		registry = "ghcr"
	}
	name := detected.Name
	if name == "" {
		name = "my-app"
	}
	return Options{
		Name:         name,
		Stack:        stack,
		Registry:     registry,
		Environments: []string{"dev", "staging", "prod"},
		TargetType:   "vps",
		Services: []ServiceChoice{
			{Name: "app", Type: config.ServiceTypeApp, Port: 8080},
		},
		OutputDir: ".",
	}
}

// ApplyDefaults fills in any missing fields with sensible defaults.
func (o *Options) ApplyDefaults() {
	if o.LanguageVersion == "" {
		o.LanguageVersion = defaultVersionForStack(o.Stack)
	}
	if o.OutputDir == "" {
		o.OutputDir = "."
	}
}

func printSummary(opts Options) {
	fmt.Println()
	ui.Success("pilot.yaml generated")
	ui.Success(".mcp.json generated  ← connects your AI agent to pilot")
	if opts.TargetHost == "" && opts.TargetType != "" {
		fmt.Println()
		ui.Warn("Target host not configured — edit pilot.yaml before deploying:")
		ui.Dim(fmt.Sprintf("  targets:\n    %s-prod:\n      host: \"YOUR_VPS_IP\"", opts.TargetType))
		ui.Dim("  Or run: pilot setup --env prod  (after filling the host)")
	}
	fmt.Println()
	ui.Bold("  How the AI agent creates missing files (Dockerfile, compose):")
	ui.Dim("  1. Open this project in Claude Code or Cursor")
	ui.Dim("     The agent connects automatically via .mcp.json")
	ui.Dim("  2. Ask: \"generate the Dockerfile and docker-compose.dev.yml\"")
	ui.Dim("     The agent calls pilot_generate_dockerfile + pilot_generate_compose")
	ui.Dim("  3. pilot up       — start local services")
	ui.Dim("  4. pilot push     — build + push image to " + opts.Registry)
	ui.Dim("  5. pilot deploy   — deploy to your VPS / cloud")
	fmt.Println()
	ui.Dim("  Without AI agent:")
	ui.Dim("  → Write Dockerfile and docker-compose.dev.yml manually, then pilot up")
	fmt.Println()
}

func defaultPortForService(svcType string) int {
	ports := map[string]int{
		"app":      8080,
		"postgres": 5432,
		"mysql":    3306,
		"mongodb":  27017,
		"redis":    6379,
		"rabbitmq": 5672,
		"nats":     4222,
		"nginx":    80,
	}
	if p, ok := ports[svcType]; ok {
		return p
	}
	return 8080
}

func defaultVersionForService(svcType string) string {
	versions := map[string]string{
		"postgres": "16",
		"mysql":    "8",
		"mongodb":  "7",
		"redis":    "7",
		"rabbitmq": "3",
		"nginx":    "alpine",
	}
	return versions[svcType]
}

func defaultVersionForStack(stack string) string {
	versions := map[string]string{
		"go":     "1.23",
		"node":   "20",
		"python": "3.12",
		"rust":   "1.82",
		"java":   "21",
	}
	if v, ok := versions[stack]; ok {
		return v
	}
	return ""
}
