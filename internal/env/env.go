// Package env manages the active kaal environment.
// The active environment is persisted in .kaal-current-env at the project root.
package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const stateFile = ".kaal-current-env"

// Active returns the current environment name, in order of priority:
//  1. explicit override (non-empty string passed by the caller)
//  2. .kaal-current-env file at the project root
//  3. "dev" as the universal default
func Active(override string) string {
	if override != "" {
		return override
	}
	if env, err := read(); err == nil && env != "" {
		return env
	}
	return "dev"
}

// Use sets the active environment and writes it to .kaal-current-env.
func Use(env string) error {
	if env == "" {
		return fmt.Errorf("environment name cannot be empty")
	}
	return os.WriteFile(stateFile, []byte(env), 0644)
}

// Current reads the active environment from .kaal-current-env.
// Returns "dev" if the file does not exist.
func Current() string {
	env, err := read()
	if err != nil || env == "" {
		return "dev"
	}
	return env
}

// StateFilePath returns the absolute path to .kaal-current-env.
func StateFilePath() string {
	abs, _ := filepath.Abs(stateFile)
	return abs
}

func read() (string, error) {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
