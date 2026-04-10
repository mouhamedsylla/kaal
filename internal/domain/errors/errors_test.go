package errors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pilotErrors "github.com/mouhamedsylla/pilot/internal/domain/errors"
)

// TypeA — auto-fix silencieux, aucune interaction requise.
func TestTypeA_AutoFixSilent(t *testing.T) {
	err := pilotErrors.NewTypeA("PILOT-SSH-001", "SSH key permissions incorrect (0644)")

	assert.Equal(t, pilotErrors.TypeA, err.Type)
	assert.Equal(t, "PILOT-SSH-001", err.Code)
	assert.False(t, err.RequiresChoice())
	assert.False(t, err.RequiresHuman())
	assert.False(t, err.CanDryRun())
}

// TypeB — auto-fix annoncé, supporte --dry-run.
func TestTypeB_AutoFixAnnounced(t *testing.T) {
	err := pilotErrors.NewTypeB("PILOT-DOCKER-001", "Docker not running")

	assert.Equal(t, pilotErrors.TypeB, err.Type)
	assert.True(t, err.CanDryRun())
	assert.False(t, err.RequiresChoice())
	assert.False(t, err.RequiresHuman())
}

// TypeC — choix requis, options connues. L'opération est suspendue.
func TestTypeC_ChoiceRequired(t *testing.T) {
	err := pilotErrors.NewTypeC("PILOT-NET-001", "Port 8080 in use",
		pilotErrors.WithOptions([]string{"8081", "8082", "3001"}, "8081"),
		pilotErrors.WithAppliesTo("environments.dev.ports.api"),
	)

	assert.Equal(t, pilotErrors.TypeC, err.Type)
	assert.True(t, err.RequiresChoice())
	assert.False(t, err.RequiresHuman())
	assert.Equal(t, []string{"8081", "8082", "3001"}, err.Options)
	assert.Equal(t, "8081", err.Recommended)
	assert.Equal(t, "environments.dev.ports.api", err.AppliesTo)
}

// TypeC sans options — invalide : RequiresChoice vrai mais Options vide.
func TestTypeC_WithoutOptions_OptionsEmpty(t *testing.T) {
	err := pilotErrors.NewTypeC("PILOT-NET-002", "No available ports")

	assert.True(t, err.RequiresChoice())
	assert.Empty(t, err.Options)
}

// TypeD — stop complet, instructions exactes pour l'humain.
func TestTypeD_StopWithInstructions(t *testing.T) {
	err := pilotErrors.NewTypeD("PILOT-SECRET-001", "DATABASE_URL is not set",
		"Add DATABASE_URL to .env.prod then run: pilot deploy")

	assert.Equal(t, pilotErrors.TypeD, err.Type)
	assert.True(t, err.RequiresHuman())
	assert.False(t, err.RequiresChoice())
	assert.Equal(t, "Add DATABASE_URL to .env.prod then run: pilot deploy", err.Instructions)
}

// Tous les types satisfont l'interface error standard.
func TestAllTypes_SatisfyErrorInterface(t *testing.T) {
	errors := []error{
		pilotErrors.NewTypeA("PILOT-A-001", "msg a"),
		pilotErrors.NewTypeB("PILOT-B-001", "msg b"),
		pilotErrors.NewTypeC("PILOT-C-001", "msg c"),
		pilotErrors.NewTypeD("PILOT-D-001", "msg d", "do this"),
	}

	for _, err := range errors {
		assert.NotNil(t, err)
		assert.NotEmpty(t, err.Error())
	}
}

// Error() doit exposer le code pour que les logs soient identifiables.
func TestError_MessageContainsCode(t *testing.T) {
	err := pilotErrors.NewTypeA("PILOT-SSH-001", "bad permissions")

	assert.Contains(t, err.Error(), "PILOT-SSH-001")
	assert.Contains(t, err.Error(), "bad permissions")
}
