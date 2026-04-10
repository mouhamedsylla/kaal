// Package aws_sm implements SecretManager backed by AWS Secrets Manager.
// It delegates to the `aws` CLI — no SDK dependency required.
// Requires: aws CLI installed and configured (aws configure / IAM role / env vars).
package aws_sm

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SecretManager fetches secrets from AWS Secrets Manager via the aws CLI.
type SecretManager struct{}

func New() *SecretManager { return &SecretManager{} }

// Get retrieves a secret string by its ARN or name.
func (s *SecretManager) Get(ctx context.Context, key string) (string, error) {
	out, err := run(ctx, "aws", "secretsmanager", "get-secret-value",
		"--secret-id", key,
		"--query", "SecretString",
		"--output", "text",
	)
	if err != nil {
		return "", fmt.Errorf("aws_sm: get %q: %w", key, err)
	}
	return strings.TrimSpace(out), nil
}

// Set creates or updates a secret value.
func (s *SecretManager) Set(ctx context.Context, key, value string) error {
	_, err := run(ctx, "aws", "secretsmanager", "put-secret-value",
		"--secret-id", key,
		"--secret-string", value,
	)
	if err != nil {
		// Secret may not exist yet — create it.
		_, err2 := run(ctx, "aws", "secretsmanager", "create-secret",
			"--name", key,
			"--secret-string", value,
		)
		if err2 != nil {
			return fmt.Errorf("aws_sm: set %q: %w", key, err2)
		}
	}
	return nil
}

// Inject resolves all refs from AWS Secrets Manager.
// refs maps ENV_VAR → secret name/ARN in AWS SM.
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
