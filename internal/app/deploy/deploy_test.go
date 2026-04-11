package deploy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/deploy"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// ── mocks ────────────────────────────────────────────────────────────────────

type mockDeployProvider struct {
	syncErr     error
	deployErr   error
	restoredTag string
	rollbackErr error
	statuses    []domain.ServiceStatus
	statusErr   error

	syncCalled     bool
	deployCalled   bool
	rollbackCalled bool
}

func (m *mockDeployProvider) Sync(ctx context.Context, env string) error {
	m.syncCalled = true
	return m.syncErr
}
func (m *mockDeployProvider) Deploy(ctx context.Context, env string, opts domain.DeployOptions) error {
	m.deployCalled = true
	return m.deployErr
}
func (m *mockDeployProvider) Rollback(ctx context.Context, env string, tag string) (string, error) {
	m.rollbackCalled = true
	return m.restoredTag, m.rollbackErr
}
func (m *mockDeployProvider) Status(ctx context.Context, env string) ([]domain.ServiceStatus, error) {
	return m.statuses, m.statusErr
}

func (m *mockDeployProvider) Logs(_ context.Context, _, _ string, _ domain.LogOptions) (<-chan string, error) {
	return nil, nil
}

type mockSecretManager struct {
	resolved map[string]string
	err      error
	called   bool
}

func (m *mockSecretManager) Inject(ctx context.Context, env string, refs map[string]string) (map[string]string, error) {
	m.called = true
	return m.resolved, m.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func healthyStatuses() []domain.ServiceStatus {
	return []domain.ServiceStatus{
		{Name: "api", State: "running", Health: "healthy"},
		{Name: "db", State: "running", Health: "healthy"},
	}
}

func unhealthyStatuses() []domain.ServiceStatus {
	return []domain.ServiceStatus{
		{Name: "api", State: "running", Health: "unhealthy"},
		{Name: "db", State: "running", Health: "healthy"},
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

// Happy path : sync → deploy → healthcheck OK → succeeded.
func TestDeploy_HappyPath(t *testing.T) {
	provider := &mockDeployProvider{statuses: healthyStatuses()}
	uc := deploy.New(provider, nil, t.TempDir())

	out, err := uc.Execute(context.Background(), deploy.Input{
		Env: "prod",
		Tag: "abc123",
	})

	require.NoError(t, err)
	assert.True(t, provider.syncCalled)
	assert.True(t, provider.deployCalled)
	assert.False(t, provider.rollbackCalled)
	assert.Equal(t, "abc123", out.Tag)
	assert.Len(t, out.Statuses, 2)
}

// Healthcheck échoué → rollback automatique → erreur retournée.
func TestDeploy_HealthcheckFailure_TriggersRollback(t *testing.T) {
	provider := &mockDeployProvider{
		statuses:    unhealthyStatuses(),
		restoredTag: "abc000",
	}
	uc := deploy.New(provider, nil, t.TempDir())

	_, err := uc.Execute(context.Background(), deploy.Input{Env: "prod", Tag: "abc123"})

	require.Error(t, err)
	assert.True(t, provider.rollbackCalled)
	assert.Contains(t, err.Error(), "abc000") // le tag restauré est mentionné
}

// Sync échoué → rien d'autre n'est exécuté.
func TestDeploy_SyncFailure_StopsExecution(t *testing.T) {
	provider := &mockDeployProvider{
		syncErr: errors.New("connection refused"),
	}
	uc := deploy.New(provider, nil, t.TempDir())

	_, err := uc.Execute(context.Background(), deploy.Input{Env: "prod", Tag: "abc123"})

	require.Error(t, err)
	assert.False(t, provider.deployCalled)
	assert.False(t, provider.rollbackCalled)
}

// Deploy échoué → rollback déclenché.
func TestDeploy_DeployFailure_TriggersRollback(t *testing.T) {
	provider := &mockDeployProvider{
		deployErr:   errors.New("OOM killed"),
		restoredTag: "abc000",
	}
	uc := deploy.New(provider, nil, t.TempDir())

	_, err := uc.Execute(context.Background(), deploy.Input{Env: "prod", Tag: "abc123"})

	require.Error(t, err)
	assert.True(t, provider.rollbackCalled)
}

// Rollback lui-même échoue → erreur explicite avec instructions de recovery manuel.
func TestDeploy_RollbackFailure_ReturnsExplicitError(t *testing.T) {
	provider := &mockDeployProvider{
		deployErr:   errors.New("OOM"),
		rollbackErr: errors.New("rollback also failed"),
	}
	uc := deploy.New(provider, nil, t.TempDir())

	_, err := uc.Execute(context.Background(), deploy.Input{Env: "prod", Tag: "abc123"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback")
}

// Secrets injectés avant le deploy.
func TestDeploy_WithSecrets_InjectsBeforeDeployy(t *testing.T) {
	provider := &mockDeployProvider{statuses: healthyStatuses()}
	secrets := &mockSecretManager{
		resolved: map[string]string{"DATABASE_URL": "postgres://..."},
	}
	uc := deploy.New(provider, secrets, t.TempDir())

	_, err := uc.Execute(context.Background(), deploy.Input{
		Env:        "prod",
		Tag:        "abc123",
		SecretRefs: map[string]string{"DATABASE_URL": "DATABASE_URL"},
	})

	require.NoError(t, err)
	assert.True(t, secrets.called)
	assert.True(t, provider.deployCalled)
}

// Dry-run : aucun appel au provider, le plan est retourné.
func TestDeploy_DryRun_NoExecution(t *testing.T) {
	provider := &mockDeployProvider{}
	uc := deploy.New(provider, nil, t.TempDir())

	out, err := uc.Execute(context.Background(), deploy.Input{
		Env:    "prod",
		Tag:    "abc123",
		DryRun: true,
	})

	require.NoError(t, err)
	assert.False(t, provider.syncCalled)
	assert.False(t, provider.deployCalled)
	assert.NotNil(t, out.DryRunPlan)
}
