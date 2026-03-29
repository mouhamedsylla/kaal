package scaffold

import "github.com/mouhamedsylla/kaal/internal/config"

// Options holds everything collected during kaal init.
type Options struct {
	// Project
	Name            string
	Stack           string // detected or typed
	LanguageVersion string

	// Services selected by the user
	Services []ServiceChoice

	// Environments selected (e.g. dev, staging, prod, test...)
	Environments []string

	// Target type for non-dev environments
	TargetType string // vps | aws | gcp | azure | do | hetzner

	// Registry
	Registry      string // ghcr | dockerhub | custom
	RegistryImage string
	RegistryURL   string // custom only

	// Where to scaffold
	OutputDir string // "." for existing project, or "./<name>" for new one

	// Non-interactive mode
	Yes bool
}

// ServiceChoice represents a selected service with its configuration.
type ServiceChoice struct {
	Name    string
	Type    string
	Port    int
	Version string
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
		cfg.Services[svc.Name] = config.Service{
			Type:    svc.Type,
			Port:    svc.Port,
			Version: svc.Version,
		}
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
			cfg.Targets[targetName] = config.Target{
				Type: o.TargetType,
				Host: "",
				User: "deploy",
				Key:  "~/.ssh/id_kaal",
				Port: 22,
			}
		}
		cfg.Environments[env] = e
	}

	return cfg
}
