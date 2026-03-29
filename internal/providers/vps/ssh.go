package vps

import (
	"context"
	"fmt"

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
	envCfg := p.cfg.Environments[env]
	composeFile := envCfg.ComposeFile
	if composeFile == "" {
		composeFile = fmt.Sprintf("docker-compose.%s.yml", env)
	}

	commands := []string{
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("docker compose -f %s up -d --remove-orphans", composeFile),
	}

	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			return fmt.Errorf("remote command %q failed: %w\n%s", cmd, err, out)
		}
	}
	return nil
}

func (p *Provider) Sync(ctx context.Context, target string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	files := []string{"kaal.yaml"}
	for _, env := range p.cfg.Environments {
		if env.ComposeFile != "" {
			files = append(files, env.ComposeFile)
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

	envCfg := p.cfg.Environments[env]
	composeFile := envCfg.ComposeFile
	if composeFile == "" {
		composeFile = fmt.Sprintf("docker-compose.%s.yml", env)
	}

	out, err := client.Run(ctx, fmt.Sprintf("docker compose -f %s ps --format json", composeFile))
	if err != nil {
		return nil, fmt.Errorf("remote status: %w", err)
	}
	return parseRemotePS(out), nil
}

func (p *Provider) Rollback(ctx context.Context, env string, version string) error {
	client, err := p.connect()
	if err != nil {
		return err
	}
	defer client.Close()

	image := fmt.Sprintf("%s:%s", p.cfg.Registry.Image, version)
	envCfg := p.cfg.Environments[env]
	composeFile := envCfg.ComposeFile
	if composeFile == "" {
		composeFile = fmt.Sprintf("docker-compose.%s.yml", env)
	}

	commands := []string{
		fmt.Sprintf("docker pull %s", image),
		fmt.Sprintf("IMAGE_TAG=%s docker compose -f %s up -d --remove-orphans", version, composeFile),
	}

	for _, cmd := range commands {
		if out, err := client.Run(ctx, cmd); err != nil {
			return fmt.Errorf("rollback command %q failed: %w\n%s", cmd, err, out)
		}
	}
	return nil
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
