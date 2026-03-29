// Package kaalerr defines structured error types for kaal.
// Every error carries: what phase failed, what caused it, and an exit code
// so the CLI can return meaningful codes to scripts and CI pipelines.
package kaalerr

import (
	"errors"
	"fmt"
)

// Exit codes — stable across releases, safe to use in scripts.
const (
	ExitOK         = 0
	ExitGeneral    = 1
	ExitConfig     = 2
	ExitDeploy     = 3
	ExitSecrets    = 4
	ExitSSH        = 5
	ExitRegistry   = 6
	ExitNotFound   = 7
)

// ── Config ───────────────────────────────────────────────────────────────────

// ConfigError is returned when kaal.yaml cannot be loaded or is invalid.
type ConfigError struct {
	Path  string // file that was being read (may be empty if not found)
	Cause error
}

func (e *ConfigError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("config %s: %v", e.Path, e.Cause)
	}
	return fmt.Sprintf("config: %v", e.Cause)
}

func (e *ConfigError) Unwrap() error { return e.Cause }
func (e *ConfigError) ExitCode() int { return ExitConfig }

// ── Deploy ───────────────────────────────────────────────────────────────────

// DeployError is returned when a deployment step fails on the remote target.
type DeployError struct {
	Phase   string // pull | restart | state | sync
	Target  string // target name from kaal.yaml
	Command string // remote command that failed (may be empty)
	Output  string // combined stdout/stderr from the remote
	Cause   error
}

func (e *DeployError) Error() string {
	msg := fmt.Sprintf("deploy [%s] on %s", e.Phase, e.Target)
	if e.Command != "" {
		msg += fmt.Sprintf(": command %q failed", e.Command)
	}
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	if e.Output != "" {
		msg += "\n" + e.Output
	}
	return msg
}

func (e *DeployError) Unwrap() error { return e.Cause }
func (e *DeployError) ExitCode() int { return ExitDeploy }

// ── Secrets ──────────────────────────────────────────────────────────────────

// SecretsError is returned when secret resolution fails.
type SecretsError struct {
	Provider string // local | aws_sm | gcp_sm
	Key      string // secret key or ref that failed
	Cause    error
}

func (e *SecretsError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("secrets [%s]: key %q: %v", e.Provider, e.Key, e.Cause)
	}
	return fmt.Sprintf("secrets [%s]: %v", e.Provider, e.Cause)
}

func (e *SecretsError) Unwrap() error { return e.Cause }
func (e *SecretsError) ExitCode() int { return ExitSecrets }

// ── SSH ──────────────────────────────────────────────────────────────────────

// SSHError is returned when an SSH connection or command fails.
type SSHError struct {
	Host    string
	Op      string // connect | run | copy
	Cause   error
}

func (e *SSHError) Error() string {
	return fmt.Sprintf("ssh [%s] %s: %v", e.Host, e.Op, e.Cause)
}

func (e *SSHError) Unwrap() error { return e.Cause }
func (e *SSHError) ExitCode() int { return ExitSSH }

// ── Registry ─────────────────────────────────────────────────────────────────

// RegistryError is returned when a build, push or login operation fails.
type RegistryError struct {
	Op      string // login | build | push
	Image   string
	Cause   error
}

func (e *RegistryError) Error() string {
	return fmt.Sprintf("registry %s %s: %v", e.Op, e.Image, e.Cause)
}

func (e *RegistryError) Unwrap() error { return e.Cause }
func (e *RegistryError) ExitCode() int { return ExitRegistry }

// ── helpers ──────────────────────────────────────────────────────────────────

// ExitCode extracts the exit code from a kaalerr error.
// Returns ExitGeneral if the error does not carry a code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	type coder interface{ ExitCode() int }
	var c coder
	if errors.As(err, &c) {
		return c.ExitCode()
	}
	return ExitGeneral
}
