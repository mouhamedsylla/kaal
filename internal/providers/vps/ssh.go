package vps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/kaalerr"
	"github.com/mouhamedsylla/kaal/internal/providers"
	kaalSSH "github.com/mouhamedsylla/kaal/pkg/ssh"
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

func (p *Provider) Deploy(ctx context.Context, env string, opts providers.DeployOptions) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	image := fmt.Sprintf("%s:%s", p.cfg.Registry.Image, opts.Tag)
	composeFile := remoteComposeFile(env) // always ~/kaal/docker-compose.<env>.yml
	stateDir := p.stateDir()

	// Copy resolved env files to remote before running compose.
	if len(opts.EnvFiles) > 0 {
		if err := client.CopyFiles(ctx, opts.EnvFiles, "~/kaal/"); err != nil {
			p.recordDeploy(ctx, client, env, opts.Tag, false, err.Error())
			return fmt.Errorf("sync env files: %w", err)
		}
	}

	// Build --env-file flags for docker compose.
	envFileFlags := ""
	for _, f := range opts.EnvFiles {
		// Remote path: ~/kaal/<basename>
		base := f
		if idx := strings.LastIndex(f, "/"); idx >= 0 {
			base = f[idx+1:]
		}
		envFileFlags += fmt.Sprintf(" --env-file ~/kaal/%s", base)
	}

	commands := []string{
		// Ensure state dir exists
		fmt.Sprintf("mkdir -p %s", stateDir),
		// Rotate tags: current → prev, new → current
		fmt.Sprintf("[ -f %s/current-tag ] && cp %s/current-tag %s/prev-tag || true", stateDir, stateDir, stateDir),
		fmt.Sprintf("echo %s > %s/current-tag", opts.Tag, stateDir),
		// Pull and restart
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s%s up -d --remove-orphans", opts.Tag, composeFile, envFileFlags),
	}
	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			cause := err
			if isDockerPermissionError(out) {
				cause = fmt.Errorf(
					"user %q is not in the docker group\n\n"+
						"  Fix it automatically:\n"+
						"    kaal setup --env %s\n\n"+
						"  Or manually on your VPS:\n"+
						"    sudo usermod -aG docker %s\n"+
						"  Then run kaal deploy again (new SSH session picks up the group change)",
					p.target.User, env, p.target.User,
				)
			}
			deployErr := &kaalerr.DeployError{
				Phase:   "restart",
				Target:  p.target.Host,
				Command: cmd,
				Output:  out,
				Cause:   cause,
			}
			p.recordDeploy(ctx, client, env, opts.Tag, false, err.Error())
			return deployErr
		}
	}
	p.recordDeploy(ctx, client, env, opts.Tag, true, "")
	return nil
}

func (p *Provider) Sync(ctx context.Context, _ string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	// ── Flat files: kaal.yaml + compose files + env_file ──────────────────
	// These all land directly in ~/kaal/ (no subdirectory).
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

	addFlat("kaal.yaml")
	for envName, envCfg := range p.cfg.Environments {
		composeFile := composeFileForEnv(envName)
		addFlat(composeFile)
		addFlat(envCfg.EnvFile)
	}

	if err := client.CopyFiles(ctx, flatFiles, "~/kaal/"); err != nil {
		return err
	}

	// ── Bind-mount config files: preserve relative path under ~/kaal/ ─────
	// Scan every compose file for local bind-mounts (./nginx/prod.conf, etc.).
	// If the source is a local file, copy it to ~/kaal/<relative-path> so
	// docker compose running from ~/kaal/ finds it exactly where it expects.
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
			// Remote path mirrors the local relative path: ~/kaal/nginx/prod.conf
			remotePath := fmt.Sprintf("~/kaal/%s", strings.TrimPrefix(localSrc, "./"))
			if copyErr := client.CopyFileTo(ctx, localSrc, remotePath); copyErr != nil {
				return fmt.Errorf("sync bind-mount %s: %w", localSrc, copyErr)
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

func (p *Provider) Status(ctx context.Context, env string) ([]providers.ServiceStatus, error) {
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

func (p *Provider) Logs(ctx context.Context, env string, opts providers.LogOptions) (<-chan string, error) {
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
	if opts.Service != "" {
		args += " " + opts.Service
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

	// Resolve version: explicit tag or read prev-tag from VPS state
	tag := version
	if tag == "" {
		out, err := client.Run(ctx, fmt.Sprintf("cat %s/prev-tag 2>/dev/null", p.stateDir()))
		if err != nil || strings.TrimSpace(out) == "" {
			return "", fmt.Errorf(
				"no previous deployment found on %s\n  Use --version <tag> to specify a version explicitly",
				p.target.Host,
			)
		}
		tag = strings.TrimSpace(out)
	}

	image := fmt.Sprintf("%s:%s", p.cfg.Registry.Image, tag)
	composeFile := remoteComposeFile(env) // always ~/kaal/docker-compose.<env>.yml
	stateDir := p.stateDir()

	commands := []string{
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", tag, composeFile),
		fmt.Sprintf("echo %s > %s/current-tag", tag, stateDir),
	}
	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			deployErr := &kaalerr.DeployError{
				Phase:   "rollback",
				Target:  p.target.Host,
				Command: cmd,
				Output:  out,
				Cause:   err,
			}
			p.recordDeploy(ctx, client, env, tag, false, "rollback: "+err.Error())
			return "", deployErr
		}
	}
	p.recordDeploy(ctx, client, env, tag, true, "rollback")
	return tag, nil
}

// stateDir returns the remote path where kaal stores deploy state for this project.
func (p *Provider) stateDir() string {
	return fmt.Sprintf("~/.kaal/%s", p.cfg.Project.Name)
}

func (p *Provider) connect() (*kaalSSH.Client, error) {
	port := p.target.Port
	if port == 0 {
		port = 22
	}
	c, err := kaalSSH.NewClient(kaalSSH.Config{
		Host:    p.target.Host,
		User:    p.target.User,
		KeyPath: p.target.Key,
		Port:    port,
	})
	if err != nil {
		return nil, &kaalerr.SSHError{Host: p.target.Host, Op: "connect", Cause: err}
	}
	return c, nil
}

// composeFileForEnv returns the conventional compose filename for an environment.
// Use remoteComposeFile when building SSH commands — files live in ~/kaal/ on the VPS.
func composeFileForEnv(env string) string {
	return fmt.Sprintf("docker-compose.%s.yml", env)
}

// remoteComposeFile returns the full remote path where kaal stores compose files.
// kaal sync always copies compose files to ~/kaal/, so all remote docker compose
// commands must use this path, not the bare filename.
func remoteComposeFile(env string) string {
	return fmt.Sprintf("~/kaal/docker-compose.%s.yml", env)
}

// isDockerPermissionError detects the classic "user not in docker group" error.
func isDockerPermissionError(output string) bool {
	lower := strings.ToLower(output)
	return (strings.Contains(lower, "permission denied") && strings.Contains(lower, "docker")) ||
		strings.Contains(lower, "got permission denied while trying to connect to the docker daemon socket")
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
