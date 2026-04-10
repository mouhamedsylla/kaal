package plan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

// ── CompensationSteps ────────────────────────────────────────────────────────

// La compensation s'exécute en ordre inverse (LIFO).
func TestCompensation_IsLIFO(t *testing.T) {
	p := plan.Plan{
		Steps: []plan.Step{
			{Name: plan.StepMigrations, Reversible: true, RollbackCommand: "migrate down"},
			{Name: plan.StepDeploy, Reversible: true, RollbackCommand: "restore image"},
		},
	}

	comp := p.CompensationSteps()

	require.Len(t, comp, 2)
	assert.Equal(t, plan.StepDeploy, comp[0].Name)
	assert.Equal(t, plan.StepMigrations, comp[1].Name)
}

// Les steps irréversibles sont exclus de la compensation.
func TestCompensation_ExcludesIrreversible(t *testing.T) {
	p := plan.Plan{
		Steps: []plan.Step{
			{Name: plan.StepMigrations, Reversible: false},
			{Name: plan.StepDeploy, Reversible: true, RollbackCommand: "restore image"},
			{Name: plan.StepHealthcheck, Reversible: false},
		},
	}

	comp := p.CompensationSteps()

	require.Len(t, comp, 1)
	assert.Equal(t, plan.StepDeploy, comp[0].Name)
}

// Aucun step réversible → compensation vide.
func TestCompensation_EmptyWhenNothingReversible(t *testing.T) {
	p := plan.Plan{
		Steps: []plan.Step{
			{Name: plan.StepMigrations, Reversible: false},
			{Name: plan.StepPush, Reversible: false},
		},
	}

	assert.Empty(t, p.CompensationSteps())
}

// La compensation porte uniquement sur les steps exécutés avant l'échec,
// pas sur l'ensemble du plan.
func TestCompensation_OnlyExecutedSteps(t *testing.T) {
	p := plan.Plan{
		Steps: []plan.Step{
			{Name: plan.StepMigrations, Reversible: true, RollbackCommand: "migrate down"},
			{Name: plan.StepPush, Reversible: false},
			{Name: plan.StepDeploy, Reversible: true, RollbackCommand: "restore image"},
			{Name: plan.StepHealthcheck, Reversible: false},
		},
	}

	// Échec au step deploy (index 2) — healthcheck n'a pas tourné.
	comp := p.CompensationFor(plan.StepDeploy)

	require.Len(t, comp, 2)
	assert.Equal(t, plan.StepDeploy, comp[0].Name)   // LIFO : deploy d'abord
	assert.Equal(t, plan.StepMigrations, comp[1].Name)
}

// ── ActiveSteps ──────────────────────────────────────────────────────────────

// Seuls les steps actifs participent à l'exécution.
func TestActiveSteps_OnlyActive(t *testing.T) {
	p := plan.Plan{
		Steps: []plan.Step{
			{Name: plan.StepPreflight, Active: true},
			{Name: plan.StepMigrations, Active: false}, // pas de migrations dans ce projet
			{Name: plan.StepPush, Active: true},
			{Name: plan.StepDeploy, Active: true},
			{Name: plan.StepHealthcheck, Active: true},
		},
	}

	active := p.ActiveSteps()

	require.Len(t, active, 4)
	assert.Equal(t, plan.StepPreflight, active[0].Name)
	assert.Equal(t, plan.StepPush, active[1].Name)
}

// ── reversible default ───────────────────────────────────────────────────────

// reversible: false est le défaut sûr — une Step sans Reversible explicite
// ne doit jamais apparaître dans le plan de compensation.
func TestStep_ReversibleDefaultIsFalse(t *testing.T) {
	s := plan.Step{Name: plan.StepMigrations}
	assert.False(t, s.Reversible)
}
