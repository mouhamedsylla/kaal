package handlers

import (
	"context"
	"fmt"

	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/app/preflight"
	"github.com/mouhamedsylla/pilot/internal/config"
)

// HandlePreflight runs pre-flight checks and returns a structured report
// the agent uses to decide what to fix and in what order.
func HandlePreflight(ctx context.Context, params map[string]any) (any, error) {
	targetStr := strParam(params, "target")
	if targetStr == "" {
		targetStr = "deploy"
	}

	target := preflight.Target(targetStr)
	switch target {
	case preflight.TargetUp, preflight.TargetPush, preflight.TargetDeploy:
	default:
		return nil, fmt.Errorf("unknown target %q — use: up | push | deploy", targetStr)
	}

	activeEnv := strParam(params, "env")
	if activeEnv == "" {
		activeEnv = pilotenv.Active("")
	}

	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("preflight: load config: %w", err)
	}

	uc := preflight.New(preflight.RealDockerChecker{}, preflight.RealSSHChecker{})
	out, err := uc.Execute(ctx, preflight.Input{
		Target: target,
		Env:    activeEnv,
		Config: cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("preflight: %w", err)
	}

	result := map[string]any{
		"report":       out.Report,
		"lock_written": out.LockWritten,
	}
	if out.LockPath != "" {
		result["lock_path"] = out.LockPath
	}
	return result, nil
}
