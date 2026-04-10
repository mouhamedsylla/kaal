package preflight_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/preflight"
	"github.com/mouhamedsylla/pilot/internal/config"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockDockerChecker struct{ err error }

func (m *mockDockerChecker) IsRunning(ctx context.Context) error { return m.err }

type mockSSHChecker struct {
	connectErr   error
	dockerAccess bool // true = user is in docker group
	envFileFound bool
}

func (m *mockSSHChecker) CheckConnectivity(ctx context.Context, host, user, keyPath string, port int) error {
	return m.connectErr
}
func (m *mockSSHChecker) HasDockerAccess(ctx context.Context, host, user, keyPath string, port int) (bool, error) {
	if m.connectErr != nil {
		return false, m.connectErr
	}
	return m.dockerAccess, nil
}
func (m *mockSSHChecker) FileExists(ctx context.Context, host, user, keyPath string, port int, remotePath string) (bool, error) {
	if m.connectErr != nil {
		return false, m.connectErr
	}
	return m.envFileFound, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func minimalConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app", Stack: "go"},
		Registry: config.RegistryConfig{
			Provider: "ghcr",
			Image:    "ghcr.io/user/my-app",
		},
		Environments: map[string]config.Environment{
			"dev": {EnvFile: ".env.dev"},
		},
	}
}

func deployConfig() *config.Config {
	cfg := minimalConfig()
	cfg.Environments["prod"] = config.Environment{
		Target: "vps-prod",
	}
	cfg.Targets = map[string]config.Target{
		"vps-prod": {Host: "1.2.3.4", User: "deploy", Key: "~/.ssh/id_pilot", Port: 22},
	}
	return cfg
}

// ── tests ─────────────────────────────────────────────────────────────────────

// Toutes les vérifications passent → AllOK true, aucun blocker.
func TestPreflight_AllOK(t *testing.T) {
	dir := t.TempDir()
	// docker-compose.dev.yml requis pour TargetUp
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "docker-compose.dev.yml"), []byte("services: {}"), 0644,
	))

	uc := preflight.New(
		&mockDockerChecker{err: nil},
		&mockSSHChecker{dockerAccess: true, envFileFound: true},
	)

	out, err := uc.Execute(context.Background(), preflight.Input{
		Target:     preflight.TargetUp,
		Env:        "dev",
		Config:     minimalConfig(),
		ProjectDir: dir,
	})

	require.NoError(t, err)
	assert.True(t, out.Report.AllOK)
	assert.Zero(t, out.Report.BlockerCount)
}

// Image placeholder → bloquant, AllOK false.
func TestPreflight_PlaceholderImage_IsBlocker(t *testing.T) {
	cfg := minimalConfig()
	cfg.Registry.Image = "ghcr.io/YOUR_USER/my-app"

	uc := preflight.New(&mockDockerChecker{}, &mockSSHChecker{})

	out, err := uc.Execute(context.Background(), preflight.Input{
		Target: preflight.TargetPush,
		Env:    "dev",
		Config: cfg,
	})

	require.NoError(t, err)
	assert.False(t, out.Report.AllOK)
	assert.Positive(t, out.Report.BlockerCount)
	assert.Equal(t, preflight.StatusError, findCheck(out.Report, "registry_image").Status)
}

// Docker daemon non démarré → bloquant.
func TestPreflight_DockerNotRunning_IsBlocker(t *testing.T) {
	uc := preflight.New(
		&mockDockerChecker{err: errors.New("docker daemon not reachable")},
		&mockSSHChecker{},
	)

	out, err := uc.Execute(context.Background(), preflight.Input{
		Target: preflight.TargetUp,
		Env:    "dev",
		Config: minimalConfig(),
	})

	require.NoError(t, err)
	assert.False(t, out.Report.AllOK)
	assert.Equal(t, preflight.StatusError, findCheck(out.Report, "docker_daemon").Status)
}

// SSH injoignable → bloquant pour target deploy.
func TestPreflight_SSHUnreachable_IsBlocker(t *testing.T) {
	uc := preflight.New(
		&mockDockerChecker{},
		&mockSSHChecker{connectErr: errors.New("connection refused")},
	)

	out, err := uc.Execute(context.Background(), preflight.Input{
		Target: preflight.TargetDeploy,
		Env:    "prod",
		Config: deployConfig(),
	})

	require.NoError(t, err)
	assert.False(t, out.Report.AllOK)
	assert.Equal(t, preflight.StatusError, findCheck(out.Report, "vps_connectivity").Status)
}

// Les vérifications SSH sont skippées pour target=up (pas de remote).
func TestPreflight_TargetUp_SkipsRemoteChecks(t *testing.T) {
	// SSH checker retourne une erreur — mais ça ne doit pas compter pour target=up
	uc := preflight.New(
		&mockDockerChecker{},
		&mockSSHChecker{connectErr: errors.New("unreachable")},
	)

	out, err := uc.Execute(context.Background(), preflight.Input{
		Target: preflight.TargetUp,
		Env:    "dev",
		Config: minimalConfig(),
	})

	require.NoError(t, err)
	// Aucun check SSH ne doit apparaître
	assert.Nil(t, findCheck(out.Report, "vps_connectivity"))
}

// ── helper ────────────────────────────────────────────────────────────────────

func findCheck(r *preflight.Report, name string) *preflight.Check {
	for i := range r.Checks {
		if r.Checks[i].Name == name {
			return &r.Checks[i]
		}
	}
	return nil
}
