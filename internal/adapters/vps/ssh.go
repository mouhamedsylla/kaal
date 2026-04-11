package vps

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/piloterr"
	pilotSSH "github.com/mouhamedsylla/pilot/pkg/ssh"
	"github.com/mouhamedsylla/pilot/pkg/ui"
	"gopkg.in/yaml.v3"
)

// Provider deploys to a VPS via SSH + docker compose.
type Provider struct {
	cfg    *config.Config
	target config.Target
}

func New(cfg *config.Config, target config.Target) *Provider {
	return &Provider{cfg: cfg, target: target}
}

func (p *Provider) Deploy(ctx context.Context, env string, opts domain.DeployOptions) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	image := fmt.Sprintf("%s:%s", p.cfg.Registry.Image, opts.Tag)
	composeFile := remoteComposeFile(env) // always ~/pilot/docker-compose.<env>.yml
	stateDir := p.stateDir()

	// Copy resolved env files to remote before running compose.
	if len(opts.EnvFiles) > 0 {
		if err := client.CopyFiles(ctx, opts.EnvFiles, "~/pilot/"); err != nil {
			p.recordDeploy(ctx, client, env, opts.Tag, false, err.Error())
			return fmt.Errorf("sync env files: %w", err)
		}
	}

	// Build --env-file flags for docker compose.
	envFileFlags := ""
	for _, f := range opts.EnvFiles {
		base := f
		if idx := strings.LastIndex(f, "/"); idx >= 0 {
			base = f[idx+1:]
		}
		envFileFlags += fmt.Sprintf(" --env-file ~/pilot/%s", base)
	}

	// ── Step 1: prepare remote state (fast ops, spinner) ──────────────────
	setupCmds := []string{
		fmt.Sprintf("mkdir -p %s", stateDir),
		fmt.Sprintf("[ -f %s/current-tag ] && cp %s/current-tag %s/prev-tag || true", stateDir, stateDir, stateDir),
		fmt.Sprintf("echo %s > %s/current-tag", opts.Tag, stateDir),
	}
	if err := ui.Spinner("Preparing remote state", func() error {
		for _, cmd := range setupCmds {
			if _, err := client.Run(ctx, cmd); err != nil {
				return fmt.Errorf("state setup: %w", err)
			}
		}
		return nil
	}); err != nil {
		p.recordDeploy(ctx, client, env, opts.Tag, false, err.Error())
		return &piloterr.DeployError{Phase: "state", Target: p.target.Host, Cause: err}
	}

	// ── Step 2: docker pull — stream output ────────────────────────────────
	// Always capture in a buffer for error reporting.
	// Only forward to the terminal when stdout is a real TTY (not MCP/CI pipe).
	streamed := ui.IsTerminal()
	ui.Info(fmt.Sprintf("Pulling %s", image))
	pullCmd := fmt.Sprintf("docker pull %s", image)
	var pullBuf strings.Builder
	if err := client.RunWithOutput(ctx, pullCmd, remoteOutputWriter(&pullBuf)); err != nil {
		p.recordDeploy(ctx, client, env, opts.Tag, false, pullBuf.String())
		return &piloterr.DeployError{
			Phase:    "pull",
			Target:   p.target.Host,
			Command:  pullCmd,
			Output:   pullBuf.String(),
			Streamed: streamed,
			Cause: fmt.Errorf(
				"%w\n\n  Hints:\n"+
					"  • Verify the image tag %q exists in the registry\n"+
					"  • Check your registry credentials: pilot preflight --target push\n"+
					"  • If credentials expired, re-export and retry",
				err, opts.Tag,
			),
		}
	}

	// ── Step 3: docker compose up — stream output ─────────────────────────
	ui.Info("Restarting services")
	composeCmd := fmt.Sprintf(
		"IMAGE_TAG=%s docker compose -f %s%s up -d --remove-orphans",
		opts.Tag, composeFile, envFileFlags,
	)
	var composeBuf strings.Builder
	if err := client.RunWithOutput(ctx, composeCmd, remoteOutputWriter(&composeBuf)); err != nil {
		cause := err
		capturedOutput := composeBuf.String()
		if isDockerPermissionError(capturedOutput) {
			cause = fmt.Errorf(
				"user %q is not in the docker group on %s\n\n"+
					"  Fix automatically:\n"+
					"    pilot setup --env %s\n\n"+
					"  Or manually on your VPS:\n"+
					"    sudo usermod -aG docker %s\n"+
					"  Then reconnect and run pilot deploy again",
				p.target.User, p.target.Host, env, p.target.User,
			)
		}
		p.recordDeploy(ctx, client, env, opts.Tag, false, capturedOutput)
		return &piloterr.DeployError{
			Phase:    "restart",
			Target:   p.target.Host,
			Command:  composeCmd,
			Output:   capturedOutput,
			Streamed: streamed,
			Cause:    cause,
		}
	}

	p.recordDeploy(ctx, client, env, opts.Tag, true, "")
	return nil
}

