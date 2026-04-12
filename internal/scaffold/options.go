package scaffold

import (
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
)

// Options holds everything collected during pilot init.
type Options struct {
	// Project
	Name            string
	Stack           string
	LanguageVersion string

	// Services selected by the user
	Services []ServiceChoice

	// Environments selected (e.g. dev, staging, prod, test...)
	Environments []string

	// Target type and connection info for non-dev environments
	TargetType   string // vps | aws | gcp | azure | do | hetzner
	TargetHost   string // IP or hostname (empty = not yet configured)
	TargetUser   string // SSH user (default: deploy)
	TargetSSHKey string // SSH key path (default: ~/.ssh/id_pilot)

	// Registry
	Registry      string            // ghcr | dockerhub | custom
	RegistryImage string
	RegistryURL   string            // custom only
	RegistryCreds map[string]string // vars collected in wizard → written to .env.local

	// Where to scaffold
	OutputDir string // "." for existing project, or "./<name>" for new one

	// Non-interactive mode
	Yes bool
}

// ServiceChoice represents a selected service with its resolved configuration.
type ServiceChoice struct {
	Name     string
	Type     string
	Port     int
	Version  string
	Hosting  string // config.HostingContainer | config.HostingManaged | config.HostingLocalOnly
	Provider string // catalog provider key: "neon" | "supabase" | "container" | "upstash" | ...
}

// EnvVars returns the environment variables expected by this service choice.
// Returns nil for container-hosted services.
func (s ServiceChoice) EnvVars() []string {
	return catalog.EnvVarsFor(s.Type, s.Provider)
}

// EnvHints returns example values for each expected env var.
func (s ServiceChoice) EnvHints() map[string]string {
	return catalog.EnvHintsFor(s.Type, s.Provider)
}

// IsManaged returns true when the service is externally managed (no container).
func (s ServiceChoice) IsManaged() bool {
	return s.Hosting == config.HostingManaged
}

// imageOrPlaceholder returns the registry image or a descriptive placeholder.
func (o *Options) imageOrPlaceholder() string {
	if o.RegistryImage != "" {
		return o.RegistryImage
	}
	switch o.Registry {
	case "ghcr":
		return "ghcr.io/YOUR_GITHUB_USER/" + o.Name
	case "dockerhub":
		return "YOUR_DOCKERHUB_USER/" + o.Name
	default:
		return o.Name
	}
}

// ToConfig converts Options into a config.Config ready to be serialized.
func (o *Options) ToConfig() *config.Config {
	cfg := &config.Config{
		APIVersion: config.APIVersion,
		Project: config.Project{
			Name:            o.Name,
			Stack:           o.Stack,
			LanguageVersion: o.LanguageVersion,
		},
		Services:     make(map[string]config.Service),
		Environments: make(map[string]config.Environment),
		Targets:      make(map[string]config.Target),
		Registry: config.RegistryConfig{
			Provider: o.Registry,
			Image:    o.imageOrPlaceholder(),
			URL:      o.RegistryURL,
		},
	}

	// Services
	for _, svc := range o.Services {
		hosting := svc.Hosting
		if hosting == "" {
			hosting = config.HostingContainer
		}

		s := config.Service{
			Type:     svc.Type,
			Port:     svc.Port,
			Version:  svc.Version,
			Hosting:  hosting,
			Provider: svc.Provider,
		}

		// For container-hosted services with no explicit version,
		// populate the default image tag from the catalog.
		if hosting == config.HostingContainer && svc.Version == "" {
			if img := catalog.DefaultImageFor(svc.Type); img != "" {
				s.Image = img
			}
		}

		cfg.Services[svc.Name] = s
	}

	// Environments
	for _, env := range o.Environments {
		e := config.Environment{
			Runtime: config.RuntimeCompose,
			EnvFile: ".env." + env,
		}
		if env != "dev" && o.TargetType != "" {
			targetName := o.TargetType + "-" + env
			e.Target = targetName
			user := o.TargetUser
			if user == "" {
				user = "deploy"
			}
			key := o.TargetSSHKey
			if key == "" {
				key = "~/.ssh/id_pilot"
			}
			cfg.Targets[targetName] = config.Target{
				Type: o.TargetType,
				Host: o.TargetHost,
				User: user,
				Key:  key,
				Port: 22,
			}
		}
		cfg.Environments[env] = e
	}

	return cfg
}

// ManagedServices returns only the services that are externally managed.
// Used by generator.go to build .env.example.
func (o *Options) ManagedServices() []ServiceChoice {
	var managed []ServiceChoice
	for _, svc := range o.Services {
		if svc.IsManaged() {
			managed = append(managed, svc)
		}
	}
	return managed
}
