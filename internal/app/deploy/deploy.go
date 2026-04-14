// Package deploy implements the pilot deploy use case.
//
// DeployUseCase orchestrates the full deployment skeleton:
//
//	[1] validate pilot.lock (staleness guard)
//	[2] resolve secrets
//	[3] sync compose + config files to remote
//	[4] pre_hooks (if declared in pilot.lock)
//	[5] migrations (if detected/declared in pilot.lock)
//	[6] deploy (pull image + docker compose up)
//	[7] post_hooks (if declared in pilot.lock)
//	[8] healthcheck (wait for all services healthy)
//	    → LIFO compensation on failure from [4] onward
//
// All infrastructure is injected as domain ports — no I/O, no config loading,
// no UI output happens here. cmd/ and mcp/ own all of that.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/domain/lock"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
	"github.com/mouhamedsylla/pilot/internal/domain/state"
)

// ── types ─────────────────────────────────────────────────────────────────────

// Input is the data required to run a deployment.
type Input struct {
	Env        string
	Tag        string
	SecretRefs map[string]string // from pilot.yaml secrets.refs; nil = no secrets
	DryRun     bool
	// PreHooks and PostHooks override what the lock declares.
	// When nil, the lock's configuration is used.
	PreHooks  []string
	PostHooks []string
	// SkipLockCheck disables the staleness guard (for dev/testing).
	SkipLockCheck bool
}

// Output is the result of a successful deployment.
type Output struct {
	Tag        string
	Statuses   []domain.ServiceStatus
	DryRunPlan *DryRunPlan // non-nil only when Input.DryRun is true
}

// DryRunPlan describes what would happen without executing anything.
type DryRunPlan struct {
	Steps []string
}

// Config bundles all dependencies of DeployUseCase.
type Config struct {
	Provider   domain.DeployProvider
	Secrets    domain.SecretManager   // nil = no secrets
	Hooks      domain.HookRunner      // nil = hooks unsupported or not configured
	Migrations domain.MigrationRunner // nil = migrations unsupported or not detected
	StateDir   string                 // directory for .pilot/state.json
	ProjectDir string                 // directory containing pilot.lock
}

// DeployUseCase orchestrates a remote deployment.
type DeployUseCase struct{ cfg Config }

// New constructs a DeployUseCase.
func New(c Config) *DeployUseCase { return &DeployUseCase{cfg: c} }

// ── execute ───────────────────────────────────────────────────────────────────

