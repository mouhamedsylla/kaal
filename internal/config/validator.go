package config

import (
	"fmt"
	"strings"
)

var validStacks = map[string]bool{
	"go": true, "node": true, "python": true, "rust": true, "java": true,
}

var validRegistryProviders = map[string]bool{
	"ghcr": true, "dockerhub": true, "ecr": true, "gcr": true, "acr": true, "custom": true,
}

var validTargetTypes = map[string]bool{
	"vps": true, "aws": true, "gcp": true, "azure": true, "do": true, "hetzner": true,
}

var validOrchestrators = map[string]bool{
	"compose": true, "k8s": true,
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

	if cfg.Project.Stack == "" {
		errs = append(errs, "project.stack is required")
	} else if !validStacks[cfg.Project.Stack] {
		errs = append(errs, fmt.Sprintf("project.stack %q is not supported (valid: go, node, python, rust, java)", cfg.Project.Stack))
	}

	if cfg.Registry.Provider == "" {
		errs = append(errs, "registry.provider is required")
	} else if !validRegistryProviders[cfg.Registry.Provider] {
		errs = append(errs, fmt.Sprintf("registry.provider %q is not supported", cfg.Registry.Provider))
	}

	if cfg.Registry.Image == "" {
		errs = append(errs, "registry.image is required")
	}

	if cfg.Registry.Provider == "custom" && cfg.Registry.URL == "" {
		errs = append(errs, "registry.url is required when provider is 'custom'")
	}

	// Validate environments
	seenPorts := map[int]string{}
	for envName, env := range cfg.Environments {
		if env.Target != "" {
			if _, ok := cfg.Targets[env.Target]; !ok {
				errs = append(errs, fmt.Sprintf("environments.%s.target %q not found in targets", envName, env.Target))
			}
		}
		if env.Orchestrator != "" && !validOrchestrators[env.Orchestrator] {
			errs = append(errs, fmt.Sprintf("environments.%s.orchestrator %q is not valid", envName, env.Orchestrator))
		}
		for svc, port := range env.Ports {
			if conflict, seen := seenPorts[port]; seen {
				errs = append(errs, fmt.Sprintf("port %d used by both %s and %s in env %s", port, conflict, svc, envName))
			}
			seenPorts[port] = svc
		}
		seenPorts = map[int]string{} // reset per-env
	}

	// Validate targets
	for targetName, target := range cfg.Targets {
		if !validTargetTypes[target.Type] {
			errs = append(errs, fmt.Sprintf("targets.%s.type %q is not supported", targetName, target.Type))
		}
		if target.Type == "vps" || target.Type == "hetzner" {
			if target.Host == "" {
				errs = append(errs, fmt.Sprintf("targets.%s.host is required for type %q", targetName, target.Type))
			}
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
