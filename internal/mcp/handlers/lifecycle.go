package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/deploy"
pilotPush "github.com/mouhamedsylla/pilot/internal/app/push"
	"github.com/mouhamedsylla/pilot/internal/app/rollback"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
	pilotSync "github.com/mouhamedsylla/pilot/internal/app/sync"
	"github.com/mouhamedsylla/pilot/internal/app/up"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
)

// HandleEnvSwitch switches the active pilot environment.
func HandleEnvSwitch(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	if env == "" {
		return nil, fmt.Errorf("env is required")
	}
	if err := pilotenv.Use(env); err != nil {
		return nil, err
	}
	return map[string]any{
		"env":     env,
		"message": fmt.Sprintf("Switched to environment %q", env),
	}, nil
}

// HandleUp starts local services for the given environment.
func HandleUp(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")
	var services []string
	if s := strParam(params, "services"); s != "" {
		services = SplitTrim(s)
	}

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot up: load config: %w", err)
	}
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	provider, err := runtime.NewExecutionProvider(cfg, activeEnv)
	if err != nil {
		return nil, fmt.Errorf("pilot up: provider: %w", err)
	}

	uc := up.New(provider)
	out, err := uc.Execute(ctx, up.Input{
		Env:      activeEnv,
		Services: services,
		Config:   cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot up: %w", err)
	}

	result := map[string]any{"message": "Services started", "env": out.Env}
	if out.IsRemoteEnv {
		result["warning"] = fmt.Sprintf("env %q has a remote target — image must already exist in registry", activeEnv)
	}
	return result, nil
}

// HandleDown stops local services.
func HandleDown(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot down: load config: %w", err)
	}
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	provider, err := runtime.NewExecutionProvider(cfg, activeEnv)
	if err != nil {
		return nil, fmt.Errorf("pilot down: provider: %w", err)
	}

	uc := up.NewDown(provider)
	out, err := uc.Execute(ctx, up.DownInput{
		Env:    activeEnv,
		Config: cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot down: %w", err)
	}

	return map[string]any{"message": "Services stopped", "env": out.Env}, nil
}

// HandlePush builds and pushes the Docker image.
func HandlePush(ctx context.Context, params map[string]any) (any, error) {
	tag := strParam(params, "tag")
	noCache := strParam(params, "no_cache") == "true"
	var platforms []string
	if p := strParam(params, "platform"); p != "" {
		platforms = SplitTrim(p)
	}
	activeEnv := strParam(params, "env")

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot push: load config: %w", err)
	}
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	provider, err := runtime.NewRegistryProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("pilot push: registry: %w", err)
	}

	uc := pilotPush.New(provider)
	out, err := uc.Execute(ctx, pilotPush.Input{
		Env:       activeEnv,
		Tag:       tag,
		NoCache:   noCache,
		Platforms: platforms,
		Config:    cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot push: %w", err)
	}

	return map[string]any{
		"message": "Image built and pushed",
		"tag":     out.Tag,
		"image":   out.Image,
	}, nil
}

// HandleDeploy deploys to a remote target.
func HandleDeploy(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")
	tag := strParam(params, "tag")
	dryRun := strParam(params, "dry_run") == "true"

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot deploy: load config: %w", err)
	}

	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	targetName := ""
	if envCfg, ok := cfg.Environments[activeEnv]; ok {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return nil, fmt.Errorf("pilot deploy: no target for environment %q", activeEnv)
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		return nil, fmt.Errorf("pilot deploy: provider: %w", err)
	}

	var secretRefs map[string]string
	var secrets domain.SecretManager
	if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.Secrets != nil && len(envCfg.Secrets.Refs) > 0 {
		secretRefs = envCfg.Secrets.Refs
		secrets, err = runtime.NewSecretManager(envCfg.Secrets.Provider)
		if err != nil {
			return nil, fmt.Errorf("pilot deploy: secrets: %w", err)
		}
	}

	uc := deploy.New(provider, secrets, ".pilot")
	out, err := uc.Execute(ctx, deploy.Input{
		Env:        activeEnv,
		Tag:        tag,
		SecretRefs: secretRefs,
		DryRun:     dryRun,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot deploy: %w", err)
	}

	if dryRun {
		return map[string]any{
			"dry_run": true,
			"steps":   out.DryRunPlan.Steps,
		}, nil
	}

	statuses := make([]map[string]string, 0, len(out.Statuses))
	for _, s := range out.Statuses {
		statuses = append(statuses, map[string]string{
			"name": s.Name, "state": s.State, "health": s.Health,
		})
	}
	return map[string]any{
		"message":  "Deployed successfully",
		"tag":      out.Tag,
		"statuses": statuses,
	}, nil
}

// HandleRollback rolls back to a previous deployment.
func HandleRollback(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")
	version := strParam(params, "version")
	targetOverride := strParam(params, "target")

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot rollback: load config: %w", err)
	}
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("pilot rollback: environment %q not defined", activeEnv)
	}

	targetName := targetOverride
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return nil, fmt.Errorf("pilot rollback: no target for environment %q", activeEnv)
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		return nil, fmt.Errorf("pilot rollback: provider: %w", err)
	}

	uc := rollback.New(provider)
	out, err := uc.Execute(ctx, rollback.Input{
		Env:        activeEnv,
		Version:    version,
		TargetName: targetOverride,
		Config:     cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot rollback: %w", err)
	}

	return map[string]any{
		"message":      "Rolled back successfully",
		"restored_tag": out.RestoredTag,
		"target":       out.TargetName,
	}, nil
}

// HandleSync syncs config files to the remote target.
func HandleSync(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")
	targetOverride := strParam(params, "target")

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("pilot sync: load config: %w", err)
	}
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("pilot sync: environment %q not defined", activeEnv)
	}

	targetName := targetOverride
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return nil, fmt.Errorf("pilot sync: no target for environment %q", activeEnv)
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		return nil, fmt.Errorf("pilot sync: provider: %w", err)
	}

	uc := pilotSync.New(provider)
	out, err := uc.Execute(ctx, pilotSync.Input{
		Env:            activeEnv,
		TargetOverride: targetOverride,
		Config:         cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("pilot sync: %w", err)
	}

	return map[string]any{
		"message": "Config synced",
		"target":  out.TargetName,
	}, nil
}

// SplitTrim splits a comma-separated string and trims whitespace from each part.
func SplitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
