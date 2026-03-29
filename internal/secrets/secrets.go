package secrets

import "context"

// SecretManager abstracts secret backends: local .env, AWS SM, GCP SM, Azure Key Vault.
type SecretManager interface {
	// Get retrieves a single secret value by key.
	Get(ctx context.Context, key string) (string, error)

	// Set writes or updates a secret value.
	Set(ctx context.Context, key, value string) error

	// Inject resolves all secret references for an environment
	// and returns a flat map of ENV_VAR -> resolved_value.
	Inject(ctx context.Context, env string, refs map[string]string) (map[string]string, error)
}