// remoteOutputWriter returns a writer that:
//   - always captures into buf (for error reporting and AI agents)
//   - additionally forwards each line to stdout with a 2-space indent,
//     but ONLY when stdout is an interactive terminal (not MCP/CI mode).
func remoteOutputWriter(buf *strings.Builder) io.Writer {
	if ui.IsTerminal() {
		return io.MultiWriter(ui.PrefixWriter(os.Stdout, "  "), buf)
	}
	return buf
}

func (p *Provider) Sync(ctx context.Context, _ string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	// ── Flat files: pilot.yaml + compose files + env_file ──────────────────
	// These all land directly in ~/pilot/ (no subdirectory).
	seen := map[string]bool{}
	var flatFiles []string

	addFlat := func(f string) {
		if f == "" || seen[f] {
			return
		}
		if _, statErr := os.Stat(f); statErr == nil {
			seen[f] = true
			flatFiles = append(flatFiles, f)
		}
	}

	addFlat("pilot.yaml")
	for envName, envCfg := range p.cfg.Environments {
		composeFile := composeFileForEnv(envName)
		addFlat(composeFile)
		addFlat(envCfg.EnvFile)
	}

	if err := client.CopyFiles(ctx, flatFiles, "~/pilot/"); err != nil {
		return fmt.Errorf("sync flat files: %w", err)
	}
	// Print each flat file after the batch copy succeeds.
	for _, f := range flatFiles {
		ui.Dim(fmt.Sprintf("  ✓ %s", filepath.Base(f)))
	}

	// ── Bind-mount config files: preserve relative path under ~/pilot/ ─────
	// Scan every compose file for local bind-mounts (./nginx/prod.conf, etc.).
	// If the source is a local file, copy it to ~/pilot/<relative-path> so
	// docker compose running from ~/pilot/ finds it exactly where it expects.
	seenMounts := map[string]bool{}
	for envName := range p.cfg.Environments {
		composeFile := composeFileForEnv(envName)
		mounts, parseErr := parseComposeMounts(composeFile)
		if parseErr != nil {
			continue // compose file may not exist yet — not an error
		}
		for _, localSrc := range mounts {
			if seenMounts[localSrc] {
				continue
			}
			seenMounts[localSrc] = true
			info, statErr := os.Stat(localSrc)
			if statErr != nil {
				continue // file doesn't exist locally — skip silently
			}
			if info.IsDir() {
				continue // directories handled separately (not supported yet)
			}
			// Remote path mirrors the local relative path: ~/pilot/nginx/prod.conf
			remotePath := fmt.Sprintf("~/pilot/%s", strings.TrimPrefix(localSrc, "./"))
			if copyErr := client.CopyFileTo(ctx, localSrc, remotePath); copyErr != nil {
				return fmt.Errorf("sync bind-mount %s: %w", localSrc, copyErr)
			}
			ui.Dim(fmt.Sprintf("  ✓ %s", strings.TrimPrefix(localSrc, "./")))
		}
	}

	// ── Auto-reload nginx for any service whose config was just synced ─────
	// nginx keeps its config in memory; a file update on disk has no effect
	// until nginx reloads. We detect nginx services in each compose file and
	// run "nginx -s reload" inside the container — no restart, zero downtime.
	for envName := range p.cfg.Environments {
		composeFile := composeFileForEnv(envName)
		svcs := nginxServicesWithUpdatedMounts(composeFile, seenMounts)
		for _, svc := range svcs {
			reloadCmd := fmt.Sprintf(
				"docker compose -f %s exec -T %s nginx -s reload",
				remoteComposeFile(envName), svc,
			)
			reloadErr := ui.Spinner(fmt.Sprintf("Reloading nginx (%s)", svc), func() error {
				_, runErr := client.Run(ctx, reloadCmd)
				return runErr
			})
			if reloadErr != nil {
				// Non-fatal: container may not be running yet (first deploy pending).
				ui.Warn(fmt.Sprintf("nginx reload skipped — %s not running on remote", svc))
				ui.Dim(fmt.Sprintf("  Start it with: pilot deploy --env %s", envName))
			} else {
				ui.Success(fmt.Sprintf("nginx reloaded (%s)", svc))
			}
		}
	}

	return nil
}

