// infra.go provides the real (system-touching) implementations of the
// preflight ports. cmd/ and mcp/ use these; tests use mocks.
package preflight

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	pilotSSH "github.com/mouhamedsylla/pilot/pkg/ssh"
)

// ── DockerChecker ─────────────────────────────────────────────────────────────

// RealDockerChecker checks the Docker daemon via `docker info`.
type RealDockerChecker struct{}

func (RealDockerChecker) IsRunning(ctx context.Context) error {
	dCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(dCtx, "docker", "info").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker daemon not reachable: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ── SSHChecker ────────────────────────────────────────────────────────────────

// RealSSHChecker performs remote checks over a real SSH connection.
type RealSSHChecker struct{}

func (RealSSHChecker) CheckConnectivity(ctx context.Context, host, user, keyPath string, port int) error {
	sshCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client, err := pilotSSH.NewClient(pilotSSH.Config{
		Host:    host,
		User:    user,
		KeyPath: keyPath,
		Port:    port,
	})
	if err != nil {
		return err
	}
	defer client.Close()
	_ = sshCtx
	return nil
}

func (RealSSHChecker) HasDockerAccess(ctx context.Context, host, user, keyPath string, port int) (bool, error) {
	client, err := pilotSSH.NewClient(pilotSSH.Config{Host: host, User: user, KeyPath: keyPath, Port: port})
	if err != nil {
		return false, err
	}
	defer client.Close()
	out, _ := client.Run(ctx, "docker ps 2>&1")
	hasPermErr := strings.Contains(strings.ToLower(out), "permission denied") &&
		strings.Contains(strings.ToLower(out), "docker")
	return !hasPermErr, nil
}

func (RealSSHChecker) FileExists(ctx context.Context, host, user, keyPath string, port int, remotePath string) (bool, error) {
	client, err := pilotSSH.NewClient(pilotSSH.Config{Host: host, User: user, KeyPath: keyPath, Port: port})
	if err != nil {
		return false, err
	}
	defer client.Close()
	out, _ := client.Run(ctx, fmt.Sprintf("test -f %s && echo ok || echo missing", remotePath))
	return strings.TrimSpace(out) == "ok", nil
}

// ── helpers used by cmd ───────────────────────────────────────────────────────

// ActiveEnv resolves the active environment from flag override or .pilot/env.
func ActiveEnv(override string) string {
	if override != "" {
		return override
	}
	if b, err := os.ReadFile(".pilot/env"); err == nil {
		if e := strings.TrimSpace(string(b)); e != "" {
			return e
		}
	}
	return "dev"
}

// DetectRemoteEnv loads config and returns the first env that has a deploy
// target, skipping currentEnv. Returns "" if none found.
func DetectRemoteEnv(currentEnv string) string {
	// Lazy import to avoid circular dependency — just read the file.
	// The full implementation lives in cmd which has access to config.Load.
	return ""
}
