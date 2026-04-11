package up_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/up"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockExecution struct {
	upErr   error
	downErr error
}

func (m *mockExecution) Up(_ context.Context, _ string, _ []string) error  { return m.upErr }
func (m *mockExecution) Down(_ context.Context, _ string) error            { return m.downErr }
func (m *mockExecution) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, nil
}
func (m *mockExecution) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func baseConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"dev": {EnvFile: ".env.dev"},
		},
	}
}

func withComposeFile(t *testing.T, env string) (dir string) {
	t.Helper()
	dir = t.TempDir()
	path := filepath.Join(dir, "docker-compose."+env+".yml")
	require.NoError(t, os.WriteFile(path, []byte("services: {}"), 0644))
	return dir
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestUp_AllOK(t *testing.T) {
	dir := withComposeFile(t, "dev")
	uc := up.New(&mockExecution{})

	out, err := uc.Execute(context.Background(), up.Input{
		Env:        "dev",
		Config:     baseConfig(),
		ProjectDir: dir,
	})

	require.NoError(t, err)
	assert.Equal(t, "dev", out.Env)
	assert.False(t, out.IsRemoteEnv)
}

func TestUp_MissingComposeFile(t *testing.T) {
	uc := up.New(&mockExecution{})

	_, err := uc.Execute(context.Background(), up.Input{
		Env:        "dev",
		Config:     baseConfig(),
		ProjectDir: t.TempDir(), // no compose file
	})

	require.Error(t, err)
	var mce *up.MissingComposeError
	assert.True(t, errors.As(err, &mce))
	assert.Equal(t, "dev", mce.Env)
}

func TestUp_UnknownEnv(t *testing.T) {
	uc := up.New(&mockExecution{})

	_, err := uc.Execute(context.Background(), up.Input{
		Env:        "staging",
		Config:     baseConfig(),
		ProjectDir: t.TempDir(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging")
}

func TestUp_ProviderError(t *testing.T) {
	dir := withComposeFile(t, "dev")
	uc := up.New(&mockExecution{upErr: errors.New("port already in use")})

	_, err := uc.Execute(context.Background(), up.Input{
		Env:        "dev",
		Config:     baseConfig(),
		ProjectDir: dir,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "port already in use")
}

func TestUp_RemoteEnvWarning(t *testing.T) {
	dir := withComposeFile(t, "prod")
	cfg := &config.Config{
		Project: config.Project{Name: "my-app"},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
	}
	uc := up.New(&mockExecution{})

	out, err := uc.Execute(context.Background(), up.Input{
		Env:        "prod",
		Config:     cfg,
		ProjectDir: dir,
	})

	require.NoError(t, err)
	assert.True(t, out.IsRemoteEnv)
	assert.Equal(t, "vps-prod", out.TargetName)
}

func TestDown_AllOK(t *testing.T) {
	uc := up.NewDown(&mockExecution{})

	out, err := uc.Execute(context.Background(), up.DownInput{
		Env:    "dev",
		Config: baseConfig(),
	})

	require.NoError(t, err)
	assert.Equal(t, "dev", out.Env)
}

func TestDown_UnknownEnv(t *testing.T) {
	uc := up.NewDown(&mockExecution{})

	_, err := uc.Execute(context.Background(), up.DownInput{
		Env:    "unknown",
		Config: baseConfig(),
	})

	require.Error(t, err)
}