// parseComposeMounts reads a docker-compose file and returns all local bind-mount
// source paths (relative paths starting with ./ or plain filenames, not named volumes
// or absolute paths). Supports both short syntax (./src:/dest) and long syntax.
func parseComposeMounts(composeFile string) ([]string, error) {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, err
	}

	// Minimal compose structure for volume parsing.
	var compose struct {
		Services map[string]struct {
			Volumes []any `yaml:"volumes"`
		} `yaml:"services"`
	}

	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var mounts []string

	for _, svc := range compose.Services {
		for _, v := range svc.Volumes {
			var src string
			switch vol := v.(type) {
			case string:
				// Short syntax: "./nginx/prod.conf:/etc/nginx/conf.d/default.conf:ro"
				parts := strings.SplitN(vol, ":", 2)
				src = parts[0]
			case map[string]any:
				// Long syntax: {type: bind, source: ./nginx/prod.conf, target: ...}
				if t, _ := vol["type"].(string); t != "bind" {
					continue
				}
				src, _ = vol["source"].(string)
			}
			// Only collect relative local paths — skip named volumes and absolute paths.
			if src == "" || filepath.IsAbs(src) {
				continue
			}
			if !strings.HasPrefix(src, "./") && !strings.HasPrefix(src, "../") {
				// Could be a named volume (e.g. "postgres_data:/var/lib/postgresql")
				// Only collect if it looks like a file path (has an extension or subdir).
				if !strings.Contains(src, "/") && !strings.Contains(src, ".") {
					continue
				}
			}
			if !seen[src] {
				seen[src] = true
				mounts = append(mounts, src)
			}
		}
	}

	return mounts, nil
}

func (p *Provider) Status(ctx context.Context, env string) ([]domain.ServiceStatus, error) {
	client, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	out, err := client.Run(ctx, fmt.Sprintf("docker compose -f %s ps --format json", remoteComposeFile(env)))
	if err != nil {
		return nil, fmt.Errorf("remote status: %w", err)
	}
	return parseRemotePS(out), nil
}

func (p *Provider) Logs(ctx context.Context, env string, service string, opts domain.LogOptions) (<-chan string, error) {
	client, err := p.connect()
	if err != nil {
		return nil, err
	}

	composeFile := remoteComposeFile(env)
	args := fmt.Sprintf("docker compose -f %s logs", composeFile)
	if opts.Follow {
		args += " --follow"
	}
	if opts.Since != "" {
		args += fmt.Sprintf(" --since %s", opts.Since)
	}
	if opts.Lines > 0 {
		args += fmt.Sprintf(" --tail %d", opts.Lines)
	}
	if service != "" {
		args += " " + service
	}

	ch, err := client.Stream(ctx, args)
	if err != nil {
		client.Close()
		return nil, err
	}

	// Close SSH connection when streaming ends
	go func() {
		for range ch {
		}
		client.Close()
	}()

	return ch, nil
}

