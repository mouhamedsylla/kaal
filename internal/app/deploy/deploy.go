// Package deploy implements the pilot deploy use case.
//
// DeployUseCase orchestrates the deployment skeleton:
//
//	sync → deploy → healthcheck → (rollback on failure)
//
// It has no I/O, no config loading, and no UI output.
// All infrastructure concerns are handled by injected ports.
// cmd/ and mcp/ are responsible for constructing the use case and
// formatting the output.
package deploy

import (
	"context"
	"fmt"
	"os"
	"strings"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/domain/state"
)

// Input is the data required to run a deployment.
type Input struct {
	Env        string
	Tag        string
	SecretRefs map[string]string // from pilot.yaml secrets.refs; nil = no secrets
	DryRun     bool
}

// Output is the result of a successful deployment.
type Output struct {
	Tag        string
	Statuses   []domain.ServiceStatus
	DryRunPlan *DryRunPlan // non-nil only when Input.DryRun is true
}

// DryRunPlan describes what would run without executing anything.
type DryRunPlan struct {
	Steps []string
}

// DeployUseCase orchestrates a remote deployment.
type DeployUseCase struct {
	provider domain.DeployProvider
	secrets  domain.SecretManager // nil when no secrets are configured
	stateDir string               // directory for .pilot/state.json
}

// New constructs a DeployUseCase. secrets may be nil.
func New(provider domain.DeployProvider, secrets domain.SecretManager, stateDir string) *DeployUseCase {
	return &DeployUseCase{
		provider: provider,
		secrets:  secrets,
		stateDir: stateDir,
	}
}

// Execute runs the deployment and returns the result.
// On healthcheck or deploy failure it triggers automatic rollback before returning an error.
func (uc *DeployUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.DryRun {
		return uc.dryRun(in), nil
	}

	s := state.New(in.Env)
	s.MachineState = state.StateExecuting
	_ = s.Write(uc.stateDir) // best-effort — don't abort on state write failure

	// [1] Resolve secrets → write temp env file.
	var envFiles []string
	if len(in.SecretRefs) > 0 && uc.secrets != nil {
		resolved, err := uc.secrets.Inject(ctx, in.Env, in.SecretRefs)
		if err != nil {
			return Output{}, uc.fail(s, fmt.Errorf("secrets: %w", err))
		}
		tmp, err := writeTempEnv(resolved)
		if err != nil {
			return Output{}, uc.fail(s, fmt.Errorf("secrets temp file: %w", err))
		}
		defer os.Remove(tmp)
		envFiles = append(envFiles, tmp)
	}

	// [2] Sync compose + env files to remote.
	if err := uc.provider.Sync(ctx, in.Env); err != nil {
		return Output{}, uc.fail(s, fmt.Errorf("sync: %w", err))
	}

	// [3] Deploy — pull image + restart services.
	if err := uc.provider.Deploy(ctx, in.Env, domain.DeployOptions{
		Tag:      in.Tag,
		EnvFiles: envFiles,
	}); err != nil {
		return Output{}, uc.rollbackAndFail(ctx, s, in, fmt.Errorf("deploy: %w", err))
	}

	// [4] Healthcheck.
	statuses, err := uc.provider.Status(ctx, in.Env)
	if err != nil {
		return Output{}, uc.rollbackAndFail(ctx, s, in, fmt.Errorf("healthcheck: %w", err))
	}
	if svc := firstUnhealthy(statuses); svc != "" {
		return Output{}, uc.rollbackAndFail(ctx, s, in,
			fmt.Errorf("service %q went unhealthy after deploy", svc))
	}

	// Success.
	s.MachineState = state.StateSucceeded
	_ = s.Write(uc.stateDir)

	return Output{Tag: in.Tag, Statuses: statuses}, nil
}

// ── compensation ─────────────────────────────────────────────────────────────

func (uc *DeployUseCase) rollbackAndFail(ctx context.Context, s *state.State, in Input, cause error) error {
	restoredTag, rbErr := uc.provider.Rollback(ctx, in.Env, "")
	if rbErr != nil {
		return uc.fail(s, fmt.Errorf(
			"%w — rollback also failed: %v\n\n"+
				"  Manual recovery:\n"+
				"    pilot rollback --env %s --version <previous-tag>",
			cause, rbErr, in.Env,
		))
	}
	return uc.fail(s, fmt.Errorf(
		"%w — auto-rolled back to %s\n\n"+
			"  Investigate:\n"+
			"    pilot logs --env %s --follow",
		cause, restoredTag, in.Env,
	))
}

func (uc *DeployUseCase) fail(s *state.State, err error) error {
	s.MachineState = state.StateGuidedFailure
	_ = s.Write(uc.stateDir)
	return err
}

// ── dry-run ───────────────────────────────────────────────────────────────────

func (uc *DeployUseCase) dryRun(in Input) Output {
	steps := []string{
		fmt.Sprintf("sync compose files → remote (env: %s)", in.Env),
		fmt.Sprintf("docker pull <image>:%s", in.Tag),
		"docker compose up -d --remove-orphans",
		"healthcheck — wait for all services healthy",
	}
	if len(in.SecretRefs) > 0 {
		steps = append([]string{
			fmt.Sprintf("resolve %d secret(s) via provider", len(in.SecretRefs)),
		}, steps...)
	}
	return Output{Tag: in.Tag, DryRunPlan: &DryRunPlan{Steps: steps}}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func firstUnhealthy(statuses []domain.ServiceStatus) string {
	for _, s := range statuses {
		if s.Health == "unhealthy" {
			return s.Name
		}
	}
	return ""
}

func writeTempEnv(vars map[string]string) (string, error) {
	f, err := os.CreateTemp("", "pilot-env-*.env")
	if err != nil {
		return "", err
	}
	defer f.Close()
	var sb strings.Builder
	for k, v := range vars {
		if strings.ContainsAny(v, " \t\"'") {
			sb.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, v))
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
		}
	}
	if _, err := f.WriteString(sb.String()); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := os.Chmod(f.Name(), 0600); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
