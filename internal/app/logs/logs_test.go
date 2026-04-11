package logs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pilotLogs "github.com/mouhamedsylla/pilot/internal/app/logs"
	"github.com/mouhamedsylla/pilot/internal/config"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockExecution struct {
	ch  chan string
	err error
}

func (m *mockExecution) Up(_ context.Context, _ string, _ []string) error { return nil }
func (m *mockExecution) Down(_ context.Context, _ string) error           { return nil }
func (m *mockExecution) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, nil
}
func (m *mockExecution) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ch, nil
}

type mockDeploy struct {
	ch  chan string
	err error
}

func (m *mockDeploy) Sync(_ context.Context, _ string) error                        { return nil }
func (m *mockDeploy) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error { return nil }
func (m *mockDeploy) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}
func (m *mockDeploy) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
	return nil, nil
}
func (m *mockDeploy) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ch, nil
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
	}
}

func closedChan(lines ...string) chan string {
	ch := make(chan string, len(lines))
	for _, l := range lines {
		ch <- l
	}
	close(ch)
	return ch
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestLogs_Local(t *testing.T) {
	ch := closedChan("line1", "line2")
	uc := pilotLogs.New(&mockExecution{ch: ch})

	out, err := uc.Execute(context.Background(), pilotLogs.Input{
		Env:    "dev",
		Config: localConfig(),
	})

	require.NoError(t, err)
	var received []string
	for l := range out.Lines {
		received = append(received, l)
	}
	assert.Equal(t, []string{"line1", "line2"}, received)
}

func TestLogs_Remote(t *testing.T) {
	ch := closedChan("remote-line")
	uc := pilotLogs.NewRemote(&mockDeploy{ch: ch})

	out, err := uc.Execute(context.Background(), pilotLogs.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.NoError(t, err)
	var received []string
	for l := range out.Lines {
		received = append(received, l)
	}
	assert.Equal(t, []string{"remote-line"}, received)
}

func TestLogs_LocalError(t *testing.T) {
	uc := pilotLogs.New(&mockExecution{err: errors.New("compose not running")})

	_, err := uc.Execute(context.Background(), pilotLogs.Input{
		Env:    "dev",
		Config: localConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose not running")
}

func TestLogs_RemoteError(t *testing.T) {
	uc := pilotLogs.NewRemote(&mockDeploy{err: errors.New("SSH refused")})

	_, err := uc.Execute(context.Background(), pilotLogs.Input{
		Env:    "prod",
		Config: remoteConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH refused")
}

func TestLogs_UnknownEnv(t *testing.T) {
	uc := pilotLogs.New(&mockExecution{ch: make(chan string)})

	_, err := uc.Execute(context.Background(), pilotLogs.Input{
		Env:    "unknown",
		Config: localConfig(),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}
