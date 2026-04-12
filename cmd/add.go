package cmd

import (
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/scaffold"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
	"github.com/mouhamedsylla/pilot/internal/scaffold/tui"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [type]",
	Short: "Add a service to an existing pilot project",
	Long: `Add a new service to pilot.yaml without reinitialising the project.

Launches a short wizard to collect the service type, hosting mode, and provider.
Updates pilot.yaml and .env.example in place — existing content is preserved.

Examples:
  pilot add                     # interactive — choose type, hosting, provider
  pilot add storage             # interactive — skip type step
  pilot add redis --managed     # managed hosting, pick provider interactively
  pilot add postgres --managed --provider neon --name db
  pilot add elasticsearch --yes # non-interactive, container, default name`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

func init() {
	addCmd.Flags().StringP("name", "n", "", "service name in pilot.yaml (default: type)")
	addCmd.Flags().BoolP("managed", "m", false, "mark service as externally managed")
	addCmd.Flags().StringP("provider", "p", "", "managed provider key (neon, supabase, upstash...)")
	addCmd.Flags().BoolP("yes", "y", false, "non-interactive — accept defaults (container hosting)")
}

func runAdd(cmd *cobra.Command, args []string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	name, _ := cmd.Flags().GetString("name")
	managed, _ := cmd.Flags().GetBool("managed")
	provider, _ := cmd.Flags().GetString("provider")

	presetType := ""
	if len(args) > 0 {
		presetType = args[0]
	}

	var opts scaffold.AddOptions

	if yes {
		// Non-interactive: require type as argument.
		if presetType == "" {
			return fmt.Errorf(
				"service type required in non-interactive mode\n\n"+
					"  Usage: pilot add <type> --yes\n\n"+
					"  Available types: %s",
				catalogTypesSummary(),
			)
		}
		opts = scaffold.AddOptions{
			Type:     presetType,
			Name:     name,
			Provider: provider,
		}
		if managed {
			opts.Hosting = "managed"
		} else {
			opts.Hosting = "container"
		}
	} else {
		// Interactive wizard.
		wizardResult, err := tui.RunAddWizard(presetType)
		if err != nil {
			return err
		}
		if wizardResult.Cancelled {
			ui.Warn("Cancelled.")
			return nil
		}
		opts = scaffold.AddOptions{
			Type:     wizardResult.Type,
			Name:     wizardResult.Name,
			Hosting:  wizardResult.Hosting,
			Provider: wizardResult.Provider,
		}
		// CLI flags override wizard if explicitly set.
		if name != "" {
			opts.Name = name
		}
		if managed {
			opts.Hosting = "managed"
		}
		if provider != "" {
			opts.Provider = provider
		}
	}

	result, err := scaffold.Add(opts)
	if err != nil {
		ui.Fatal(err)
	}

	printAddSummary(result)
	return nil
}

func printAddSummary(r *scaffold.AddResult) {
	fmt.Println()

	svcDef, _ := catalog.Get(r.ServiceType)
	label := svcDef.Label
	if label == "" {
		label = r.ServiceType
	}

	ui.Success(fmt.Sprintf("Service %q added to pilot.yaml", r.ServiceName))

	hostingDesc := "container"
	if r.Hosting == "managed" {
		if pDef, ok := catalog.GetProvider(r.ServiceType, r.Provider); ok {
			hostingDesc = "managed → " + pDef.Label
		} else {
			hostingDesc = "managed"
		}
	} else if r.Hosting == "local-only" {
		hostingDesc = "local-only (dev only)"
	}
	ui.Dim(fmt.Sprintf("  %-14s %s", "type:", label))
	ui.Dim(fmt.Sprintf("  %-14s %s", "hosting:", hostingDesc))

	if len(r.EnvVarsAdded) > 0 {
		fmt.Println()
		ui.Success(".env.example updated")
		ui.Dim("  Add these to .env.dev, .env.staging, .env.prod:")
		for _, v := range r.EnvVarsAdded {
			ui.Dim("    " + v + "=")
		}
	}

	fmt.Println()
	ui.Bold("  Next steps:")

	if r.Hosting == "managed" {
		ui.Dim("  1. Fill in the env vars in .env.dev (and other envs)")
		ui.Dim("  2. pilot up  →  regenerate compose (managed service skipped automatically)")
		ui.Dim("  3. pilot deploy  →  push to VPS")
	} else {
		ui.Dim("  1. pilot up  →  regenerate compose with the new service")
		ui.Dim("  2. pilot deploy  →  deploy to VPS")
	}

	if strings.Contains(r.Hosting, "local-only") {
		fmt.Println()
		ui.Warn("local-only service: will appear in docker-compose.dev.yml only, skipped in prod")
	}

	fmt.Println()
}

func catalogTypesSummary() string {
	var names []string
	for _, svc := range catalog.Services {
		if svc.Key != "app" {
			names = append(names, svc.Key)
		}
	}
	return strings.Join(names, ", ")
}
