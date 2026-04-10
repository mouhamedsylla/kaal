package lock_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/domain/lock"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

// ── IsStale ───────────────────────────────────────────────────────────────────

// Même hash → lock frais.
func TestLock_NotStale_WhenHashUnchanged(t *testing.T) {
	l := lock.Lock{ProjectHash: "aabbcc"}
	assert.False(t, l.IsStale("aabbcc"))
}

// Hash différent → lock stale, pilot doit refuser de continuer.
func TestLock_Stale_WhenHashChanged(t *testing.T) {
	l := lock.Lock{ProjectHash: "aabbcc"}
	assert.True(t, l.IsStale("ddeeff"))
}

// Lock vide (jamais généré) → toujours stale.
func TestLock_Stale_WhenEmpty(t *testing.T) {
	l := lock.Lock{}
	assert.True(t, l.IsStale("anything"))
}

// ── Nodes actifs ─────────────────────────────────────────────────────────────

// ActiveNodes reflète exactement ce qui est dans le lock.
func TestLock_ActiveNodes(t *testing.T) {
	l := lock.Lock{
		ExecutionPlan: lock.ExecutionPlan{
			NodesActive: []plan.StepName{
				plan.StepPreflight,
				plan.StepMigrations,
				plan.StepPush,
				plan.StepDeploy,
				plan.StepHealthcheck,
			},
		},
	}

	active := l.ActiveNodes()
	assert.Len(t, active, 5)
	assert.Contains(t, active, plan.StepMigrations)
	assert.NotContains(t, active, plan.StepPreHooks)
}

// ── Read / Write ──────────────────────────────────────────────────────────────

func TestLock_WriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	l := lock.Lock{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC().Truncate(time.Second),
		GeneratedFrom: []string{"pilot.yaml", "docker-compose.prod.yml"},
		ProjectHash:   "abc123",
		ExecutionPlan: lock.ExecutionPlan{
			NodesActive: []plan.StepName{plan.StepPreflight, plan.StepDeploy},
			Migrations: &lock.MigrationConfig{
				Tool:            "prisma",
				Command:         "npx prisma migrate deploy",
				RollbackCommand: "npx prisma migrate rollback",
				Reversible:      true,
				DetectedFrom:    "prisma/schema.prisma",
			},
		},
		ExecutionProvider: "compose",
	}

	err := l.Write(dir)
	require.NoError(t, err)

	loaded, err := lock.Read(dir)
	require.NoError(t, err)

	assert.Equal(t, l.ProjectHash, loaded.ProjectHash)
	assert.Equal(t, l.ExecutionProvider, loaded.ExecutionProvider)
	require.NotNil(t, loaded.ExecutionPlan.Migrations)
	assert.Equal(t, "prisma", loaded.ExecutionPlan.Migrations.Tool)
	assert.True(t, loaded.ExecutionPlan.Migrations.Reversible)
}

func TestLock_Read_MissingFile_ReturnsNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := lock.Read(dir)
	assert.ErrorIs(t, err, lock.ErrNotFound)
}

func TestLock_Write_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	l := lock.Lock{ProjectHash: "x"}
	require.NoError(t, l.Write(dir))
	_, err := os.Stat(filepath.Join(dir, "pilot.lock"))
	assert.NoError(t, err)
}
