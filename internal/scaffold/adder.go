package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
)

// AddOptions holds everything needed to add a service to an existing project.
type AddOptions struct {
	// Type is the catalog service key (e.g. "postgres", "redis", "storage").
	Type string

	// Name is the key used in pilot.yaml services: section.
	// Defaults to Type if empty.
	Name string

	// Hosting is "container", "managed", or "local-only".
	// Defaults to "container" for non-interactive mode.
	Hosting string

	// Provider is the catalog provider key (e.g. "neon", "upstash").
	// Required when Hosting is "managed". Defaults to the first managed
	// provider in the catalog if empty.
	Provider string
}

// AddResult summarises what was written.
type AddResult struct {
	ServiceName    string
	ServiceType    string
	Hosting        string
	Provider       string
	EnvVarsAdded   []string // vars written to .env.example
	PilotYAMLPath  string
	EnvExamplePath string
}

// Add adds a new service to the pilot.yaml in the current directory and
// updates .env.example with the provider's required env vars.
//
// It is safe to call multiple times with different services — it will return
// an error if the service name already exists rather than overwriting it.
func Add(opts AddOptions) (*AddResult, error) {
	// ── Resolve defaults ─────────────────────────────────────────────────────
	if opts.Name == "" {
		opts.Name = opts.Type
	}
	if opts.Hosting == "" {
		opts.Hosting = config.HostingContainer
	}

	// ── Validate service type against catalog ─────────────────────────────────
	svcDef, ok := catalog.Get(opts.Type)
	if !ok {
		return nil, fmt.Errorf(
			"unknown service type %q\n\n"+
				"  Available types: %s\n\n"+
				"  For a custom service, add it manually to pilot.yaml:\n"+
				"    services:\n      %s:\n        type: custom\n        image: your-image:tag",
			opts.Type,
			catalogTypeList(),
			opts.Name,
		)
	}

	// ── Validate hosting / provider combo ────────────────────────────────────
	if opts.Hosting == config.HostingManaged {
		if !svcDef.CanBeManaged {
			return nil, fmt.Errorf(
				"service type %q cannot be managed externally\n"+
					"  Only these types support managed hosting: %s",
				opts.Type, managedTypeList(),
			)
		}
		// Resolve provider: use first managed catalog provider as default.
		if opts.Provider == "" || opts.Provider == config.HostingContainer {
			providers := catalog.ManagedProviders(opts.Type)
			if len(providers) > 0 {
				opts.Provider = providers[0].Key
			}
		}
		// Validate provider exists in catalog.
		if _, ok := catalog.GetProvider(opts.Type, opts.Provider); !ok {
			return nil, fmt.Errorf(
				"unknown provider %q for service type %q\n"+
					"  Available managed providers: %s",
				opts.Provider, opts.Type,
				managedProviderList(opts.Type),
			)
		}
	} else {
		opts.Provider = config.HostingContainer
	}

	// ── Load existing pilot.yaml ──────────────────────────────────────────────
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	// ── Guard: service name must be unique ────────────────────────────────────
	if _, exists := cfg.Services[opts.Name]; exists {
		return nil, fmt.Errorf(
			"service %q already exists in pilot.yaml\n\n"+
				"  To change it, edit pilot.yaml directly.\n"+
				"  To add a second instance, use a different name:\n"+
				"    pilot add %s --name %s-2",
			opts.Name, opts.Type, opts.Name,
		)
	}

	// ── Build Service entry ───────────────────────────────────────────────────
	svc := config.Service{
		Type:     opts.Type,
		Hosting:  opts.Hosting,
		Provider: opts.Provider,
	}

	// For container services: set default image from catalog.
	if opts.Hosting == config.HostingContainer {
		svc.Image = catalog.DefaultImageFor(opts.Type)
		svc.Port = defaultPortForService(opts.Type)
	}

	// ── Write pilot.yaml ──────────────────────────────────────────────────────
	cfg.Services[opts.Name] = svc

	yamlPath := config.FileName
	if err := config.Save(cfg, yamlPath); err != nil {
		return nil, fmt.Errorf("update pilot.yaml: %w", err)
	}

	// ── Update .env.example for managed services ──────────────────────────────
	var envVarsAdded []string
	envExamplePath := ".env.example"

	if opts.Hosting == config.HostingManaged {
		envVarsAdded = catalog.EnvVarsFor(opts.Type, opts.Provider)
		if err := appendManagedEnvSection(envExamplePath, opts, svcDef); err != nil {
			// Non-fatal: warn but continue.
			fmt.Printf("warning: could not update .env.example: %v\n", err)
		}
	}

	return &AddResult{
		ServiceName:    opts.Name,
		ServiceType:    opts.Type,
		Hosting:        opts.Hosting,
		Provider:       opts.Provider,
		EnvVarsAdded:   envVarsAdded,
		PilotYAMLPath:  yamlPath,
		EnvExamplePath: envExamplePath,
	}, nil
}

// appendManagedEnvSection appends the env var section for a managed service
// to .env.example. Idempotent — skips if the section already exists.
func appendManagedEnvSection(path string, opts AddOptions, svcDef catalog.ServiceDef) error {
	provider, ok := catalog.GetProvider(opts.Type, opts.Provider)
	if !ok || !provider.IsManaged {
		return nil
	}

	existing := readFileOrEmpty(path)

	sectionComment := fmt.Sprintf("# ── %s (%s: %s)", svcDef.Label, opts.Type, provider.Label)
	if strings.Contains(existing, sectionComment) {
		return nil // already documented
	}

	var b strings.Builder
	b.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(sectionComment + "\n")

	for _, envVar := range provider.EnvVars {
		hint := ""
		if provider.EnvHints != nil {
			hint = provider.EnvHints[envVar]
		}
		if hint != "" {
			b.WriteString(envVar + "=" + hint + "\n")
		} else {
			b.WriteString(envVar + "=\n")
		}
	}

	return writeFile(path, b.String())
}

// ── helpers ───────────────────────────────────────────────────────────────────

func catalogTypeList() string {
	var keys []string
	for _, svc := range catalog.Services {
		if svc.Key != "app" {
			keys = append(keys, svc.Key)
		}
	}
	return strings.Join(keys, ", ")
}

func managedTypeList() string {
	var keys []string
	for _, svc := range catalog.Services {
		if svc.CanBeManaged {
			keys = append(keys, svc.Key)
		}
	}
	return strings.Join(keys, ", ")
}

func managedProviderList(serviceType string) string {
	providers := catalog.ManagedProviders(serviceType)
	var keys []string
	for _, p := range providers {
		keys = append(keys, p.Key)
	}
	return strings.Join(keys, ", ")
}

// readFileOrEmpty and writeFile are shared helpers — defined in generator.go
// within the same package. Avoid redeclaring them here.

func writeFile(path string, content string) error {
	return os.WriteFile(filepath.Clean(path), []byte(content), 0644)
}
