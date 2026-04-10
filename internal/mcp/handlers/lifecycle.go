package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/app/deploy"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/app/push"
	"github.com/mouhamedsylla/pilot/internal/app/rollback"
	pilotSync "github.com/mouhamedsylla/pilot/internal/app/sync"
	"github.com/mouhamedsylla/pilot/internal/app/up"
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
func HandleUp(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	var services []string
	if s := strParam(params, "services"); s != "" {
		services = SplitTrim(s)
	}

	output, err := captureOutput(func() error {
		return up.Run(context.Background(), up.Options{
			Env:      env,
			Services: services,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot up: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Services started",
		"output":  strings.TrimSpace(output),
	}, nil
}

// HandleDown stops local services.
func HandleDown(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")

	output, err := captureOutput(func() error {
		return up.RunDown(context.Background(), up.DownOptions{Env: env})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot down: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Services stopped",
		"output":  strings.TrimSpace(output),
	}, nil
}

// HandlePush builds and pushes the Docker image.
func HandlePush(_ context.Context, params map[string]any) (any, error) {
	tag := strParam(params, "tag")
	noCache := strParam(params, "no_cache") == "true"
	var platforms []string
	if p := strParam(params, "platform"); p != "" {
		platforms = SplitTrim(p)
	}

	env := strParam(params, "env")
	output, err := captureOutput(func() error {
		return push.Run(context.Background(), push.Options{
			Env:       env,
			Tag:       tag,
			NoCache:   noCache,
			Platforms: platforms,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot push: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Image built and pushed",
		"output":  strings.TrimSpace(output),
	}, nil
}

// HandleDeploy deploys to a remote target.
func HandleDeploy(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	tag := strParam(params, "tag")
	target := strParam(params, "target")
	dryRun := strParam(params, "dry_run") == "true"

	output, err := captureOutput(func() error {
		return deploy.Run(context.Background(), deploy.Options{
			Env:    env,
			Tag:    tag,
			Target: target,
			DryRun: dryRun,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot deploy: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Deployed successfully",
		"output":  strings.TrimSpace(output),
	}, nil
}

// HandleRollback rolls back to a previous deployment.
func HandleRollback(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	version := strParam(params, "version")
	target := strParam(params, "target")

	output, err := captureOutput(func() error {
		return rollback.Run(context.Background(), rollback.Options{
			Env:     env,
			Version: version,
			Target:  target,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot rollback: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Rolled back successfully",
		"output":  strings.TrimSpace(output),
	}, nil
}

// HandleSync syncs config files to the remote target.
func HandleSync(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	target := strParam(params, "target")

	output, err := captureOutput(func() error {
		return pilotSync.Run(context.Background(), pilotSync.Options{
			Env:    env,
			Target: target,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("pilot sync: %w\n%s", err, output)
	}
	return map[string]any{
		"message": "Config synced",
		"output":  strings.TrimSpace(output),
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
