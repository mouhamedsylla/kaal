package handlers

import (
	"context"
	"fmt"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/runtime"
)

// dockerSetup is a duck-typed interface satisfied by *vps.Provider.
type dockerSetup interface {
	SetupDockerGroup(ctx context.Context) error
}

// HandleSetup runs one-time VPS setup tasks (docker group, etc.) via SSH.
func HandleSetup(ctx context.Context, params map[string]any) (any, error) {
	activeEnv := strParam(params, "env")
	if activeEnv == "" {
		activeEnv = env.Active("")
	}

	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return nil, fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}

	targetName := envCfg.Target
	if targetName == "" {
		return nil, fmt.Errorf("no deploy target for environment %q", activeEnv)
	}

	target, ok := cfg.Targets[targetName]
	if !ok {
		return nil, fmt.Errorf("target %q not defined in pilot.yaml", targetName)
	}
	if target.Host == "" {
		return nil, fmt.Errorf(
			"target %q has no host configured — edit pilot.yaml:\n  targets:\n    %s:\n      host: \"YOUR_VPS_IP\"",
			targetName, targetName,
		)
	}

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return nil, err
	}

	ds, ok := provider.(dockerSetup)
	if !ok {
		return nil, fmt.Errorf("target type %q does not support docker setup (VPS/hetzner only)", target.Type)
	}

	if err := ds.SetupDockerGroup(ctx); err != nil {
		return nil, fmt.Errorf("setup docker group: %w", err)
	}

	return map[string]any{
		"message": fmt.Sprintf(
			"User %q added to docker group on %s. Run pilot_deploy — the new SSH session will pick up the group change.",
			target.User, target.Host,
		),
		"target": targetName,
		"host":   target.Host,
		"user":   target.User,
	}, nil
}