func (p *Provider) Rollback(ctx context.Context, env string, version string) (string, error) {
	client, err := p.connect()
	if err != nil {
		return "", err
	}
	defer client.Close()

	// Resolve version: explicit tag or read prev-tag from VPS state.
	tag := version
	if tag == "" {
		var out string
		if err := ui.Spinner("Reading previous deployment tag", func() error {
			var runErr error
			out, runErr = client.Run(ctx, fmt.Sprintf("cat %s/prev-tag 2>/dev/null", p.stateDir()))
			return runErr
		}); err != nil || strings.TrimSpace(out) == "" {
			return "", fmt.Errorf(
				"no previous deployment found on %s\n  Use --version <tag> to specify a version explicitly",
				p.target.Host,
			)
		}
		tag = strings.TrimSpace(out)
	}

	image := fmt.Sprintf("%s:%s", p.cfg.Registry.Image, tag)
	composeFile := remoteComposeFile(env)
	stateDir := p.stateDir()

	streamed := ui.IsTerminal()

	// docker pull
	ui.Info(fmt.Sprintf("Pulling %s", image))
	pullCmd := fmt.Sprintf("docker pull %s", image)
	var pullBuf strings.Builder
	if err := client.RunWithOutput(ctx, pullCmd, remoteOutputWriter(&pullBuf)); err != nil {
		p.recordDeploy(ctx, client, env, tag, false, "rollback pull: "+err.Error())
		return "", &piloterr.DeployError{
			Phase: "rollback-pull", Target: p.target.Host,
			Command: pullCmd, Output: pullBuf.String(),
			Streamed: streamed, Cause: err,
		}
	}

	// docker compose up
	ui.Info("Restarting services")
	composeCmd := fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", tag, composeFile)
	var composeBuf strings.Builder
	if err := client.RunWithOutput(ctx, composeCmd, remoteOutputWriter(&composeBuf)); err != nil {
		p.recordDeploy(ctx, client, env, tag, false, "rollback restart: "+err.Error())
		return "", &piloterr.DeployError{
			Phase: "rollback-restart", Target: p.target.Host,
			Command: composeCmd, Output: composeBuf.String(),
			Streamed: streamed, Cause: err,
		}
	}

	// Persist the rolled-back tag as current.
	if _, saveErr := client.Run(ctx, fmt.Sprintf("echo %s > %s/current-tag", tag, stateDir)); saveErr != nil {
		ui.Warn(fmt.Sprintf("Could not save current-tag on remote: %v", saveErr))
	}

	p.recordDeploy(ctx, client, env, tag, true, "rollback")
	return tag, nil
}

// stateDir returns the remote path where pilot stores deploy state for this project.
func (p *Provider) stateDir() string {
	return fmt.Sprintf("~/.pilot/%s", p.cfg.Project.Name)
}

func (p *Provider) connect() (*pilotSSH.Client, error) {
	port := p.target.Port
	if port == 0 {
		port = 22
	}
	c, err := pilotSSH.NewClient(pilotSSH.Config{
		Host:    p.target.Host,
		User:    p.target.User,
		KeyPath: p.target.Key,
		Port:    port,
	})
	if err != nil {
		return nil, &piloterr.SSHError{Host: p.target.Host, Op: "connect", Cause: err}
	}
	return c, nil
}

// composeFileForEnv returns the conventional compose filename for an environment.
// Use remoteComposeFile when building SSH commands — files live in ~/pilot/ on the VPS.
func composeFileForEnv(env string) string {
	return fmt.Sprintf("docker-compose.%s.yml", env)
}

