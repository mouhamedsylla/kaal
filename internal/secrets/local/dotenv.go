package local

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// SecretManager reads secrets from local .env files.
type SecretManager struct{}

func New() *SecretManager {
	return &SecretManager{}
}

func (s *SecretManager) Get(_ context.Context, key string) (string, error) {
	val := os.Getenv(key)
	if val == "" {
		return "", fmt.Errorf("secret %q not found in environment", key)
	}
	return val, nil
}

func (s *SecretManager) Set(_ context.Context, key, value string) error {
	return os.Setenv(key, value)
}

func (s *SecretManager) Inject(_ context.Context, env string, refs map[string]string) (map[string]string, error) {
	envFile := fmt.Sprintf(".env.%s", env)
	fileVars, _ := parseEnvFile(envFile) // best-effort, ignore missing file

	result := make(map[string]string, len(refs))
	for envVar, secretRef := range refs {
		// secretRef is just the key name for local provider
		if val, ok := fileVars[secretRef]; ok {
			result[envVar] = val
			continue
		}
		if val := os.Getenv(secretRef); val != "" {
			result[envVar] = val
			continue
		}
		return nil, fmt.Errorf("secret %q not found in %s or environment", secretRef, envFile)
	}
	return result, nil
}

// parseEnvFile reads KEY=VALUE pairs from a .env file.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		vars[key] = val
	}
	return vars, scanner.Err()
}
