// Package resume implements the TypeC suspension/resume mechanism.
// When pilot encounters a TypeC error, it saves the suspended operation to
// .pilot/suspended.json. "pilot resume" reads it and retries with the answer.
package resume

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const suspendedFile = ".pilot/suspended.json"

// SuspendedOp is the state saved to disk when a TypeC error occurs.
type SuspendedOp struct {
	ErrorCode   string            `json:"error_code"`   // e.g. PILOT-DEPLOY-003
	Command     string            `json:"command"`      // e.g. "deploy"
	Args        map[string]string `json:"args"`         // original command args
	Options     []string          `json:"options"`      // choices presented to user
	Recommended string            `json:"recommended"`  // pilot's recommendation
	SuspendedAt time.Time         `json:"suspended_at"`
}

// SaveSuspension writes the suspended operation to .pilot/suspended.json.
func SaveSuspension(op SuspendedOp) error {
	if err := os.MkdirAll(filepath.Dir(suspendedFile), 0755); err != nil {
		return fmt.Errorf("create .pilot/: %w", err)
	}
	op.SuspendedAt = time.Now()
	data, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(suspendedFile, data, 0644)
}

// LoadSuspension reads the suspended operation from .pilot/suspended.json.
// Returns (nil, nil) if no suspended operation exists.
func LoadSuspension() (*SuspendedOp, error) {
	data, err := os.ReadFile(suspendedFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read suspended state: %w", err)
	}
	var op SuspendedOp
	if err := json.Unmarshal(data, &op); err != nil {
		return nil, fmt.Errorf("parse suspended state: %w", err)
	}
	return &op, nil
}

// ClearSuspension removes the suspended state (called after successful resume).
func ClearSuspension() error {
	if err := os.Remove(suspendedFile); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Input for the resume use case.
type Input struct {
	Answer string // user's choice: index (e.g. "0") or verbatim option text
}

// Output describes what was resumed.
type Output struct {
	ErrorCode    string
	Command      string
	AppliedOption string
}

// UseCase handles the resume workflow.
type UseCase struct{}

func New() *UseCase { return &UseCase{} }

// Resolve loads the suspended state, validates the answer, and returns the
// resolved option text. The caller is responsible for actually re-running
// the command (cmd/resume.go does this by re-dispatching to the right use case).
func (uc *UseCase) Resolve(ctx context.Context, in Input) (Output, *SuspendedOp, error) {
	op, err := LoadSuspension()
	if err != nil {
		return Output{}, nil, err
	}
	if op == nil {
		return Output{}, nil, fmt.Errorf("no suspended operation found\n  Run a pilot command first")
	}

	chosen, err := resolveAnswer(op, in.Answer)
	if err != nil {
		return Output{}, nil, err
	}

	return Output{
		ErrorCode:    op.ErrorCode,
		Command:      op.Command,
		AppliedOption: chosen,
	}, op, nil
}

func resolveAnswer(op *SuspendedOp, answer string) (string, error) {
	if len(op.Options) == 0 {
		return answer, nil
	}

	// Try as numeric index first.
	idx := -1
	fmt.Sscanf(answer, "%d", &idx)
	if idx >= 0 && idx < len(op.Options) {
		return op.Options[idx], nil
	}

	// Try verbatim match.
	for _, opt := range op.Options {
		if opt == answer {
			return opt, nil
		}
	}

	// Empty answer → use recommended.
	if answer == "" && op.Recommended != "" {
		return op.Recommended, nil
	}

	return "", fmt.Errorf(
		"invalid answer %q\nValid choices:\n%s",
		answer, formatOptions(op.Options),
	)
}

func formatOptions(opts []string) string {
	var s string
	for i, o := range opts {
		s += fmt.Sprintf("  [%d] %s\n", i, o)
	}
	return s
}
