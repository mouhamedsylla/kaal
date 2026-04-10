// Package state manages the pilot runtime state machine and its persistence
// in .pilot/state.json.
//
// pilot guarantees it only ever terminates in StateSucceeded or StateGuidedFailure —
// never in an indeterminate state. state.json is the single source of truth from
// the first write onward; in-memory state alone is never sufficient.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ── Machine à états ───────────────────────────────────────────────────────────

// MachineState is one of the stable or transient states pilot can be in.
type MachineState string

const (
	StateIdle           MachineState = "idle"
	StatePreflighting   MachineState = "preflighting"
	StateRecovering     MachineState = "recovering"      // transient — TypeA/B fix in progress
	StateAwaitingChoice MachineState = "awaiting_choice" // transient — TypeC waiting
	StateExecuting      MachineState = "executing"
	StateSucceeded      MachineState = "succeeded"      // terminal
	StateGuidedFailure  MachineState = "guided_failure" // terminal
)

// Event drives a transition in the machine.
type Event string

const (
	EventStart  Event = "start"   // begin an operation
	EventOK     Event = "ok"      // current phase succeeded, advance
	EventTypeAB Event = "type_ab" // TypeA or TypeB error detected → enter recovering
	EventTypeC  Event = "type_c"  // TypeC error detected → await choice
	EventTypeD  Event = "type_d"  // TypeD error detected → guided failure
	EventResume Event = "resume"  // choice provided or recovery done → resume prior phase
)

// Machine holds the current state and drives transitions.
type Machine struct {
	Current MachineState
}

var transitions = map[MachineState]map[Event]MachineState{
	StateIdle: {
		EventStart: StatePreflighting,
	},
	StatePreflighting: {
		EventOK:     StateExecuting,
		EventTypeAB: StateRecovering,
		EventTypeC:  StateAwaitingChoice,
	},
	StateRecovering: {
		EventResume: StatePreflighting,
	},
	StateAwaitingChoice: {
		EventResume: StatePreflighting,
	},
	StateExecuting: {
		EventOK:     StateSucceeded,
		EventTypeAB: StateRecovering,
		EventTypeC:  StateAwaitingChoice,
		EventTypeD:  StateGuidedFailure,
	},
	// StateSucceeded and StateGuidedFailure are terminal — no transitions.
}

// Transition moves the machine to the next state.
// Returns an error if the transition is invalid; state is unchanged on error.
func (m *Machine) Transition(event Event) error {
	events, ok := transitions[m.Current]
	if !ok {
		return fmt.Errorf("state %q is terminal — no transitions allowed", m.Current)
	}
	next, ok := events[event]
	if !ok {
		return fmt.Errorf("invalid transition: %q → event %q", m.Current, event)
	}
	m.Current = next
	return nil
}

// ── State (state.json) ────────────────────────────────────────────────────────

const schemaVersion = 1
const stateFile = "state.json"

// State is the full persistent state of a pilot project.
type State struct {
	SchemaVersion         int                     `json:"schema_version"`
	ActiveEnv             string                  `json:"active_env"`
	MachineState          MachineState            `json:"machine_state"`
	LastOperation         *OperationRecord        `json:"last_operation,omitempty"`
	LastSuccessPerCommand map[string]time.Time    `json:"last_success_per_command,omitempty"`
	PendingChoice         *PendingChoice          `json:"pending_choice,omitempty"`
	Deployed              map[string]DeployRecord `json:"deployed,omitempty"`
	KnownContainers       map[string][]string     `json:"known_containers,omitempty"`
}

// OperationRecord describes the last (or current) operation.
type OperationRecord struct {
	Command     string    `json:"command"`
	Env         string    `json:"env"`
	Status      string    `json:"status"` // running | succeeded | failed
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// DeployRecord tracks the currently deployed image per environment.
type DeployRecord struct {
	Image         string    `json:"image"`
	DeployedAt    time.Time `json:"deployed_at"`
	PreviousImage string    `json:"previous_image,omitempty"`
}

// PendingChoice is written when a TypeC error suspends an operation.
type PendingChoice struct {
	Code               string    `json:"code"`
	Prompt             string    `json:"prompt"`
	Options            []string  `json:"options"`
	Recommended        string    `json:"recommended"`
	AppliesTo          string    `json:"applies_to"`
	OperationSuspended string    `json:"operation_suspended"`
	SuspendedAt        time.Time `json:"suspended_at,omitempty"`
}

// New returns a fresh State for the given active environment.
func New(activeEnv string) *State {
	return &State{
		SchemaVersion:         schemaVersion,
		ActiveEnv:             activeEnv,
		MachineState:          StateIdle,
		LastSuccessPerCommand: make(map[string]time.Time),
		Deployed:              make(map[string]DeployRecord),
		KnownContainers:       make(map[string][]string),
	}
}

// HasPendingChoice reports whether an operation is suspended waiting for input.
func (s *State) HasPendingChoice() bool { return s.PendingChoice != nil }

// SetPendingChoice records a suspended TypeC choice.
func (s *State) SetPendingChoice(c PendingChoice) {
	if c.SuspendedAt.IsZero() {
		c.SuspendedAt = time.Now().UTC()
	}
	s.PendingChoice = &c
}

// ClearPendingChoice removes a resolved pending choice.
func (s *State) ClearPendingChoice() { s.PendingChoice = nil }

// Write atomically persists state to <dir>/state.json.
func (s *State) Write(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("state: create dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	// Write to a temp file then rename for atomicity.
	tmp := filepath.Join(dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("state: write tmp: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, stateFile)); err != nil {
		return fmt.Errorf("state: rename: %w", err)
	}
	return nil
}

// Read loads state from <dir>/state.json.
// If the file does not exist, returns a default idle state with no error.
func Read(dir string) (*State, error) {
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{MachineState: StateIdle}, nil
		}
		return nil, fmt.Errorf("state: read: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("state: unmarshal: %w", err)
	}
	return &s, nil
}
