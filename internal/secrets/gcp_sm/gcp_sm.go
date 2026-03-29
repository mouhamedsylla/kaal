// Package gcp_sm implements SecretManager backed by GCP Secret Manager.
// It delegates to the `gcloud` CLI — no SDK dependency required.
// Requires: gcloud CLI installed and authenticated (gcloud auth login / Workload Identity).
package gcp_sm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SecretManager fetches secrets from GCP Secret Manager via the gcloud CLI.
type SecretManager struct {
	Project string // GCP project ID (optional — uses gcloud default if empty)
}

func New() *SecretManager { return &SecretManager{} }

// NewWithProject creates a SecretManager for a specific GCP project.
func NewWithProject(project string) *SecretManager {
	return &SecretManager{Project: project}
}

// Get retrieves the latest version of a secret by name.
func (s *SecretManager) Get(ctx context.Context, key string) (string, error) {
	args := []string{"secrets", "versions", "access", "latest", "--secret", key}
	if s.Project != "" {
		args = append(args, "--project", s.Project)
	}
	out, err := run(ctx, "gcloud", args...)
	if err != nil {
		return "", fmt.Errorf("gcp_sm: get %q: %w", key, err)
	}
	return strings.TrimSpace(out), nil
}

// Set creates or updates a secret. Creates the secret first if it doesn't exist.
func (s *SecretManager) Set(ctx context.Context, key, value string) error {
	projectArgs := []string{}
	if s.Project != "" {
		projectArgs = []string{"--project", s.Project}
	}

	// Ensure the secret exists (create if needed, ignore error if it already exists).
	createArgs := append([]string{"secrets", "create", key,
		"--replication-policy", "automatic"}, projectArgs...)
	_, _ = run(ctx, "gcloud", createArgs...) // ignore error — secret may already exist

	// Add a new version.
	addArgs := append([]string{"secrets", "versions", "add", key,
		"--data-file", "-"}, projectArgs...)
	cmd := exec.CommandContext(ctx, "gcloud", addArgs...)
	cmd.Stdin = strings.NewReader(value)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("gcp_sm: set %q: %s", key, strings.TrimSpace(string(ee.Stderr)))
		}
		return fmt.Errorf("gcp_sm: set %q: %w", key, err)
	}
	_ = out
	return nil
}

// Inject resolves all refs from GCP Secret Manager.
// refs maps ENV_VAR → secret name in GCP SM.
func (s *SecretManager) Inject(ctx context.Context, _ string, refs map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(refs))
	for envVar, secretRef := range refs {
		val, err := s.Get(ctx, secretRef)
		if err != nil {
			return nil, err
		}
		result[envVar] = val
	}
	return result, nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", name, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}
