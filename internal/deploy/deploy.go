// Package deploy implements the pilot deploy command logic.
package deploy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/gitutil"
	"github.com/mouhamedsylla/pilot/internal/providers"
	"github.com/mouhamedsylla/pilot/internal/runtime"
	"github.com/mouhamedsylla/pilot/pkg/ui"
)

// Options controls pilot deploy behaviour.
type Options struct {
	Env        string // override active env
	Tag        string // image tag; empty = git short SHA
	Target     string // override target from pilot.yaml
	Strategy   string // rolling | blue-green | canary
	DryRun     bool
	NoRollback bool // skip auto-rollback on healthcheck failure
}

// Run executes pilot deploy: sync files → pull image → compose up.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in pilot.yaml", activeEnv)
	}

	// Resolve target
	targetName := opts.Target
	if targetName == "" {
		targetName = envCfg.Target
	}
	if targetName == "" {
		return fmt.Errorf(
			"no deploy target for environment %q\n  Add: environments.%s.target: <target-name>",
			activeEnv, activeEnv,
		)
	}
	target, ok := cfg.Targets[targetName]
	if !ok {
		return fmt.Errorf("target %q not defined in pilot.yaml", targetName)
	}

	// Resolve tag
	tag, err := resolveTag(opts.Tag)
	if err != nil {
		return err
	}

	// Verify the compose file exists locally before trying to copy it
	composeFile := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf(
			"%s not found — generate it first with your AI agent or 'pilot context'",
			composeFile,
		)
	}

	if opts.Strategy != "" && opts.Strategy != "rolling" {
		ui.Warn(fmt.Sprintf(
			"Strategy %q is not yet implemented — falling back to rolling update\n"+
				"  canary/blue-green require Traefik or Kubernetes; see docs/workflows/deploy-vps.md",
			opts.Strategy,
		))
	}

	if opts.DryRun {
		return printDryRun(cfg, activeEnv, targetName, target, tag, composeFile)
	}

	ui.Info(fmt.Sprintf("Deploying %s to %s (%s:%s)", activeEnv, targetName, target.Type, target.Host))

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
	}

	// Resolve secrets and write to a temp env file for deployment.
	var deployEnvFiles []string
	if envCfg.Secrets != nil && len(envCfg.Secrets.Refs) > 0 {
		ui.Info(fmt.Sprintf("Resolving secrets via %q", envCfg.Secrets.Provider))
		sm, err := runtime.NewSecretManager(envCfg.Secrets.Provider)
		if err != nil {
			return fmt.Errorf("secrets: %w", err)
		}
		resolved, err := sm.Inject(ctx, activeEnv, envCfg.Secrets.Refs)
		if err != nil {
			return fmt.Errorf("secrets inject: %w", err)
		}

		// Write resolved secrets to a temp env file.
		tmpFile, err := writeTempEnv(resolved)
		if err != nil {
			return fmt.Errorf("write resolved env: %w", err)
		}
		defer os.Remove(tmpFile)
		deployEnvFiles = append(deployEnvFiles, tmpFile)
		ui.Success(fmt.Sprintf("Resolved %d secret(s)", len(resolved)))
	}

	// Sync compose file (and env file if present) to the remote
	ui.Info("Syncing files to remote")
	if err := provider.Sync(ctx, activeEnv); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// Pull image + restart services
	ui.Info(fmt.Sprintf("Pulling image and restarting services (tag: %s)", tag))
	if err := provider.Deploy(ctx, activeEnv, providers.DeployOptions{
		Tag:      tag,
		Strategy: opts.Strategy,
		EnvFiles: deployEnvFiles,
	}); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	// ── Post-deploy: health check with optional auto-rollback ────────────
	fmt.Println()
	if opts.NoRollback {
		// Quick status snapshot, no waiting.
		statuses, _ := provider.Status(ctx, activeEnv)
		ui.Success(fmt.Sprintf("Deployed %s:%s → %s (%s)", cfg.Registry.Image, tag, targetName, target.Host))
		printStatuses(statuses)
	} else {
		failedSvc, err := waitForHealth(ctx, provider, activeEnv)
		if err != nil {
			// A service went unhealthy — auto-rollback.
			ui.Warn(fmt.Sprintf("Service %q went unhealthy — rolling back automatically", failedSvc))
			fmt.Println()
			prevTag, rbErr := provider.Rollback(ctx, activeEnv, "")
			if rbErr != nil {
				return fmt.Errorf(
					"deploy succeeded but service %q went unhealthy AND auto-rollback failed: %v\n\n"+
						"  Manual recovery on VPS:\n"+
						"    docker compose -f ~/pilot/docker-compose.%s.yml down\n"+
						"    docker compose -f ~/pilot/docker-compose.%s.yml up -d\n\n"+
						"  Or force a specific tag:\n"+
						"    pilot rollback --env %s --version <previous-tag>",
					failedSvc, rbErr, activeEnv, activeEnv, activeEnv,
				)
			}
			return fmt.Errorf(
				"deploy of %s went unhealthy — auto-rolled back to %s\n\n"+
					"  Investigate:\n"+
					"    pilot logs --env %s --follow\n"+
					"    pilot status --env %s",
				tag, prevTag, activeEnv, activeEnv,
			)
		}

		statuses, _ := provider.Status(ctx, activeEnv)
		fmt.Println()
		ui.Success(fmt.Sprintf("Deployed %s:%s → %s (%s)", cfg.Registry.Image, tag, targetName, target.Host))
		printStatuses(statuses)
	}

	ui.Dim(fmt.Sprintf("  pilot logs --env %s --follow", activeEnv))
	ui.Dim(fmt.Sprintf("  pilot status --env %s", activeEnv))
	ui.Dim(fmt.Sprintf("  pilot rollback --env %s   (si quelque chose cloche)", activeEnv))
	fmt.Println()

	return nil
}

