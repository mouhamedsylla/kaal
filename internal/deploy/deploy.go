// Package deploy implements the kaal deploy command logic.
package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/env"
	"github.com/mouhamedsylla/kaal/internal/gitutil"
	"github.com/mouhamedsylla/kaal/internal/providers"
	"github.com/mouhamedsylla/kaal/internal/runtime"
	"github.com/mouhamedsylla/kaal/pkg/ui"
)

// Options controls kaal deploy behaviour.
type Options struct {
	Env      string // override active env
	Tag      string // image tag; empty = git short SHA
	Target   string // override target from kaal.yaml
	Strategy string // rolling | blue-green | canary
	DryRun   bool
}

// Run executes kaal deploy: sync files → pull image → compose up.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(".")
	if err != nil {
		return err
	}

	activeEnv := env.Active(opts.Env)

	envCfg, ok := cfg.Environments[activeEnv]
	if !ok {
		return fmt.Errorf("environment %q not defined in kaal.yaml", activeEnv)
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
		return fmt.Errorf("target %q not defined in kaal.yaml", targetName)
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
			"%s not found — generate it first with your AI agent or 'kaal context'",
			composeFile,
		)
	}

	if opts.DryRun {
		return printDryRun(cfg, activeEnv, targetName, target, tag, composeFile)
	}

	ui.Info(fmt.Sprintf("Deploying %s to %s (%s:%s)", activeEnv, targetName, target.Type, target.Host))

	provider, err := runtime.NewProvider(cfg, targetName)
	if err != nil {
		return err
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
	}); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	// Post-deploy status check
	statuses, err := provider.Status(ctx, activeEnv)
	if err != nil {
		// Non-blocking — deploy succeeded even if status check fails
		ui.Warn(fmt.Sprintf("Could not retrieve post-deploy status: %v", err))
	}

	fmt.Println()
	ui.Success(fmt.Sprintf("Deployed %s:%s → %s (%s)", cfg.Registry.Image, tag, targetName, target.Host))
	fmt.Println()

	if len(statuses) > 0 {
		for _, s := range statuses {
			health := s.Health
			if health == "" {
				health = s.State
			}
			ui.Dim(fmt.Sprintf("  %-16s %s", s.Name, health))
		}
		fmt.Println()
	}

	ui.Dim(fmt.Sprintf("  kaal logs --env %s --follow", activeEnv))
	ui.Dim(fmt.Sprintf("  kaal status --env %s", activeEnv))
	ui.Dim(fmt.Sprintf("  kaal rollback --env %s   (if something looks wrong)", activeEnv))
	fmt.Println()

	return nil
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
	ui.Dim(fmt.Sprintf("    1. SCP %s → %s:~/kaal/", composeFile, target.Host))
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
