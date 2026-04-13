package compose

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// Orchestrator implements domain.ExecutionProvider using docker compose.
type Orchestrator struct {
	cfg         *config.Config
	env         string
	composeFile string
	envFile     string
	projectName string
}

func New(cfg *config.Config, env string) *Orchestrator {
	envCfg := cfg.Environments[env]
	return &Orchestrator{
		cfg:         cfg,
		env:         env,
		composeFile: composeFileForEnv(env),
		envFile:     envCfg.EnvFile,
		projectName: fmt.Sprintf("%s-%s", cfg.Project.Name, env),
	}
}

// composeFileForEnv returns the conventional compose filename for an environment.
func composeFileForEnv(env string) string {
	return fmt.Sprintf("docker-compose.%s.yml", env)
}

func (o *Orchestrator) Up(ctx context.Context, env string, services []string) error {
	args := o.baseArgs()
	args = append(args, "up", "--detach", "--remove-orphans", "--build")
	args = append(args, services...)
	return o.run(ctx, args...)
}

func (o *Orchestrator) Down(ctx context.Context, env string) error {
	args := o.baseArgs()
	args = append(args, "down", "--remove-orphans")
	return o.run(ctx, args...)
}

// Logs streams log output for a specific service.
// The env parameter is unused — compose uses the env set at construction time.
func (o *Orchestrator) Logs(ctx context.Context, _ string, service string, opts domain.LogOptions) (<-chan string, error) {
	args := o.baseArgs()
	args = append(args, "logs")
	if opts.Follow {
		args = append(args, "--follow")
	}
	if opts.Since != "" {
		args = append(args, "--since", opts.Since)
	}
	if opts.Lines > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Lines))
	}
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("logs pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("logs start: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		buf := make([]byte, 4096)
		for {
			n, err := out.Read(buf)
			if n > 0 {
				for _, line := range strings.Split(string(buf[:n]), "\n") {
					if line != "" {
						ch <- line
					}
				}
			}
			if err != nil {
				break
			}
		}
		_ = cmd.Wait()
	}()

	return ch, nil
}

// Status returns the current state of all services.
// The env parameter is unused — compose uses the env set at construction time.
func (o *Orchestrator) Status(ctx context.Context, _ string) ([]domain.ServiceStatus, error) {
	args := o.baseArgs()
	args = append(args, "ps", "--format", "json")
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("compose ps: %w", err)
	}
	return parseComposePS(out)
}

// baseArgs returns the common docker compose flags.
func (o *Orchestrator) baseArgs() []string {
	args := []string{"compose", "-p", o.projectName, "-f", o.composeFile}
	if o.envFile != "" {
		args = append(args, "--env-file", o.envFile)
	}
	return args
}

func (o *Orchestrator) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)

	if os.Getenv("PILOT_MCP") != "1" {
		// CLI mode: stream docker output directly so the user sees it in real time.
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%w", err)
		}
		return nil
	}

	// MCP mode: capture output — never write to os.Stdout (JSON-RPC pipe).
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(buf.String())
		if output != "" {
			return fmt.Errorf("%w\n\n%s", err, output)
		}
		return fmt.Errorf("%w", err)
	}
	return nil
}
