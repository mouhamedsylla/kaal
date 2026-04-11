package deploy_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/deploy"
	domain "github.com/mouhamedsylla/pilot/internal/domain"
	"github.com/mouhamedsylla/pilot/internal/domain/lock"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockDeployProvider struct {
	syncErr   error
	deployErr error
	rollbackTag string
	rollbackErr error
	statuses  []domain.ServiceStatus
	statusErr error
}

func (m *mockDeployProvider) Sync(_ context.Context, _ string) error                        { return m.syncErr }
func (m *mockDeployProvider) Deploy(_ context.Context, _ string, _ domain.DeployOptions) error { return m.deployErr }
func (m *mockDeployProvider) Rollback(_ context.Context, _ string, _ string) (string, error) {
	return m.rollbackTag, m.rollbackErr
}
func (m *mockDeployProvider) Status(_ context.Context, _ string) ([]domain.ServiceStatus, error) {
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

func (m *mockSecretManager) Inject(_ context.Context, _ string, _ map[string]string) (map[string]string, error) {
	m.called = true
	return m.resolved, m.err
}

type mockHookRunner struct {
	err      error
	commands []string
}

func (m *mockHookRunner) RunHooks(_ context.Context, cmds []string) error {
	m.commands = append(m.commands, cmds...)
	return m.err
}

type mockMigrationRunner struct {
	runErr      error
	rollbackErr error
	ran         bool
	rolledBack  bool
}

func (m *mockMigrationRunner) RunMigrations(_ context.Context, _ domain.MigrationConfig) error {
	m.ran = true
	return m.runErr
}
func (m *mockMigrationRunner) RollbackMigrations(_ context.Context, _ domain.MigrationConfig) error {
	m.rolledBack = true
	return m.rollbackErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

func healthy(names ...string) []domain.ServiceStatus {
	out := make([]domain.ServiceStatus, len(names))
	for i, n := range names {
		out[i] = domain.ServiceStatus{Name: n, State: "running", Health: "healthy"}
	}
	return out
}

// writeLock writes a minimal valid pilot.lock to dir and returns its hash.
func writeLock(t *testing.T, dir string, nodes []plan.StepName, migCfg *lock.MigrationConfig) {
	t.Helper()
	l := &lock.Lock{
		SchemaVersion: 1,
		ProjectHash:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		GeneratedFrom: []string{},
		ExecutionPlan: lock.ExecutionPlan{
			NodesActive: nodes,
			Migrations:  migCfg,
		},
		ExecutionProvider: "compose",
	}
	require.NoError(t, l.Write(dir))
}

func minimalNodes() []plan.StepName {
	return []plan.StepName{plan.StepPreflight, plan.StepDeploy, plan.StepHealthcheck}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestDeploy_AllOK(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{statuses: healthy("api")}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	out, err := u.Execute(context.Background(), deploy.Input{
		Env:           "prod",
		Tag:           "v1.0.0",
		SkipLockCheck: true,
	})

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", out.Tag)
	assert.Len(t, out.Statuses, 1)
}

func TestDeploy_SyncFails(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{syncErr: errors.New("SSH refused")}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1", SkipLockCheck: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH refused")
}

func TestDeploy_DeployFails_AutoRollback(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{
		deployErr:   errors.New("compose up failed"),
		rollbackTag: "v0.9.0",
	}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1", SkipLockCheck: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "compose up failed")
	assert.Contains(t, err.Error(), "v0.9.0")
}

func TestDeploy_HealthcheckUnhealthy_AutoRollback(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{
		statuses:    []domain.ServiceStatus{{Name: "api", State: "running", Health: "unhealthy"}},
		rollbackTag: "v0.9.0",
	}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1", SkipLockCheck: true,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy")
	assert.Contains(t, err.Error(), "v0.9.0")
}

func TestDeploy_WithSecrets(t *testing.T) {
	dir := t.TempDir()
	secrets := &mockSecretManager{resolved: map[string]string{"DB_URL": "postgres://..."}}
	provider := &mockDeployProvider{statuses: healthy("api")}
	u := deploy.New(deploy.Config{
		Provider: provider, Secrets: secrets, StateDir: dir, ProjectDir: dir,
	})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env:           "prod",
		Tag:           "v1",
		SecretRefs:    map[string]string{"DB_URL": "prod/db-url"},
		SkipLockCheck: true,
	})

	require.NoError(t, err)
	assert.True(t, secrets.called)
}

