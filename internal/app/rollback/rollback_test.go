package rollback_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/rollback"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockDeploy struct {
	rollbackTag string
	rollbackErr error
}

func (m *mockDeploy) Sync(_ context.Context, _ string) error { return nil }
func (m *mockDeploy) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error {
	return nil
}
func (m *mockDeploy) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return m.rollbackTag, m.rollbackErr
}
func (m *mockDeploy) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, nil
}
func (m *mockDeploy) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func deployConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
		Targets: map[string]config.Target{
			"vps-prod": {Host: "1.2.3.4", User: "deploy", Port: 22},
		},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestRollback_AllOK(t *testing.T) {
	uc := rollback.New(&mockDeploy{rollbackTag: "v1.0.0"})

	out, err := uc.Execute(context.Background(), rollback.Input{
		Env:    "prod",
		Config: deployConfig(),
	})

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", out.RestoredTag)
	assert.Equal(t, "vps-prod", out.TargetName)
	assert.Equal(t, "1.2.3.4", out.TargetHost)
}

func TestRollback_SpecificVersion(t *testing.T) {
	mock := &mockDeploy{rollbackTag: "v0.9.0"}
	uc := rollback.New(mock)

	out, err := uc.Execute(context.Background(), rollback.Input{
		Env:     "prod",
		Version: "v0.9.0",
		Config:  deployConfig(),
	})

	require.NoError(t, err)
	assert.Equal(t, "v0.9.0", out.RestoredTag)
}

func TestRollback_NoTarget(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"dev": {},
		},
	}
	uc := rollback.New(&mockDeploy{})

	_, err := uc.Execute(context.Background(), rollback.Input{
		Env:    "dev",
		Config: cfg,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deploy target")
}

func TestRollback_TargetOverride(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
		Targets: map[string]config.Target{
			"vps-staging": {Host: "5.6.7.8"},
		},
	}
	uc := rollback.New(&mockDeploy{rollbackTag: "v1.1.0"})

	out, err := uc.Execute(context.Background(), rollback.Input{
		Env:        "prod",
		TargetName: "vps-staging",
		Config:     cfg,
	})

	require.NoError(t, err)
	assert.Equal(t, "vps-staging", out.TargetName)
	assert.Equal(t, "5.6.7.8", out.TargetHost)
}

func TestRollback_ProviderError(t *testing.T) {
	uc := rollback.New(&mockDeploy{rollbackErr: errors.New("SSH timeout")})

	_, err := uc.Execute(context.Background(), rollback.Input{
		Env:    "prod",
		Config: deployConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH timeout")
}

func TestRollback_UnknownEnv(t *testing.T) {
	uc := rollback.New(&mockDeploy{})

	_, err := uc.Execute(context.Background(), rollback.Input{
		Env:    "unknown",
		Config: deployConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}