// waitForHealth polls service health after a deploy, up to 60 seconds.
//
// Returns ("", nil)        — all services healthy or running (no healthcheck)
// Returns ("svc", error)   — service "svc" went unhealthy → caller should rollback
// Returns ("", nil)        — timeout: services still starting, non-fatal warning
func waitForHealth(ctx context.Context, provider providers.Provider, env string) (string, error) {
	const (
		pollInterval = 5 * time.Second
		maxWait      = 60 * time.Second
	)

	ui.Info("Verifying service health")
	deadline := time.Now().Add(maxWait)
	elapsed := 0

	for time.Now().Before(deadline) {
		statuses, err := provider.Status(ctx, env)
		if err != nil {
			time.Sleep(pollInterval)
			elapsed += int(pollInterval.Seconds())
			continue
		}

		var report []string
		unhealthySvc := ""
		allOK := true

		for _, s := range statuses {
			label := s.State
			if s.Health != "" {
				label = s.Health
			}
			report = append(report, fmt.Sprintf("%s=%s", s.Name, label))

			switch s.Health {
			case "unhealthy":
				unhealthySvc = s.Name
				allOK = false
			case "starting":
				allOK = false
			case "":
				if s.State != "running" {
					allOK = false
				}
			}
		}

		ui.Dim(fmt.Sprintf("  [%ds] %s", elapsed, strings.Join(report, "  ")))

		if unhealthySvc != "" {
			return unhealthySvc, fmt.Errorf("service %q went unhealthy", unhealthySvc)
		}
		if allOK && len(statuses) > 0 {
			ui.Success("All services healthy")
			return "", nil
		}

		time.Sleep(pollInterval)
		elapsed += int(pollInterval.Seconds())
	}

	ui.Warn(fmt.Sprintf("Health check timed out after %ds — services may still be starting", int(maxWait.Seconds())))
	ui.Dim(fmt.Sprintf("  Monitor with: pilot status --env %s", env))
	return "", nil // non-fatal timeout
}

func printStatuses(statuses []providers.ServiceStatus) {
	if len(statuses) == 0 {
		return
	}
	for _, s := range statuses {
		health := s.Health
		if health == "" {
			health = s.State
		}
		ui.Dim(fmt.Sprintf("  %-20s %s", s.Name, health))
	}
	fmt.Println()
}

// printDryRun shows what would happen without executing.
func printDryRun(cfg *config.Config, activeEnv, targetName string, target config.Target, tag, composeFile string) error {
	fmt.Println()
	ui.Bold("Dry run — nothing will be executed")
	fmt.Println()
	ui.Dim(fmt.Sprintf("  Environment : %s", activeEnv))
	ui.Dim(fmt.Sprintf("  Target      : %s (%s@%s:%d)", targetName, target.User, target.Host, targetPort(target)))
	ui.Dim(fmt.Sprintf("  Image       : %s:%s", cfg.Registry.Image, tag))
	ui.Dim(fmt.Sprintf("  Compose     : %s", composeFile))
	fmt.Println()
	ui.Dim("  Steps that would run:")
	ui.Dim(fmt.Sprintf("    1. SCP %s → %s:~/pilot/", composeFile, target.Host))
	ui.Dim(fmt.Sprintf("    2. docker pull %s:%s", cfg.Registry.Image, tag))
	ui.Dim(fmt.Sprintf("    3. IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", tag, composeFile))
	fmt.Println()
	return nil
}

func targetPort(t config.Target) int {
	if t.Port == 0 {
		return 22
	}
	return t.Port
}

// resolveTag returns the explicit tag or the git short SHA of HEAD.
func resolveTag(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	return gitutil.ShortSHA()
}

// writeTempEnv writes resolved secrets to a temporary env file (0600).
// Returns the file path; caller is responsible for removing it.
func writeTempEnv(vars map[string]string) (string, error) {
	f, err := os.CreateTemp("", "pilot-env-*.env")
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	for k, v := range vars {
		if strings.ContainsAny(v, " \t\"'") {
			sb.WriteString(fmt.Sprintf(`%s="%s"`, k, v))
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteByte('\n')
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
