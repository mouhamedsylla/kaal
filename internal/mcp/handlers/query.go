package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/orchestrator"
	"github.com/mouhamedsylla/pilot/internal/providers"
	"github.com/mouhamedsylla/pilot/internal/runtime"
)

// HandleStatus returns the complete project state as structured JSON.
func HandleStatus(ctx context.Context, params map[string]any) (any, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	activeEnv := pilotenv.Active(strParam(params, "env"))
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined", activeEnv)
	}

	result := map[string]any{
		"env":     activeEnv,
		"project": cfg.Project.Name,
	}

	// Remote target
	if envCfg.Target != "" {
		provider, err := runtime.NewProvider(cfg, envCfg.Target)
		if err != nil {
			return nil, err
		}
		statuses, err := provider.Status(ctx, activeEnv)
		if err != nil {
			result["remote_error"] = err.Error()
		} else {
			result["remote"] = statusesToMap(statuses)
		}
		return result, nil
	}

	// Local orchestrator
	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return nil, err
	}
	orchStatuses, err := orch.Status(ctx)
	if err != nil {
		result["local_error"] = err.Error()
	} else {
		rows := make([]map[string]any, len(orchStatuses))
		for i, s := range orchStatuses {
			rows[i] = map[string]any{
				"name":   s.Name,
				"state":  s.State,
				"health": s.Health,
				"image":  s.Image,
				"ports":  s.Ports,
			}
		}
		result["local"] = rows
	}
	return result, nil
}

// HandleLogs collects log lines from a service and returns them as a string.
// MCP is synchronous so streaming is not possible — we collect for up to 3s
// or until `lines` lines are received.
func HandleLogs(ctx context.Context, params map[string]any) (any, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	activeEnv := pilotenv.Active(strParam(params, "env"))
	service := strParam(params, "service")
	lines := 100
	if l := strParam(params, "lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			lines = n
		}
	}
	since := strParam(params, "since")

	envCfg := cfg.Environments[activeEnv]

	var ch <-chan string
	if envCfg.Target != "" {
		provider, err := runtime.NewProvider(cfg, envCfg.Target)
		if err != nil {
			return nil, err
		}
		ch, err = provider.Logs(ctx, activeEnv, providers.LogOptions{
			Service: service,
			Follow:  false,
			Lines:   lines,
			Since:   since,
		})
		if err != nil {
			return nil, err
		}
	} else {
		orch, err := runtime.NewOrchestrator(cfg, activeEnv)
		if err != nil {
			return nil, err
		}
		ch, err = orch.Logs(ctx, service, orchestrator.LogOptions{
			Follow: false,
			Lines:  lines,
			Since:  since,
		})
		if err != nil {
			return nil, err
		}
	}

	// Collect lines with a 5s deadline to avoid hanging forever.
	deadline := time.After(5 * time.Second)
	var collected []string
	for len(collected) < lines {
		select {
		case line, ok := <-ch:
			if !ok {
				goto done
			}
			collected = append(collected, line)
		case <-deadline:
			goto done
		case <-ctx.Done():
			goto done
		}
	}
done:
	return map[string]any{
		"env":     activeEnv,
		"service": service,
		"lines":   len(collected),
		"output":  strings.Join(collected, "\n"),
	}, nil
}

// HandleConfigGet reads a dot-notation key from pilot.yaml.
func HandleConfigGet(_ context.Context, params map[string]any) (any, error) {
	key := strParam(params, "key")
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	value, err := configGet(cfg, key)
	if err != nil {
		return nil, err
	}
	return map[string]any{"key": key, "value": value}, nil
}

// HandleConfigSet sets a dot-notation key in pilot.yaml.
func HandleConfigSet(_ context.Context, params map[string]any) (any, error) {
	key := strParam(params, "key")
	value := strParam(params, "value")
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	if err := configSet(cfg, key, value); err != nil {
		return nil, err
	}
	if err := config.Save(cfg, "pilot.yaml"); err != nil {
		return nil, err
	}
	return map[string]any{
		"key":     key,
		"value":   value,
		"message": fmt.Sprintf("Set %s = %s in pilot.yaml", key, value),
	}, nil
}

// HandleSecretsInject resolves and returns all secrets for an environment.
func HandleSecretsInject(ctx context.Context, params map[string]any) (any, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	activeEnv := pilotenv.Active(strParam(params, "env"))
	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined", activeEnv)
	}

	provider := "local"
	var refs map[string]string

	if envCfg.Secrets != nil {
		if envCfg.Secrets.Provider != "" {
			provider = envCfg.Secrets.Provider
		}
		refs = envCfg.Secrets.Refs
	}
	if p := strParam(params, "provider"); p != "" {
		provider = p
	}

	sm, err := runtime.NewSecretManager(provider)
	if err != nil {
		return nil, err
	}

	if refs == nil {
		refs = map[string]string{}
	}

	injected, err := sm.Inject(ctx, activeEnv, refs)
	if err != nil {
		return nil, fmt.Errorf("inject secrets: %w", err)
	}

	// Redact values for safety — only return key names unless explicitly requested.
	keys := make([]string, 0, len(injected))
	for k := range injected {
		keys = append(keys, k)
	}

	return map[string]any{
		"env":      activeEnv,
		"provider": provider,
		"injected": keys,
		"message":  fmt.Sprintf("%d secrets resolved from %s", len(keys), provider),
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func statusesToMap(ss []providers.ServiceStatus) []map[string]any {
	out := make([]map[string]any, len(ss))
	for i, s := range ss {
		out[i] = map[string]any{
			"name":    s.Name,
			"state":   s.State,
			"health":  s.Health,
			"version": s.Version,
		}
	}
	return out
}

// configGet navigates dot-notation keys into the config struct.
func configGet(cfg *config.Config, key string) (string, error) {
	switch key {
	case "project.name":
		return cfg.Project.Name, nil
	case "project.stack":
		return cfg.Project.Stack, nil
	case "project.language_version":
		return cfg.Project.LanguageVersion, nil
	case "registry.provider":
		return cfg.Registry.Provider, nil
	case "registry.image":
		return cfg.Registry.Image, nil
	case "registry.url":
		return cfg.Registry.URL, nil
	default:
		return "", fmt.Errorf("unknown key %q — supported: project.name, project.stack, project.language_version, registry.provider, registry.image, registry.url", key)
	}
}

// configSet updates a dot-notation key in the config struct.
func configSet(cfg *config.Config, key, value string) error {
	switch key {
	case "project.name":
		cfg.Project.Name = value
	case "project.stack":
		cfg.Project.Stack = value
	case "project.language_version":
		cfg.Project.LanguageVersion = value
	case "registry.provider":
		cfg.Registry.Provider = value
	case "registry.image":
		cfg.Registry.Image = value
	case "registry.url":
		cfg.Registry.URL = value
	default:
		return fmt.Errorf("unknown key %q — supported: project.name, project.stack, project.language_version, registry.provider, registry.image, registry.url", key)
	}
	return nil
}

