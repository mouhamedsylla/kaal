package vps

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/providers"
	kaalSSH "github.com/mouhamedsylla/kaal/pkg/ssh"
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
	composeFile := composeFileForEnv(env)
	stateDir := p.stateDir()

	commands := []string{
		// Ensure state dir exists
		fmt.Sprintf("mkdir -p %s", stateDir),
		// Rotate tags: current → prev, new → current
		fmt.Sprintf("[ -f %s/current-tag ] && cp %s/current-tag %s/prev-tag || true", stateDir, stateDir, stateDir),
		fmt.Sprintf("echo %s > %s/current-tag", opts.Tag, stateDir),
		// Pull and restart
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", opts.Tag, composeFile),
	}
	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			return fmt.Errorf("remote command %q failed: %w\n%s", cmd, err, out)
		}
	}
	return nil
}

func (p *Provider) Sync(ctx context.Context, _ string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	// Always sync kaal.yaml + any compose files that exist locally
	files := []string{"kaal.yaml"}
	for envName := range p.cfg.Environments {
		f := composeFileForEnv(envName)
		if _, err := os.Stat(f); err == nil {
			files = append(files, f)
		}
	}
	return client.CopyFiles(ctx, files, "~/kaal/")
}

func (p *Provider) Status(ctx context.Context, env string) ([]providers.ServiceStatus, error) {
	client, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	out, err := client.Run(ctx, fmt.Sprintf("docker compose -f %s ps --format json", composeFileForEnv(env)))
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

	composeFile := composeFileForEnv(env)
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
	composeFile := composeFileForEnv(env)
	stateDir := p.stateDir()

	commands := []string{
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", tag, composeFile),
		fmt.Sprintf("echo %s > %s/current-tag", tag, stateDir),
	}
	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			return "", fmt.Errorf("rollback command %q failed: %w\n%s", cmd, err, out)
		}
	}
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
	return kaalSSH.NewClient(kaalSSH.Config{
		Host:    p.target.Host,
		User:    p.target.User,
		KeyPath: p.target.Key,
		Port:    port,
	})
}

// composeFileForEnv returns the conventional compose filename for an environment.
func composeFileForEnv(env string) string {
	return fmt.Sprintf("docker-compose.%s.yml", env)
}
