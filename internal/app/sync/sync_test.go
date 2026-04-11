package sync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pilotSync "github.com/mouhamedsylla/pilot/internal/app/sync"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockDeploy struct {
	syncErr error
}

func (m *mockDeploy) Sync(_ context.Context, _ string) error { return m.syncErr }
func (m *mockDeploy) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error {
	return nil
}
func (m *mockDeploy) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (m *mockDeploy) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, nil
}
func (m *mockDeploy) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func remoteConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
		Targets: map[string]config.Target{
			"vps-prod": {Host: "1.2.3.4"},
		},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestSync_AllOK(t *testing.T) {
	uc := pilotSync.New(&mockDeploy{})

	out, err := uc.Execute(context.Background(), pilotSync.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.NoError(t, err)
	assert.Equal(t, "vps-prod", out.TargetName)
	assert.Equal(t, "1.2.3.4", out.TargetHost)
}

func TestSync_NoTarget(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"dev": {},
		},
	}
	uc := pilotSync.New(&mockDeploy{})

	_, err := uc.Execute(context.Background(), pilotSync.Input{
		Env:    "dev",
		Config: cfg,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no deploy target")
}

func TestSync_TargetOverride(t *testing.T) {
	cfg := &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
		Targets: map[string]config.Target{
			"vps-staging": {Host: "9.9.9.9"},
		},
	}
	uc := pilotSync.New(&mockDeploy{})

	out, err := uc.Execute(context.Background(), pilotSync.Input{
		Env:            "prod",
		TargetOverride: "vps-staging",
		Config:         cfg,
	})

	require.NoError(t, err)
	assert.Equal(t, "vps-staging", out.TargetName)
	assert.Equal(t, "9.9.9.9", out.TargetHost)
}

func TestSync_ProviderError(t *testing.T) {
	uc := pilotSync.New(&mockDeploy{syncErr: errors.New("SSH refused")})

	_, err := uc.Execute(context.Background(), pilotSync.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH refused")
}

func TestSync_UnknownEnv(t *testing.T) {
	uc := pilotSync.New(&mockDeploy{})

	_, err := uc.Execute(context.Background(), pilotSync.Input{
		Env:    "unknown",
		Config: remoteConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}
