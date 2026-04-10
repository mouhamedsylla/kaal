package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mouhamedsylla/pilot/internal/domain/state"
)

// ── Machine à états — transitions ────────────────────────────────────────────

func TestMachine_ValidTransitions(t *testing.T) {
	cases := []struct {
		from  state.MachineState
		event state.Event
		want  state.MachineState
	}{
		{state.StateIdle, state.EventStart, state.StatePreflighting},
		{state.StatePreflighting, state.EventOK, state.StateExecuting},
		{state.StatePreflighting, state.EventTypeAB, state.StateRecovering},
		{state.StatePreflighting, state.EventTypeC, state.StateAwaitingChoice},
		{state.StateRecovering, state.EventResume, state.StatePreflighting},
		{state.StateAwaitingChoice, state.EventResume, state.StatePreflighting},
		{state.StateExecuting, state.EventOK, state.StateSucceeded},
		{state.StateExecuting, state.EventTypeAB, state.StateRecovering},
		{state.StateExecuting, state.EventTypeC, state.StateAwaitingChoice},
		{state.StateExecuting, state.EventTypeD, state.StateGuidedFailure},
	}

	for _, tc := range cases {
		m := state.Machine{Current: tc.from}
		err := m.Transition(tc.event)
		assert.NoError(t, err, "from=%s event=%s", tc.from, tc.event)
		assert.Equal(t, tc.want, m.Current, "from=%s event=%s", tc.from, tc.event)
	}
}

func TestMachine_InvalidTransition(t *testing.T) {
	m := state.Machine{Current: state.StateIdle}
	err := m.Transition(state.EventOK) // impossible depuis idle
	assert.Error(t, err)
	assert.Equal(t, state.StateIdle, m.Current) // état inchangé
}

func TestMachine_TerminalStatesAreBlocked(t *testing.T) {
	for _, terminal := range []state.MachineState{state.StateSucceeded, state.StateGuidedFailure} {
		m := state.Machine{Current: terminal}
		err := m.Transition(state.EventStart)
		assert.Error(t, err, "terminal state %s should reject all events", terminal)
	}
}

// ── State.json — persistance ──────────────────────────────────────────────────

func TestState_WriteAndRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	s := state.New("dev")
	s.LastOperation = &state.OperationRecord{
		Command:     "pilot deploy",
		Env:         "prod",
		Status:      "succeeded",
		StartedAt:   time.Now().UTC().Truncate(time.Second),
		CompletedAt: time.Now().UTC().Truncate(time.Second),
	}

	err := s.Write(dir)
	require.NoError(t, err)

	loaded, err := state.Read(dir)
	require.NoError(t, err)

	assert.Equal(t, s.ActiveEnv, loaded.ActiveEnv)
	assert.Equal(t, s.LastOperation.Command, loaded.LastOperation.Command)
	assert.Equal(t, s.LastOperation.Status, loaded.LastOperation.Status)
}

func TestState_Write_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "pilot")
	s := state.New("dev")
	err := s.Write(dir)
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(dir, "state.json"))
	assert.NoError(t, statErr)
}

func TestState_Read_MissingFile_ReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	s, err := state.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, state.StateIdle, s.MachineState)
}

// ── PendingChoice ─────────────────────────────────────────────────────────────

func TestState_PendingChoice_SetAndClear(t *testing.T) {
	s := state.New("dev")
	assert.False(t, s.HasPendingChoice())

	s.SetPendingChoice(state.PendingChoice{
		Code:               "PILOT-NET-001",
		Prompt:             "Port 8080 in use. Choose port for api.",
		Options:            []string{"8081", "8082"},
		Recommended:        "8081",
		AppliesTo:          "environments.dev.ports.api",
		OperationSuspended: "pilot deploy",
	})
	assert.True(t, s.HasPendingChoice())

	s.ClearPendingChoice()
	assert.False(t, s.HasPendingChoice())
}
