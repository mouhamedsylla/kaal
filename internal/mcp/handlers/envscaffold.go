package handlers

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/adapters/secrets/local"
	pilotenv "github.com/mouhamedsylla/pilot/internal/env"
)

// HandleEnvScaffold returns the list of variable *names* expected for a given
// environment, sourced exclusively from .env.example.
//
// It never reads .env.dev or any other env file — no value leaks between envs.
// It never creates any file.
//
// The agent uses this to inform the user what variables are needed, then lets
// the user decide whether to configure them (via collect_credential) or skip.
func HandleEnvScaffold(_ context.Context, params map[string]any) (any, error) {
	env := strParam(params, "env")
	if env == "" {
		env = pilotenv.Active("")
	}

	// .env.example is the only legitimate source for variable names.
	const exampleFile = ".env.example"

	info, err := os.Stat(exampleFile)
	if err != nil || info.IsDir() {
		return map[string]any{
			"env":       env,
			"source":    nil,
			"variables": []string{},
			"count":     0,
			"note":      "No .env.example found. If your app needs environment variables, create .env." + env + " manually.",
		}, nil
	}

	vars, err := local.ListFile(exampleFile)
	if err != nil {
		return nil, fmt.Errorf("env_scaffold: read .env.example: %w", err)
	}

	// Extract names only — sorted for deterministic output.
	// We deliberately discard values: they belong to .env.example as placeholders,
	// not as production values.
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, k)
	}
	sort.Strings(names)

	// Check which of these are already set in the target env file or process env.
	targetFile := fmt.Sprintf(".env.%s", env)
	existingVars, _ := local.ListFile(targetFile) // best-effort, file may not exist

	missing := make([]string, 0)
	configured := make([]string, 0)

	for _, name := range names {
		alreadyInFile := existingVars != nil && existingVars[name] != ""
		alreadyInEnv := os.Getenv(name) != ""

		if alreadyInFile || alreadyInEnv {
			configured = append(configured, name)
		} else {
			missing = append(missing, name)
		}
	}

	note := ""
	switch {
	case len(names) == 0:
		note = ".env.example exists but contains no variables. Your app may not need env vars."
	case len(missing) == 0:
		note = fmt.Sprintf("All %d variables from .env.example are already configured for %q.", len(names), env)
	default:
		note = fmt.Sprintf("%d/%d variables still need to be configured for %q. Values are never copied from other envs.",
			len(missing), len(names), env)
	}

	// Classify missing variables as secret vs non-secret by name convention.
	// This is a hint for the agent — it can use secret=true in collect_credential.
	type varInfo struct {
		Name   string `json:"name"`
		Secret bool   `json:"secret"` // hint: mask input when collecting
	}
	missingInfo := make([]varInfo, 0, len(missing))
	for _, name := range missing {
		missingInfo = append(missingInfo, varInfo{
			Name:   name,
			Secret: looksLikeSecret(name),
		})
	}

	return map[string]any{
		"env":        env,
		"source":     exampleFile,
		"target":     targetFile,
		"all":        names,
		"configured": configured,
		"missing":    missingInfo,
		"count":      len(names),
		"note":       note,
	}, nil
}

// looksLikeSecret returns true if the variable name suggests it holds a
// sensitive value (password, token, key, secret…).
func looksLikeSecret(name string) bool {
	lower := strings.ToLower(name)
	for _, keyword := range []string{
		"password", "passwd", "secret", "token", "key", "apikey", "api_key",
		"credential", "private", "cert", "auth", "jwt", "signing",
	} {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}
