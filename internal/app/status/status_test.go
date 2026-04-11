package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/status"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockExecution struct {
	statuses []domain.ServiceStatus
	err      error
}

func (m *mockExecution) Up(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockExecution) Down(_ context.Context, _ string) error           { return nil }
func (m *mockExecution) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return m.statuses, m.err
}
func (m *mockExecution) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

type mockDeploy struct {
	statuses []domain.ServiceStatus
	err      error
}

func (m *mockDeploy) Sync(_ context.Context, _ string) error                        { return nil }
func (m *mockDeploy) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error { return nil }
func (m *mockDeploy) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (m *mockDeploy) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return m.statuses, m.err
}
func (m *mockDeploy) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func localConfig() *config.Config {
	return &config.Config{
		Environments: map[string]config.Environment{"dev": {}},
	}
}

func remoteConfig() *config.Config {
	return &config.Config{
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
		Targets: map[string]config.Target{
			"vps-prod": {Host: "1.2.3.4"},
		},
	}
}

var running = []domain.ServiceStatus{{Name: "api", State: "running", Health: "healthy"}}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestStatus_Local(t *testing.T) {
	uc := status.New(&mockExecution{statuses: running})

	out, err := uc.Execute(context.Background(), status.Input{
		Env:    "dev",
		Config: localConfig(),
	})

	require.NoError(t, err)
	assert.False(t, out.Remote)
	assert.Len(t, out.Statuses, 1)
	assert.Equal(t, "api", out.Statuses[0].Name)
}

func TestStatus_Remote(t *testing.T) {
	uc := status.NewRemote(&mockDeploy{statuses: running})

	out, err := uc.Execute(context.Background(), status.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.NoError(t, err)
	assert.True(t, out.Remote)
	assert.Equal(t, "vps-prod", out.Target)
	assert.Equal(t, "1.2.3.4", out.Host)
	assert.Len(t, out.Statuses, 1)
}

func TestStatus_LocalError(t *testing.T) {
	uc := status.New(&mockExecution{err: errors.New("compose not running")})

	_, err := uc.Execute(context.Background(), status.Input{
		Env:    "dev",
		Config: localConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose not running")
}

func TestStatus_RemoteError(t *testing.T) {
	uc := status.NewRemote(&mockDeploy{err: errors.New("SSH timeout")})

	_, err := uc.Execute(context.Background(), status.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH timeout")
}

func TestStatus_UnknownEnv(t *testing.T) {
	uc := status.New(&mockExecution{})

	_, err := uc.Execute(context.Background(), status.Input{
		Env:    "unknown",
		Config: localConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}
