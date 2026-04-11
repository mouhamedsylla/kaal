package preflight_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/app/preflight"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

func baseConfig() *config.Config {
	return &config.Config{
		Project: config.Project{Name: "my-app", Stack: "go"},
		Registry: config.RegistryConfig{
			Provider: "ghcr",
			Image:    "ghcr.io/user/my-app",
		},
		Environments: map[string]config.Environment{
			"prod": {Target: "vps-prod"},
		},
	}
}

func TestGenerateLock_MinimalProject(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pilot.yaml"), []byte("project:\n  name: my-app\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-compose.prod.yml"), []byte("services: {}"), 0644))

	l, err := preflight.GenerateLock(baseConfig(), dir, "prod")

	require.NoError(t, err)
	assert.NotEmpty(t, l.ProjectHash)
	assert.Equal(t, 1, l.SchemaVersion)
	assert.Equal(t, "compose", l.ExecutionProvider)

	active := l.ActiveNodes()
	assert.True(t, active[plan.StepPreflight])
	assert.True(t, active[plan.StepDeploy])
	assert.True(t, active[plan.StepHealthcheck])
	assert.False(t, active[plan.StepMigrations])
	assert.False(t, active[plan.StepPreHooks])
	assert.False(t, active[plan.StepPostHooks])
}

func TestGenerateLock_WithPrisma(t *testing.T) {
	dir := t.TempDir()
	// Create prisma/schema.prisma to trigger auto-detection.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "prisma"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prisma", "schema.prisma"), []byte("datasource db {}"), 0644))

	l, err := preflight.GenerateLock(baseConfig(), dir, "prod")

	require.NoError(t, err)
	active := l.ActiveNodes()
	assert.True(t, active[plan.StepMigrations])
	require.NotNil(t, l.ExecutionPlan.Migrations)
	assert.Equal(t, "prisma", l.ExecutionPlan.Migrations.Tool)
	assert.Equal(t, "npx prisma migrate deploy", l.ExecutionPlan.Migrations.Command)
	assert.False(t, l.ExecutionPlan.Migrations.Reversible)
}

func TestGenerateLock_WithAlembic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alembic.ini"), []byte("[alembic]"), 0644))

	l, err := preflight.GenerateLock(baseConfig(), dir, "prod")

	require.NoError(t, err)
	require.NotNil(t, l.ExecutionPlan.Migrations)
	assert.Equal(t, "alembic", l.ExecutionPlan.Migrations.Tool)
}

func TestGenerateLock_WithDeclaredMigrations(t *testing.T) {
	cfg := baseConfig()
	cfg.Environments["prod"] = config.Environment{
		Target: "vps-prod",
		Migrations: &config.Migrations{
			Tool:            "goose",
			Command:         "goose postgres $DATABASE_URL up",
			RollbackCommand: "goose postgres $DATABASE_URL down",
			Reversible:      true,
		},
	}

	dir := t.TempDir()
	l, err := preflight.GenerateLock(cfg, dir, "prod")

	require.NoError(t, err)
	require.NotNil(t, l.ExecutionPlan.Migrations)
	assert.Equal(t, "goose", l.ExecutionPlan.Migrations.Tool)
	assert.True(t, l.ExecutionPlan.Migrations.Reversible)
	assert.Equal(t, "pilot.yaml", l.ExecutionPlan.Migrations.DetectedFrom)
}

func TestGenerateLock_WithHooks(t *testing.T) {
	cfg := baseConfig()
	cfg.Environments["prod"] = config.Environment{
		Target: "vps-prod",
		Hooks: &config.Hooks{
			PreDeploy:  []config.HookCommand{{Command: "echo pre"}},
			PostDeploy: []config.HookCommand{{Command: "echo post"}},
		},
	}

	dir := t.TempDir()
	l, err := preflight.GenerateLock(cfg, dir, "prod")

	require.NoError(t, err)
	active := l.ActiveNodes()
	assert.True(t, active[plan.StepPreHooks])
	assert.True(t, active[plan.StepPostHooks])
}

func TestGenerateLock_StepOrder(t *testing.T) {
	cfg := baseConfig()
	cfg.Environments["prod"] = config.Environment{
		Target: "vps-prod",
		Hooks: &config.Hooks{
			PreDeploy:  []config.HookCommand{{Command: "echo pre"}},
			PostDeploy: []config.HookCommand{{Command: "echo post"}},
		},
		Migrations: &config.Migrations{
			Tool:    "prisma",
			Command: "npx prisma migrate deploy",
		},
	}

	dir := t.TempDir()
	l, err := preflight.GenerateLock(cfg, dir, "prod")

	require.NoError(t, err)
	steps := l.ExecutionPlan.NodesActive

	// Expected order: preflight → pre_hooks → migrations → deploy → post_hooks → healthcheck
	pos := func(name plan.StepName) int {
		for i, s := range steps {
			if s == name {
				return i
			}
		}
		return -1
	}

	assert.Less(t, pos(plan.StepPreflight), pos(plan.StepPreHooks))
	assert.Less(t, pos(plan.StepPreHooks), pos(plan.StepMigrations))
	assert.Less(t, pos(plan.StepMigrations), pos(plan.StepDeploy))
	assert.Less(t, pos(plan.StepDeploy), pos(plan.StepPostHooks))
	assert.Less(t, pos(plan.StepPostHooks), pos(plan.StepHealthcheck))
}

func TestGenerateLock_HashChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	pilotYaml := filepath.Join(dir, "pilot.yaml")
	require.NoError(t, os.WriteFile(pilotYaml, []byte("v1"), 0644))

	l1, err := preflight.GenerateLock(baseConfig(), dir, "prod")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(pilotYaml, []byte("v2"), 0644))

	l2, err := preflight.GenerateLock(baseConfig(), dir, "prod")
	require.NoError(t, err)

	assert.NotEqual(t, l1.ProjectHash, l2.ProjectHash)
}

func TestGenerateLock_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pilot.yaml"), []byte("p: v"), 0644))

	l, err := preflight.GenerateLock(baseConfig(), dir, "prod")
	require.NoError(t, err)

	require.NoError(t, l.Write(dir))

	// Verify the file was created and is readable.
	data, err := os.ReadFile(filepath.Join(dir, "pilot.lock"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "schema_version")
	assert.Contains(t, string(data), "project_hash")
}
