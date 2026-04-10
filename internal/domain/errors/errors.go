// Package errors defines the four-type error taxonomy for pilot.
//
// Every situation pilot cannot handle silently falls into one of these types.
// The type determines who acts and how — not just what went wrong.
//
//	TypeA — deterministic, low-risk:  pilot fixes silently, logs, continues.
//	TypeB — deterministic, impactful: pilot fixes and announces; supports --dry-run.
//	TypeC — choice required, options known: pilot suspends and waits for a choice.
//	TypeD — choice required, options unknown: pilot stops with exact human instructions.
package errors

import "fmt"

// ErrorType classifies a pilot error by who acts and how.
type ErrorType int

const (
	TypeA ErrorType = iota // auto-fix, silent
	TypeB                  // auto-fix, announced, dry-run safe
	TypeC                  // suspend, present options, wait
	TypeD                  // stop, exact instructions
)

// PilotError is the single error type for all pilot failures.
// Fields are populated according to the ErrorType.
type PilotError struct {
	Type    ErrorType
	Code    string // e.g. PILOT-NET-001
	Message string

	// TypeC only
	Options     []string
	Recommended string
	AppliesTo   string // pilot.yaml key affected

	// TypeD only
	Instructions string
}

func (e *PilotError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// RequiresChoice reports whether the operation must be suspended
// until the caller provides a choice (TypeC).
func (e *PilotError) RequiresChoice() bool { return e.Type == TypeC }

// RequiresHuman reports whether only a human can unblock the situation (TypeD).
func (e *PilotError) RequiresHuman() bool { return e.Type == TypeD }

// CanDryRun reports whether the fix can be previewed without executing (TypeB).
func (e *PilotError) CanDryRun() bool { return e.Type == TypeB }

// ── constructors ─────────────────────────────────────────────────────────────

func NewTypeA(code, message string) *PilotError {
	return &PilotError{Type: TypeA, Code: code, Message: message}
}

func NewTypeB(code, message string) *PilotError {
	return &PilotError{Type: TypeB, Code: code, Message: message}
}

func NewTypeC(code, message string, opts ...Option) *PilotError {
	e := &PilotError{Type: TypeC, Code: code, Message: message}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func NewTypeD(code, message, instructions string) *PilotError {
	return &PilotError{Type: TypeD, Code: code, Message: message, Instructions: instructions}
}

// ── TypeC options ─────────────────────────────────────────────────────────────

// Option configures a TypeC error.
type Option func(*PilotError)

// WithOptions sets the available choices and the recommended one.
func WithOptions(options []string, recommended string) Option {
	return func(e *PilotError) {
		e.Options = options
		e.Recommended = recommended
	}
}

// WithAppliesTo records which pilot.yaml key will be updated by the choice.
func WithAppliesTo(key string) Option {
	return func(e *PilotError) {
		e.AppliesTo = key
	}
}
