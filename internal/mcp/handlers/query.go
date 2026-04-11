package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mouhamedsylla/pilot/internal/app/logs"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	"github.com/mouhamedsylla/pilot/internal/app/status"
	"github.com/mouhamedsylla/pilot/internal/config"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
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

	in := status.Input{Env: activeEnv, Config: cfg}
	var out status.Output

	if envCfg.Target != "" {
		provider, pErr := runtime.NewDeployProvider(cfg, envCfg.Target)
		if pErr != nil {
			return nil, pErr
		}
		uc := status.NewRemote(provider)
		out, err = uc.Execute(ctx, in)
	} else {
		provider, pErr := runtime.NewExecutionProvider(cfg, activeEnv)
		if pErr != nil {
			return nil, pErr
		}
		uc := status.New(provider)
		out, err = uc.Execute(ctx, in)
	}
	if err != nil {
		return map[string]any{"env": activeEnv, "error": err.Error()}, nil
	}

	rows := make([]map[string]any, len(out.Statuses))
	for i, s := range out.Statuses {
		rows[i] = map[string]any{
			"name":   s.Name,
			"state":  s.State,
			"health": s.Health,
		}
	}

	result := map[string]any{
		"env":      activeEnv,
		"project":  cfg.Project.Name,
		"remote":   out.Remote,
		"services": rows,
	}
	if out.Remote {
		result["target"] = out.Target
		result["host"] = out.Host
	}
	return result, nil
}

// HandleLogs collects log lines from a service and returns them as a string.
// MCP is synchronous so streaming is not possible — we collect for up to 5s
// or until `lines` lines are received.
func HandleLogs(ctx context.Context, params map[string]any) (any, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	activeEnv := pilotenv.Active(strParam(params, "env"))
	service := strParam(params, "service")
	nLines := 100
	if l := strParam(params, "lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			nLines = n
		}
	}
	since := strParam(params, "since")

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined", activeEnv)
	}

	in := logs.Input{
		Env:     activeEnv,
		Service: service,
		Follow:  false,
		Lines:   nLines,
		Since:   since,
		Config:  cfg,
	}

	var uc *logs.LogsUseCase
	if envCfg.Target != "" {
		provider, pErr := runtime.NewDeployProvider(cfg, envCfg.Target)
		if pErr != nil {
			return nil, pErr
		}
		uc = logs.NewRemote(provider)
	} else {
		provider, pErr := runtime.NewExecutionProvider(cfg, activeEnv)
		if pErr != nil {
			return nil, pErr
		}
		uc = logs.New(provider)
	}

	out, err := uc.Execute(ctx, in)
	if err != nil {
		return nil, err
	}

	// Collect lines with a 5s deadline to avoid hanging forever.
	deadline := time.After(5 * time.Second)
	var collected []string
	for len(collected) < nLines {
		select {
		case line, ok := <-out.Lines:
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
