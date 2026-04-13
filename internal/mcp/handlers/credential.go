package handlers

import (
	"context"
	"fmt"
	"os"

	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/local"
)

// HandleCredentialSet sets a key=value in the pilot process environment
// AND persists it to .env.local. Called by pilot-agent's collect_credential
// virtual tool after the user has typed the value inline in the terminal.
//
// Because pilot mcp serve IS the running process, os.Setenv is visible to all
// subsequent tool calls (pilot_push, pilot_deploy…) within the same session.
func HandleCredentialSet(_ context.Context, params map[string]any) (any, error) {
	key := strParam(params, "key")
	value := strParam(params, "value")

	if key == "" {
		return nil, fmt.Errorf("credential_set: key is required")
	}

	// 1. Inject into the current process — immediately visible to push/deploy.
	if err := os.Setenv(key, value); err != nil {
		return nil, fmt.Errorf("credential_set: setenv: %w", err)
	}

	// 2. Persist to .env.local so future sessions don't need to re-enter.
	if err := local.SetInFile(".env.local", key, value); err != nil {
		// Non-fatal: the in-process value is already set.
		return map[string]any{
			"ok":      true,
			"key":     key,
			"warning": fmt.Sprintf("could not persist to .env.local: %v", err),
		}, nil
	}

	return map[string]any{
		"ok":        true,
		"key":       key,
		"persisted": ".env.local",
	}, nil
}