func TestDeploy_WithPreHooks(t *testing.T) {
	dir := t.TempDir()
	hooks := &mockHookRunner{}
	provider := &mockDeployProvider{statuses: healthy("api")}
	u := deploy.New(deploy.Config{
		Provider: provider, Hooks: hooks, StateDir: dir, ProjectDir: dir,
	})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env:           "prod",
		Tag:           "v1",
		PreHooks:      []string{"echo pre-deploy"},
		SkipLockCheck: true,
	})

	require.NoError(t, err)
	assert.Contains(t, hooks.commands, "echo pre-deploy")
}

func TestDeploy_WithMigrations_Success(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, append(minimalNodes(), plan.StepMigrations), &lock.MigrationConfig{
		Tool:    "prisma",
		Command: "npx prisma migrate deploy",
	})

	mig := &mockMigrationRunner{}
	provider := &mockDeployProvider{statuses: healthy("api")}
	u := deploy.New(deploy.Config{
		Provider: provider, Migrations: mig, StateDir: dir, ProjectDir: dir,
	})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1",
	})

	require.NoError(t, err)
	assert.True(t, mig.ran)
	assert.False(t, mig.rolledBack)
}

func TestDeploy_WithMigrations_Failure_Compensates(t *testing.T) {
	dir := t.TempDir()
	writeLock(t, dir, append(minimalNodes(), plan.StepMigrations), &lock.MigrationConfig{
		Tool:            "prisma",
		Command:         "npx prisma migrate deploy",
		RollbackCommand: "npx prisma migrate rollback",
		Reversible:      true,
	})

	mig := &mockMigrationRunner{runErr: errors.New("migration failed")}
	provider := &mockDeployProvider{}
	u := deploy.New(deploy.Config{
		Provider: provider, Migrations: mig, StateDir: dir, ProjectDir: dir,
	})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "migration failed")
	assert.True(t, mig.ran)
	assert.True(t, mig.rolledBack)
}

func TestDeploy_LockNotFound(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1",
		// SkipLockCheck: false (default)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pilot.lock not found")
	assert.Contains(t, err.Error(), "pilot preflight")
}

func TestDeploy_LockStale(t *testing.T) {
	dir := t.TempDir()
	// Write a lock with a hash that won't match (files are empty).
	l := &lock.Lock{
		SchemaVersion: 1,
		ProjectHash:   "old-hash-will-not-match",
		GeneratedFrom: []string{}, // empty list → current hash will be different
		ExecutionPlan: lock.ExecutionPlan{NodesActive: minimalNodes()},
	}
	require.NoError(t, l.Write(dir))

	// Write a file that changes the hash.
	require.NoError(t, os.WriteFile(dir+"/pilot.yaml", []byte("changed"), 0644))

	// Re-write lock pointing at the file.
	l.GeneratedFrom = []string{dir + "/pilot.yaml"}
	l.ProjectHash = "definitely-old"
	require.NoError(t, l.Write(dir))

	provider := &mockDeployProvider{}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	_, err := u.Execute(context.Background(), deploy.Input{
		Env: "prod", Tag: "v1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale")
}

func TestDeploy_DryRun(t *testing.T) {
	dir := t.TempDir()
	provider := &mockDeployProvider{}
	u := deploy.New(deploy.Config{Provider: provider, StateDir: dir, ProjectDir: dir})

	out, err := u.Execute(context.Background(), deploy.Input{
		Env:           "prod",
		Tag:           "v1.0.0",
		DryRun:        true,
		SkipLockCheck: true,
	})

	require.NoError(t, err)
	require.NotNil(t, out.DryRunPlan)
	assert.NotEmpty(t, out.DryRunPlan.Steps)
	// Provider methods should not have been called.
	assert.Nil(t, out.Statuses)
}
