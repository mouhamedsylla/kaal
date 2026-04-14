// Package up implements the pilot up / pilot down use cases.
package up

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/meta"
)

// ── Up ────────────────────────────────────────────────────────────────────────

// Input is the data required to start an environment.
type Input struct {
	Env        string
	Services   []string
	Config     *config.Config
	ProjectDir string // root dir for compose file lookup; empty → "."
}

// Output is the result of a successful pilot up.
type Output struct {
	Env            string
	IsRemoteEnv    bool   // env has a remote target — cmd/ should warn the user
	TargetName     string // non-empty when IsRemoteEnv is true
	MissingEnvFile string // non-empty if the env file is absent (non-blocking)
	StaleCompose   string // non-empty if compose file may be outdated (warning only, never a blocker)
}

// MissingComposeError is returned when the compose file doesn't exist.
type MissingComposeError struct {
	ComposePath string
	Env         string
}

func (e *MissingComposeError) Error() string {
	return fmt.Sprintf(
		"compose file not found: %s\n"+
			"  Ask your AI agent: \"Generate the missing infrastructure files for this project\"\n"+
			"  Or run manually:    pilot add <service>  to update pilot.yaml first",
		e.ComposePath,
	)
}

// StaleComposeError is returned when pilot.yaml has changed since the compose
// file was last generated. The compose file may no longer reflect the real infra.
type StaleComposeError struct {
	Env         string
	ComposeFile string
}

func (e *StaleComposeError) Error() string {
	return fmt.Sprintf(
		"docker-compose.%s.yml is stale — pilot.yaml has changed since it was generated\n\n"+
			"  Regenerate it:\n"+
			"    Ask your AI agent: \"Regenerate the compose file for the %s environment\"\n\n"+
			"  The agent will call pilot_generate_compose with the updated pilot.yaml.\n"+
			"  Your existing compose file will be replaced.",
		e.Env, e.Env,
	)
}

// UpUseCase starts the local environment.
type UpUseCase struct {
	provider domain.ExecutionProvider
}

// New constructs an UpUseCase.
func New(provider domain.ExecutionProvider) *UpUseCase {
	return &UpUseCase{provider: provider}
}

// Execute runs pilot up.
func (uc *UpUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	projectDir := in.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	envCfg, ok := in.Config.Environments[in.Env]
	if !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	// Compose file must exist before calling the runtime.
	composeFile := fmt.Sprintf("docker-compose.%s.yml", in.Env)
	composePath := filepath.Join(projectDir, composeFile)
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return Output{}, &MissingComposeError{ComposePath: composePath, Env: in.Env}
	}

	// Staleness check: advisory only — if pilot.yaml changed since the compose was
	// generated, surface a warning but never block. The user may have edited the
	// compose file manually and knows what they're doing.
	out := Output{Env: in.Env}
	if staleness, err := meta.CheckStaleness(projectDir, in.Env); err == nil && staleness.IsStale {
		out.StaleCompose = composeFile
	}

	if envCfg.Target != "" {
		out.IsRemoteEnv = true
		out.TargetName = envCfg.Target
	}

	if envCfg.EnvFile != "" {
		if _, err := os.Stat(envCfg.EnvFile); os.IsNotExist(err) {
			out.MissingEnvFile = envCfg.EnvFile
		}
	}

	if err := uc.provider.Up(ctx, in.Env, in.Services); err != nil {
		envFile := envCfg.EnvFile
		if envFile == "" {
			envFile = ".env"
		}
		return Output{}, fmt.Errorf(
			"docker compose up failed for environment %q\n\n"+
				"  Common causes:\n"+
				"  • Image not found in registry → pilot push --env %s first\n"+
				"  • Port already in use → check what's running on the configured ports\n"+
				"  • Missing env variable → check %s\n"+
				"  • Syntax error in compose file → ask your AI agent to fix it\n\n"+
				"  Cause: %w",
			in.Env, in.Env, envFile, err,
		)
	}

	return out, nil
}

// ── Down ──────────────────────────────────────────────────────────────────────

// DownInput is the data required to stop an environment.
type DownInput struct {
	Env    string
	Config *config.Config
}

// DownOutput is the result of a successful pilot down.
type DownOutput struct {
	Env string
}

// DownUseCase stops the local environment.
type DownUseCase struct {
	provider domain.ExecutionProvider
}

// NewDown constructs a DownUseCase.
func NewDown(provider domain.ExecutionProvider) *DownUseCase {
	return &DownUseCase{provider: provider}
}

// Execute runs pilot down.
func (uc *DownUseCase) Execute(ctx context.Context, in DownInput) (DownOutput, error) {
	if _, ok := in.Config.Environments[in.Env]; !ok {
		return DownOutput{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.Env)
	}

	if err := uc.provider.Down(ctx, in.Env); err != nil {
		return DownOutput{}, fmt.Errorf("docker compose down: %w", err)
	}

	return DownOutput{Env: in.Env}, nil
}
