package config

import (
	"fmt"
	"strings"
)

var validRegistryProviders = map[string]bool{
	"ghcr": true, "dockerhub": true, "ecr": true,
	"gcr": true, "acr": true, "custom": true,
}

var validTargetTypes = map[string]bool{
	"vps": true, "aws": true, "gcp": true,
	"azure": true, "do": true, "hetzner": true,
}

var validRuntimes = map[string]bool{
	RuntimeCompose: true,
	RuntimeLima:    true,
	RuntimeK3d:     true,
}

var validServiceTypes = map[string]bool{
	ServiceTypeApp:      true,
	ServiceTypePostgres: true,
	ServiceTypeMySQL:    true,
	ServiceTypeMongoDB:  true,
	ServiceTypeRedis:    true,
	ServiceTypeRabbitMQ: true,
	ServiceTypeNATS:     true,
	ServiceTypeNginx:    true,
	ServiceTypeCustom:   true,
}

// Validate performs semantic validation on a parsed Config.
func Validate(cfg *Config) error {
	var errs []string

	if cfg.APIVersion != APIVersion {
		errs = append(errs, fmt.Sprintf("apiVersion must be %q, got %q", APIVersion, cfg.APIVersion))
	}

	if cfg.Project.Name == "" {
		errs = append(errs, "project.name is required")
	}

	// Services
	hasApp := false
	for name, svc := range cfg.Services {
		if !validServiceTypes[svc.Type] {
			errs = append(errs, fmt.Sprintf("services.%s.type %q is not supported", name, svc.Type))
		}
		if svc.Type == ServiceTypeApp {
			hasApp = true
			if svc.Port == 0 {
				errs = append(errs, fmt.Sprintf("services.%s.port is required for type 'app'", name))
			}
		}
	}
	if len(cfg.Services) > 0 && !hasApp {
		errs = append(errs, "at least one service of type 'app' is required")
	}

	// Environments
	for envName, env := range cfg.Environments {
		if env.Runtime != "" && !validRuntimes[env.Runtime] {
			errs = append(errs, fmt.Sprintf("environments.%s.runtime %q is not valid (compose|lima|k3d)", envName, env.Runtime))
		}
		if env.Target != "" {
			if _, ok := cfg.Targets[env.Target]; !ok {
				errs = append(errs, fmt.Sprintf("environments.%s.target %q not found in targets", envName, env.Target))
			}
		}
	}

	// Registry
	if cfg.Registry.Provider != "" {
		if !validRegistryProviders[cfg.Registry.Provider] {
			errs = append(errs, fmt.Sprintf("registry.provider %q is not supported", cfg.Registry.Provider))
		}
		if cfg.Registry.Image == "" {
			errs = append(errs, "registry.image is required")
		}
		if cfg.Registry.Provider == "custom" && cfg.Registry.URL == "" {
			errs = append(errs, "registry.url is required when provider is 'custom'")
		}
	}

	// Targets
	for targetName, target := range cfg.Targets {
		if !validTargetTypes[target.Type] {
			errs = append(errs, fmt.Sprintf("targets.%s.type %q is not supported", targetName, target.Type))
		}
		if target.Type == "vps" || target.Type == "hetzner" {
			if target.User == "" {
				errs = append(errs, fmt.Sprintf("targets.%s.user is required for type %q", targetName, target.Type))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
