package handlers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/scaffold"
)

// HandleInit creates kaal.yaml non-interactively from MCP params.
//
// Required params:
//
//	name     — project name
//
// Optional params:
//
//	stack          — go | node | python | rust | java  (default: go)
//	services       — comma-separated list: app,postgres,redis  (default: app)
//	envs           — comma-separated list: dev,staging,prod  (default: dev,prod)
//	registry       — ghcr | dockerhub | custom  (default: ghcr)
//	registry_image — full image name (optional)
//	target_type    — vps | aws | gcp | azure | do  (default: vps)
func HandleInit(_ context.Context, params map[string]any) (any, error) {
	name := strParam(params, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Guard: refuse if kaal.yaml already exists.
	if _, err := os.Stat(config.FileName); err == nil {
		return nil, fmt.Errorf(
			"kaal.yaml already exists in this directory — edit it directly or delete it first",
		)
	}

	stack := strParam(params, "stack")
	if stack == "" {
		stack = "go"
	}

	registry := strParam(params, "registry")
	if registry == "" {
		registry = "ghcr"
	}

	targetType := strParam(params, "target_type")
	if targetType == "" {
		targetType = "vps"
	}

	// Parse envs (default: dev + prod)
	envsRaw := strParam(params, "envs")
	envs := SplitTrim(envsRaw)
	if len(envs) == 0 {
		envs = []string{"dev", "prod"}
	}

	// Parse services: "app,postgres,redis" → []ServiceChoice
	servicesRaw := strParam(params, "services")
	serviceNames := SplitTrim(servicesRaw)
	if len(serviceNames) == 0 {
		serviceNames = []string{"app"}
	}

	services := make([]scaffold.ServiceChoice, 0, len(serviceNames))
	for _, raw := range serviceNames {
		// Support "name:type" or just "name" (name == type for managed services)
		parts := strings.SplitN(raw, ":", 2)
		svcName := strings.TrimSpace(parts[0])
		svcType := svcName
		if len(parts) == 2 {
			svcType = strings.TrimSpace(parts[1])
		}
		services = append(services, scaffold.ServiceChoice{
			Name:    svcName,
			Type:    svcType,
			Port:    defaultPortForType(svcType),
			Version: defaultVersionForType(svcType),
		})
	}

	opts := scaffold.Options{
		Name:          name,
		Stack:         stack,
		Registry:      registry,
		RegistryImage: strParam(params, "registry_image"),
		Environments:  envs,
		TargetType:    targetType,
		Services:      services,
		OutputDir:     ".",
	}
	opts.ApplyDefaults()

	if err := scaffold.Generate(opts); err != nil {
		return nil, fmt.Errorf("kaal init: %w", err)
	}

	return map[string]any{
		"message":  fmt.Sprintf("kaal.yaml generated for project %q", name),
		"name":     name,
		"stack":    stack,
		"envs":     envs,
		"services": serviceNames,
		"registry": registry,
	}, nil
}

func defaultPortForType(svcType string) int {
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

func defaultVersionForType(svcType string) string {
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