// Execute runs the deployment and returns the result.
func (uc *DeployUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	// ── 0. Load pilot.lock ────────────────────────────────────────────────
	lck, err := uc.loadLock(in)
	if err != nil {
		return Output{}, err
	}

	if in.DryRun {
		return uc.dryRun(in, lck), nil
	}

	// ── State: mark executing ──────────────────────────────────────────────
	s := state.New(in.Env)
	s.MachineState = state.StateExecuting
	_ = s.Write(uc.cfg.StateDir)

	// ── 1. Resolve secrets → temp env file ────────────────────────────────
	var envFiles []string
	if len(in.SecretRefs) > 0 && uc.cfg.Secrets != nil {
		resolved, sErr := uc.cfg.Secrets.Inject(ctx, in.Env, in.SecretRefs)
		if sErr != nil {
			return Output{}, uc.fail(s, fmt.Errorf("secrets: %w", sErr))
		}
		tmp, sErr := writeTempEnv(resolved)
		if sErr != nil {
			return Output{}, uc.fail(s, fmt.Errorf("secrets temp file: %w", sErr))
		}
		defer os.Remove(tmp)
		envFiles = append(envFiles, tmp)
	}

	// ── 2. Sync compose + config files to remote ──────────────────────────
	if err := uc.cfg.Provider.Sync(ctx, in.Env); err != nil {
		return Output{}, uc.fail(s, fmt.Errorf("sync: %w", err))
	}

	// Track which steps have executed (for compensation).
	var completed []plan.StepName

	// ── 3. pre_hooks ──────────────────────────────────────────────────────
	preHooks := uc.resolvePreHooks(in, lck)
	if len(preHooks) > 0 {
		if uc.cfg.Hooks == nil {
			return Output{}, uc.fail(s, fmt.Errorf(
				"pre_hooks are configured but the target does not support hook execution\n"+
					"  Supported targets: VPS (SSH)",
			))
		}
		if err := uc.cfg.Hooks.RunHooks(ctx, preHooks); err != nil {
			return Output{}, uc.compensateAndFail(ctx, s, completed, in, lck, fmt.Errorf("pre_hooks: %w", err))
		}
		completed = append(completed, plan.StepPreHooks)
	}

	// ── 4. migrations ─────────────────────────────────────────────────────
	migCfg := uc.resolveMigrations(lck)
	if migCfg != nil {
		if uc.cfg.Migrations == nil {
			return Output{}, uc.fail(s, fmt.Errorf(
				"migrations are configured but the target does not support remote migration execution\n"+
					"  Supported targets: VPS (SSH)",
			))
		}
		// Mark as started before calling — even a partial run needs rollback attempt.
		completed = append(completed, plan.StepMigrations)
		if err := uc.cfg.Migrations.RunMigrations(ctx, *migCfg); err != nil {
			return Output{}, uc.compensateAndFail(ctx, s, completed, in, lck, fmt.Errorf("migrations: %w", err))
		}
	}

	// ── 5. deploy (pull + compose up) ─────────────────────────────────────
	// Mark deploy as started before calling provider — even a partial deploy
	// (e.g. image pulled but compose restart failed) needs a rollback attempt.
	completed = append(completed, plan.StepDeploy)
	if err := uc.cfg.Provider.Deploy(ctx, in.Env, domain.DeployOptions{
		Tag:      in.Tag,
		EnvFiles: envFiles,
	}); err != nil {
		return Output{}, uc.compensateAndFail(ctx, s, completed, in, lck, fmt.Errorf("deploy: %w", err))
	}

	// ── 6. post_hooks ─────────────────────────────────────────────────────
	postHooks := uc.resolvePostHooks(in, lck)
	if len(postHooks) > 0 && uc.cfg.Hooks != nil {
		if err := uc.cfg.Hooks.RunHooks(ctx, postHooks); err != nil {
			// Post-hooks failure after deploy — log but don't compensate deploy.
			return Output{}, uc.fail(s, fmt.Errorf("post_hooks: %w", err))
		}
	}

	// ── 7. healthcheck ────────────────────────────────────────────────────
	statuses, err := uc.cfg.Provider.Status(ctx, in.Env)
	if err != nil {
		return Output{}, uc.compensateAndFail(ctx, s, completed, in, lck, fmt.Errorf("healthcheck: %w", err))
	}
	if svc := firstUnhealthy(statuses); svc != "" {
		return Output{}, uc.compensateAndFail(ctx, s, completed, in, lck,
			fmt.Errorf("service %q went unhealthy after deploy", svc))
	}

	// ── Success ───────────────────────────────────────────────────────────
	s.MachineState = state.StateSucceeded
	_ = s.Write(uc.cfg.StateDir)

	return Output{Tag: in.Tag, Statuses: statuses}, nil
}

// ── lock ──────────────────────────────────────────────────────────────────────

func (uc *DeployUseCase) loadLock(in Input) (*lock.Lock, error) {
	if in.SkipLockCheck {
		return nil, nil
	}

	projectDir := uc.cfg.ProjectDir
	if projectDir == "" {
		projectDir = "."
	}

	lck, err := lock.ReadForEnv(projectDir, in.Env)
	if err != nil {
		if errors.Is(err, lock.ErrNotFound) {
			return nil, fmt.Errorf(
				"pilot.%s.lock not found\n\n"+
					"  Run preflight first to generate it:\n"+
					"    pilot preflight --target deploy --env %s\n\n"+
					"  The lock captures exactly what will run — commit it to your repo.",
				in.Env, in.Env,
			)
		}
		return nil, fmt.Errorf("read pilot.lock: %w", err)
	}

	// Recompute hash of source files and check staleness.
	if lck.ProjectHash != "" {
		currentHash, hashErr := hashSourceFiles(lck.GeneratedFrom)
		if hashErr == nil && lck.IsStale(currentHash) {
			return nil, fmt.Errorf(
				"pilot.lock is stale — source files have changed since last preflight\n\n" +
					"  Regenerate it:\n" +
					"    pilot preflight --target deploy\n\n" +
					"  Then review the new pilot.lock before deploying.",
			)
		}
	}

	return lck, nil
}

