package push_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/push"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockRegistry struct {
	loginErr error
	buildErr error
	pushErr  error
	builtTag string
}

func (m *mockRegistry) Login(_ context.Context) error { return m.loginErr }
func (m *mockRegistry) Build(_ context.Context, opts domain.BuildOptions) error {
	m.builtTag = opts.Tag
	return m.buildErr
}
func (m *mockRegistry) Push(_ context.Context, _ string) error { return m.pushErr }

// ── helpers ───────────────────────────────────────────────────────────────────

func minimalConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app", Stack: "go"},
		Registry: config.RegistryConfig{
			Provider: "ghcr",
			Image:    "ghcr.io/user/my-app",
		},
		Environments: map[string]config.Environment{
			"dev": {},
		},
	}
}

func withDockerfile(t *testing.T) (restore func()) {
	t.Helper()
	require.NoError(t, os.WriteFile("Dockerfile", []byte("FROM scratch\n"), 0644))
	return func() { os.Remove("Dockerfile") }
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestPush_AllOK(t *testing.T) {
	restore := withDockerfile(t)
	defer restore()

	reg := &mockRegistry{}
	uc := push.New(reg)

	out, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1.0.0",
		Config: minimalConfig(),
	})

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", out.Tag)
	assert.Equal(t, "ghcr.io/user/my-app:v1.0.0", out.Image)
}

func TestPush_PlaceholderImage(t *testing.T) {
	cfg := minimalConfig()
	cfg.Registry.Image = "ghcr.io/YOUR_GITHUB_USER/my-app"

	uc := push.New(&mockRegistry{})
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: cfg,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "placeholder")
}

func TestPush_EmptyImage(t *testing.T) {
	cfg := minimalConfig()
	cfg.Registry.Image = ""

	uc := push.New(&mockRegistry{})
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: cfg,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "registry.image")
}

func TestPush_LoginFail(t *testing.T) {
	restore := withDockerfile(t)
	defer restore()

	uc := push.New(&mockRegistry{loginErr: errors.New("unauthorized")})
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: minimalConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestPush_BuildFail(t *testing.T) {
	restore := withDockerfile(t)
	defer restore()

	uc := push.New(&mockRegistry{buildErr: errors.New("build failed")})
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: minimalConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build failed")
}

func TestPush_MissingDockerfile(t *testing.T) {
	// Ensure Dockerfile does not exist.
	os.Remove("Dockerfile")

	uc := push.New(&mockRegistry{})
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: minimalConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Dockerfile not found")
}

func TestPush_CustomDockerfile(t *testing.T) {
	// Create a custom Dockerfile in a temp dir.
	dir := t.TempDir()
	customPath := filepath.Join(dir, "Dockerfile.custom")
	require.NoError(t, os.WriteFile(customPath, []byte("FROM scratch\n"), 0644))

	cfg := minimalConfig()
	cfg.Services = map[string]config.Service{
		"api": {Type: config.ServiceTypeApp, Dockerfile: customPath},
	}

	reg := &mockRegistry{}
	uc := push.New(reg)
	_, err := uc.Execute(context.Background(), push.Input{
		Env:    "dev",
		Tag:    "v1",
		Config: cfg,
	})

	require.NoError(t, err)
	assert.Contains(t, reg.builtTag, "v1")
}
