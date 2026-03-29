package gcp_sm

import (
	"context"
	"fmt"
)

// SecretManager is a stub — GCP Secret Manager support is not yet implemented.
type SecretManager struct{}

func New() *SecretManager { return &SecretManager{} }

func (s *SecretManager) Get(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("gcp_sm: not yet implemented")
}

func (s *SecretManager) Set(_ context.Context, _, _ string) error {
	return fmt.Errorf("gcp_sm: not yet implemented")
}

func (s *SecretManager) Inject(_ context.Context, _ string, _ map[string]string) (map[string]string, error) {
	return nil, fmt.Errorf("gcp_sm: not yet implemented")
}
