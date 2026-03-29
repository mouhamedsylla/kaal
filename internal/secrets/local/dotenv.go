package local

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// SecretManager reads and writes secrets to local .env.<env> files.
type SecretManager struct{}

func New() *SecretManager { return &SecretManager{} }

// Get retrieves a secret value from environment variables.
func (s *SecretManager) Get(_ context.Context, key string) (string, error) {
	if val := os.Getenv(key); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("secret %q not found in environment", key)
}

// Set persists a key=value to the current process environment.
func (s *SecretManager) Set(_ context.Context, key, value string) error {
	return os.Setenv(key, value)
}

// SetInFile writes or updates a KEY=VALUE line in the given .env file.
// Creates the file if it does not exist. File mode is 0600.
func SetInFile(path, key, value string) error {
	vars, _ := parseEnvFile(path) // ignore error — file may not exist yet
	if vars == nil {
		vars = map[string]string{}
	}
	vars[key] = value
	return writeEnvFile(path, vars)
}

// ListFile returns all key=value pairs from a .env file.
func ListFile(path string) (map[string]string, error) {
	return parseEnvFile(path)
}

// Inject resolves all secret refs for an environment from .env.<env> + os env.
func (s *SecretManager) Inject(_ context.Context, env string, refs map[string]string) (map[string]string, error) {
	envFile := fmt.Sprintf(".env.%s", env)
	fileVars, _ := parseEnvFile(envFile) // best-effort

	result := make(map[string]string, len(refs))
	for envVar, secretRef := range refs {
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

// ── file I/O ─────────────────────────────────────────────────────────────────

func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := map[string]string{}
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
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		vars[k] = v
	}
	return vars, scanner.Err()
}

// writeEnvFile serialises a map back to KEY=VALUE format (0600 permissions).
func writeEnvFile(path string, vars map[string]string) error {
	var sb strings.Builder
	sb.WriteString("# Managed by kaal — do not commit this file.\n")
	for k, v := range vars {
		if strings.ContainsAny(v, " \t\"'") {
			sb.WriteString(fmt.Sprintf(`%s="%s"`, k, v))
		} else {
			sb.WriteString(fmt.Sprintf("%s=%s", k, v))
		}
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0600)
}