// nginxServicesWithUpdatedMounts returns the names of services that:
//   - use an nginx image (image name contains "nginx")
//   - have at least one bind-mounted file that appears in syncedFiles
//
// These services need "nginx -s reload" after pilot sync to pick up config changes.
func nginxServicesWithUpdatedMounts(composeFile string, syncedFiles map[string]bool) []string {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil
	}

	var compose struct {
		Services map[string]struct {
			Image   string `yaml:"image"`
			Volumes []any  `yaml:"volumes"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil
	}

	var result []string
	for name, svc := range compose.Services {
		if !strings.Contains(svc.Image, "nginx") {
			continue
		}
		for _, v := range svc.Volumes {
			var src string
			switch vol := v.(type) {
			case string:
				parts := strings.SplitN(vol, ":", 2)
				src = parts[0]
			case map[string]any:
				if t, _ := vol["type"].(string); t != "bind" {
					continue
				}
				src, _ = vol["source"].(string)
			}
			if syncedFiles[src] {
				result = append(result, name)
				break
			}
		}
	}
	return result
}

// remoteComposeFile returns the full remote path where pilot stores compose files.
// pilot sync always copies compose files to ~/pilot/, so all remote docker compose
// commands must use this path, not the bare filename.
func remoteComposeFile(env string) string {
	return fmt.Sprintf("~/pilot/docker-compose.%s.yml", env)
}

// isDockerPermissionError detects the classic "user not in docker group" error.
func isDockerPermissionError(output string) bool {
	lower := strings.ToLower(output)
	return (strings.Contains(lower, "permission denied") && strings.Contains(lower, "docker")) ||
		strings.Contains(lower, "got permission denied while trying to connect to the docker daemon socket")
}

// RunHooks executes a list of shell commands on the remote VPS via SSH.
// Commands run sequentially; the first failure stops execution.
func (p *Provider) RunHooks(ctx context.Context, commands []string) error {
	if len(commands) == 0 {
		return nil
	}
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	for _, cmd := range commands {
		var buf strings.Builder
		if err := client.RunWithOutput(ctx, cmd, remoteOutputWriter(&buf)); err != nil {
			return fmt.Errorf("hook %q failed: %w\n%s", cmd, err, buf.String())
		}
	}
	return nil
}

// RunMigrations runs the migration command on the remote VPS via SSH.
// The command runs from ~/pilot/ where the compose files and env files live.
func (p *Provider) RunMigrations(ctx context.Context, tool, command string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	// cd to ~/pilot/ so relative paths (e.g. prisma/schema.prisma) resolve correctly.
	fullCmd := fmt.Sprintf("cd ~/pilot && %s", command)
	var buf strings.Builder
	if err := client.RunWithOutput(ctx, fullCmd, remoteOutputWriter(&buf)); err != nil {
		return fmt.Errorf("migrations (%s): %w\n%s", tool, err, buf.String())
	}
	return nil
}

// RollbackMigrations runs the migration rollback command on the remote VPS via SSH.
func (p *Provider) RollbackMigrations(ctx context.Context, tool, rollbackCommand string) error {
	if rollbackCommand == "" {
		return fmt.Errorf("migrations (%s): no rollback_command configured — cannot compensate", tool)
	}
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	fullCmd := fmt.Sprintf("cd ~/pilot && %s", rollbackCommand)
	var buf strings.Builder
	if err := client.RunWithOutput(ctx, fullCmd, remoteOutputWriter(&buf)); err != nil {
		return fmt.Errorf("migration rollback (%s): %w\n%s", tool, err, buf.String())
	}
	return nil
}

// SetupDockerGroup adds the SSH user to the docker group on the remote VPS.
// Requires password-less sudo on the target (common on cloud VMs).
func (p *Provider) SetupDockerGroup(ctx context.Context) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	user := p.target.User
	if user == "" {
		user = "deploy"
	}

	steps := []struct {
		desc string
		cmd  string
	}{
		{"Checking docker installation", "docker --version"},
		{fmt.Sprintf("Adding %q to docker group", user), fmt.Sprintf("sudo usermod -aG docker %s", user)},
		{"Verifying (new session check)", fmt.Sprintf("id %s", user)},
	}

	for _, s := range steps {
		out, err := client.Run(ctx, s.cmd)
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", s.desc, err, out)
		}
	}
	return nil
}
