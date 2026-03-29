// Package up implements the kaal up and kaal down command logic.
package up

import (
	"context"
	"fmt"
	"os"

	kaalctx "github.com/mouhamedsylla/kaal/internal/context"
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
}

// Run executes kaal up.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)

	if _, ok := cfg.Environments[activeEnv]; !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml\n  Run 'kaal env use <env>' or add it to kaal.yaml", activeEnv)
	}

	ui.Info(fmt.Sprintf("Starting environment %q", activeEnv))

	// Collect full project context — used to check what's missing
	// and to surface context to the agent if generation is needed
	projCtx, err := kaalctx.Collect(activeEnv)
	if err != nil {
		return err
	}

	// Guard: if required files are missing, stop and ask the agent
	if projCtx.MissingDockerfile || projCtx.MissingCompose {
		return missingFilesError(projCtx)
	}

	// Warn if env file is missing (non-blocking — compose can still start)
	envCfg := cfg.Environments[activeEnv]
	if envCfg.EnvFile != "" {
		if _, err := os.Stat(envCfg.EnvFile); os.IsNotExist(err) {
			ui.Warn(fmt.Sprintf("%s not found — services may fail to start without required variables", envCfg.EnvFile))
		}
	}

	// Start via orchestrator
	orch, err := runtime.NewOrchestrator(cfg, activeEnv)
	if err != nil {
		return err
	}

	if err := orch.Up(ctx, activeEnv, opts.Services); err != nil {
		return fmt.Errorf("docker compose failed: %w", err)
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Environment %q is up", activeEnv))
	printServiceURLs(cfg)
	return nil
}

// missingFilesError returns a rich error that tells the developer exactly
// what to ask their AI agent to generate.
func missingFilesError(projCtx *kaalctx.ProjectContext) error {
	var missing []string
	if projCtx.MissingDockerfile {
		missing = append(missing, "Dockerfile")
	}
	if projCtx.MissingCompose {
		missing = append(missing, fmt.Sprintf("docker-compose.%s.yml", projCtx.ActiveEnv))
	}

	fmt.Println()
	ui.Error(fmt.Sprintf("Missing: %v", missing))
	fmt.Println()

	ui.Bold("Ask your AI agent to generate them.")
	fmt.Println()
	ui.Dim("  Option 1 — via MCP (Claude Code, Cursor):")
	ui.Dim("    kaal mcp serve is already configured in .mcp.json")
	ui.Dim("    Ask Claude: \"Generate the missing infrastructure files for this project\"")
	ui.Dim("    Claude will call kaal_context to get the full project details,")
	ui.Dim("    then write the files directly.")
	fmt.Println()
	ui.Dim("  Option 2 — paste this context into any AI chat:")
	fmt.Println()

	// Print the agent prompt (truncated for terminal display)
	prompt := projCtx.AgentPrompt()
	lines := splitLines(prompt)
	for i, line := range lines {
		if i >= 40 {
			ui.Dim(fmt.Sprintf("  ... (%d more lines — use 'kaal context' to get the full prompt)", len(lines)-40))
			break
		}
		ui.Dim("  " + line)
	}

	fmt.Println()
	ui.Dim("  Run 'kaal context' to print the full agent prompt")
	ui.Dim("  Then re-run 'kaal up' once the files are created")
	fmt.Println()

	return fmt.Errorf("infrastructure files missing — see above")
}

// DownOptions controls kaal down behaviour.
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
		return fmt.Errorf("docker compose down failed: %w", err)
	}

	ui.Success(fmt.Sprintf("Environment %q stopped", activeEnv))
	return nil
}

func printServiceURLs(cfg *config.Config) {
	fmt.Println()
	for name, svc := range cfg.Services {
		if svc.Port > 0 {
			ui.Dim(fmt.Sprintf("  %-14s http://localhost:%d", name, svc.Port))
		}
	}
	fmt.Println()
	ui.Dim("kaal logs --follow   stream logs")
	ui.Dim("kaal down            stop services")
	ui.Dim("kaal status          inspect services")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
