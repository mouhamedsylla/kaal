package handlers

import (
	"context"
	"fmt"
	"os"

	kaalctx "github.com/mouhamedsylla/kaal/internal/context"
	"github.com/mouhamedsylla/kaal/internal/env"
)

// HandleContext returns the full project context for AI agents.
// This is the primary MCP tool — agents call it first to understand the project
// before generating any infrastructure files.
func HandleContext(_ context.Context, params map[string]any) (any, error) {
	activeEnv := env.Active(strParam(params, "env"))

	projCtx, err := kaalctx.Collect(activeEnv)
	if err != nil {
		return nil, fmt.Errorf("collect context: %w", err)
	}

	return map[string]any{
		"kaal_yaml":              projCtx.KaalYAML,
		"stack":                  projCtx.Stack,
		"language_version":       projCtx.LanguageVersion,
		"is_existing_project":    projCtx.IsExistingProject,
		"file_tree":              projCtx.FileTree,
		"key_files":              projCtx.KeyFiles,
		"existing_dockerfiles":   projCtx.ExistingDockerfiles,
		"existing_compose_files": projCtx.ExistingComposeFiles,
		"existing_env_files":     projCtx.ExistingEnvFiles,
		"missing_dockerfile":     projCtx.MissingDockerfile,
		"missing_compose":        projCtx.MissingCompose,
		"active_env":             projCtx.ActiveEnv,
		"agent_prompt":           projCtx.AgentPrompt(),
		"services":               projCtx.Config.Services,
		"environments":           projCtx.Config.Environments,
	}, nil
}

// HandleGenerateDockerfile writes a Dockerfile provided by the agent.
func HandleGenerateDockerfile(_ context.Context, params map[string]any) (any, error) {
	content := strParam(params, "content")
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	dest := "Dockerfile"
	if p := strParam(params, "path"); p != "" {
		dest = p
	}

	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write %s: %w", dest, err)
	}

	return map[string]any{
		"written": dest,
		"message": fmt.Sprintf("Dockerfile written to %s", dest),
	}, nil
}

// HandleGenerateCompose writes a docker-compose file provided by the agent.
func HandleGenerateCompose(_ context.Context, params map[string]any) (any, error) {
	content := strParam(params, "content")
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	activeEnv := env.Active(strParam(params, "env"))
	dest := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
	if p := strParam(params, "path"); p != "" {
		dest = p
	}

	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("write %s: %w", dest, err)
	}

	return map[string]any{
		"written": dest,
		"message": fmt.Sprintf("docker-compose file written to %s — run 'kaal up' to start", dest),
	}, nil
}

// strParam safely extracts a string param from the map.
func strParam(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return v
}
