// Package up implements the kaal up command logic.
// It resolves the active environment, ensures required files exist
// (generating them if absent), then delegates to the orchestrator.
package up

import (
	"context"
	"fmt"
	"os"

	"github.com/mouhamedsylla/kaal/internal/composer"
	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Options controls kaal up behaviour.
type Options struct {
	Env      string   // override active env
	Services []string // empty = all services
	Build    bool     // force image rebuild
	Detach   bool     // run in background (default true)
}

// Run executes kaal up.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml\n  Run 'kaal env use <env>' or add it to kaal.yaml", activeEnv)
	}

	ui.Info(fmt.Sprintf("Starting environment %q", activeEnv))

	// Step 1 — ensure Dockerfile exists for app services
	if hasAppService(cfg) {
		dockerfilePath, generated, err := composer.EnsureDockerfile(cfg)
		if err != nil {
			return err
		}
		if generated {
			ui.Success(fmt.Sprintf("Generated %s (stack: %s)", dockerfilePath, cfg.Project.Stack))
			ui.Dim("  Review it and commit if it looks right.")
		} else {
			ui.Dim(fmt.Sprintf("Using existing %s", dockerfilePath))
		}
	}

	// Step 2 — ensure docker-compose.<env>.yml exists
	composeFile := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		written, err := composer.GenerateCompose(cfg, composer.ComposeOptions{
			Env:     activeEnv,
			EnvFile: envCfg.EnvFile,
			IsDev:   activeEnv == "dev",
		})
		if err != nil {
			return fmt.Errorf("generate compose: %w", err)
		}
		ui.Success(fmt.Sprintf("Generated %s", written))
		ui.Dim("  Review it and commit if it looks right.")
	} else {
		ui.Dim(fmt.Sprintf("Using existing %s", composeFile))
	}

	// Step 3 — warn if env file is missing
	if envCfg.EnvFile != "" {
		if _, err := os.Stat(envCfg.EnvFile); os.IsNotExist(err) {
			ui.Warn(fmt.Sprintf("%s not found — create it from .env.example or add your variables", envCfg.EnvFile))
		}
	}

	// Step 4 — start services via orchestrator
	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return err
	}

	ui.Info(fmt.Sprintf("Running docker compose up (env: %s)", activeEnv))

	if err := orch.Up(ctx, activeEnv, opts.Services); err != nil {
		return fmt.Errorf("kaal up failed: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Environment %q is up", activeEnv))
	printServiceURLs(cfg, activeEnv)
	return nil
}

// Down stops services for the active environment.
type DownOptions struct {
	Env     string
	Volumes bool
}

// RunDown executes kaal down.
func RunDown(ctx context.Context, opts DownOptions) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)
	if _, ok := cfg.Environments[activeEnv]; !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml", activeEnv)
	}

	ui.Info(fmt.Sprintf("Stopping environment %q", activeEnv))

	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return err
	}

	if err := orch.Down(ctx, activeEnv); err != nil {
		return fmt.Errorf("kaal down failed: %w", err)
	}

	ui.Success(fmt.Sprintf("Environment %q stopped", activeEnv))
	return nil
}

func hasAppService(cfg *config.Config) bool {
	for _, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp {
			return true
		}
	}
	return false
}

func printServiceURLs(cfg *config.Config, envName string) {
	envCfg := cfg.Environments[envName]
	_ = envCfg

	fmt.Println()
	for name, svc := range cfg.Services {
		if svc.Port > 0 {
			ui.Dim(fmt.Sprintf("  %-12s http://localhost:%d", name, svc.Port))
		}
	}
	fmt.Println()
	ui.Dim("kaal logs --follow   to stream logs")
	ui.Dim("kaal down            to stop")
	ui.Dim("kaal status          to inspect services")
}
