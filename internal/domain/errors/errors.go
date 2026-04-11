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

// Exit codes — stable across releases, safe to use in scripts and CI.
const (
	ExitOK       = 0
	ExitGeneral  = 1
	ExitConfig   = 2
	ExitDeploy   = 3
	ExitSecrets  = 4
	ExitSSH      = 5
	ExitRegistry = 6
	ExitNotFound = 7
)

// ErrorType classifies a pilot error by who acts and how.
type ErrorType int

const (
	TypeA ErrorType = iota // auto-fix, silent
	TypeB                  // auto-fix, announced, dry-run safe
	TypeC                  // suspend, present options, wait for choice
	TypeD                  // stop, print exact human instructions
)

// PilotError is the single error type for all pilot failures.
// Fields are populated according to the ErrorType.
type PilotError struct {
	Type    ErrorType
	Code    string // e.g. PILOT-SSH-001
	Message string
	Exit    int   // os.Exit code (use ExitXxx constants)
	Cause   error // wrapped cause for errors.Is / errors.As

	// TypeC only — pilot suspends until the user picks an option
	Options     []string // available choices (human-readable)
	Recommended string   // which option pilot recommends
	AppliesTo   string   // pilot.yaml key affected by the choice

	// TypeD only — only a human can unblock this
	Instructions string // step-by-step instructions for the user
}

func (e *PilotError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *PilotError) Unwrap() error { return e.Cause }

// RequiresChoice reports whether the operation must be suspended
// until the caller provides a choice (TypeC).
func (e *PilotError) RequiresChoice() bool { return e.Type == TypeC }

// RequiresHuman reports whether only a human can unblock the situation (TypeD).
func (e *PilotError) RequiresHuman() bool { return e.Type == TypeD }

// CanDryRun reports whether the fix can be previewed without executing (TypeB).
func (e *PilotError) CanDryRun() bool { return e.Type == TypeB }

// ── constructors ─────────────────────────────────────────────────────────────

func NewTypeA(code, message string) *PilotError {
	return &PilotError{Type: TypeA, Code: code, Message: message, Exit: ExitOK}
}

func NewTypeB(code, message string, exit int) *PilotError {
	return &PilotError{Type: TypeB, Code: code, Message: message, Exit: exit}
}

func NewTypeC(code, message string, exit int, opts ...Option) *PilotError {
	e := &PilotError{Type: TypeC, Code: code, Message: message, Exit: exit}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func NewTypeD(code, message, instructions string, exit int, opts ...Option) *PilotError {
	e := &PilotError{Type: TypeD, Code: code, Message: message, Instructions: instructions, Exit: exit}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithCause wraps the root error cause (for errors.Is / errors.As chains).
func WithCause(cause error) Option {
	return func(e *PilotError) { e.Cause = cause }
}

// ── TypeC options ─────────────────────────────────────────────────────────────

// Option configures a TypeC (or any) error.
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
	return func(e *PilotError) { e.AppliesTo = key }
}

// ── helpers ──────────────────────────────────────────────────────────────────

// ExitCodeOf extracts the exit code from any error.
// Returns ExitGeneral if the error does not carry a code.
func ExitCodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	var pe *PilotError
	// Use a manual walk instead of errors.As to avoid import cycle issues.
	for e := err; e != nil; {
		if p, ok := e.(*PilotError); ok {
			pe = p
			break
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			break
		}
		e = u.Unwrap()
	}
	if pe != nil {
		return pe.Exit
	}
	return ExitGeneral
}