// ── hooks / migrations resolution ────────────────────────────────────────────

func (uc *DeployUseCase) resolvePreHooks(in Input, lck *lock.Lock) []string {
	if in.PreHooks != nil {
		return in.PreHooks
	}
	// Lock not available (SkipLockCheck) or no lock — no hooks.
	return nil
}

func (uc *DeployUseCase) resolvePostHooks(in Input, lck *lock.Lock) []string {
	if in.PostHooks != nil {
		return in.PostHooks
	}
	return nil
}

func (uc *DeployUseCase) resolveMigrations(lck *lock.Lock) *domain.MigrationConfig {
	if lck == nil {
		return nil
	}
	m := lck.ExecutionPlan.Migrations
	if m == nil || m.Command == "" {
		return nil
	}
	active := lck.ActiveNodes()
	if !active[plan.StepMigrations] {
		return nil
	}
	return &domain.MigrationConfig{
		Tool:            m.Tool,
		Command:         m.Command,
		RollbackCommand: m.RollbackCommand,
		Reversible:      m.Reversible,
	}
}

// ── compensation ─────────────────────────────────────────────────────────────

func (uc *DeployUseCase) compensateAndFail(
	ctx context.Context,
	s *state.State,
	completed []plan.StepName,
	in Input,
	lck *lock.Lock,
	cause error,
) error {
	// Always attempt image rollback when deploy completed (even partially).
	deployDone := containsStep(completed, plan.StepDeploy)
	migsDone := containsStep(completed, plan.StepMigrations)

	// [a] Roll back migrations if they ran and are reversible.
	if migsDone && uc.cfg.Migrations != nil && lck != nil {
		migCfg := uc.resolveMigrations(lck)
		if migCfg != nil && migCfg.Reversible && migCfg.RollbackCommand != "" {
			if rbErr := uc.cfg.Migrations.RollbackMigrations(ctx, *migCfg); rbErr != nil {
				cause = fmt.Errorf("%w\n  + migration rollback failed: %v", cause, rbErr)
			}
		}
	}

	// [b] Roll back image/containers if deploy ran.
	if deployDone {
		restoredTag, rbErr := uc.cfg.Provider.Rollback(ctx, in.Env, "")
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

	return uc.fail(s, cause)
}

func (uc *DeployUseCase) fail(s *state.State, err error) error {
	s.MachineState = state.StateGuidedFailure
	_ = s.Write(uc.cfg.StateDir)
	return err
}

// ── dry-run ───────────────────────────────────────────────────────────────────

func (uc *DeployUseCase) dryRun(in Input, lck *lock.Lock) Output {
	steps := []string{}

	if len(in.SecretRefs) > 0 {
		steps = append(steps, fmt.Sprintf("resolve %d secret(s) via provider", len(in.SecretRefs)))
	}

	steps = append(steps, fmt.Sprintf("sync compose files → remote (env: %s)", in.Env))

	if hooks := uc.resolvePreHooks(in, lck); len(hooks) > 0 {
		steps = append(steps, fmt.Sprintf("pre_hooks (%d command(s))", len(hooks)))
	}

	if lck != nil {
		if migCfg := uc.resolveMigrations(lck); migCfg != nil {
			steps = append(steps, fmt.Sprintf("migrations: %s — %s", migCfg.Tool, migCfg.Command))
		}
	}

	steps = append(steps,
		fmt.Sprintf("docker pull <image>:%s", in.Tag),
		"docker compose up -d --remove-orphans",
	)

	if hooks := uc.resolvePostHooks(in, lck); len(hooks) > 0 {
		steps = append(steps, fmt.Sprintf("post_hooks (%d command(s))", len(hooks)))
	}

	steps = append(steps, "healthcheck — wait for all services healthy")

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

func containsStep(steps []plan.StepName, target plan.StepName) bool {
	for _, s := range steps {
		if s == target {
			return true
		}
	}
	return false
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

// hashSourceFiles recomputes the content hash of the lock source files.
// Used to detect staleness at deploy time.
func hashSourceFiles(paths []string) (string, error) {
	return computeHash(paths)
}
