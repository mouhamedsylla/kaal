package mcp

// registerAll wires up all MCP tools to their handlers.
func (s *Server) registerAll() {
	// Context + infra generation — primary AI agent workflow
	s.Register(toolContext, handleContext)
	s.Register(toolGenerateDockerfile, handleGenerateDockerfile)
	s.Register(toolGenerateCompose, handleGenerateCompose)

	// Environment lifecycle
	s.Register(toolInit, handleInit)
	s.Register(toolEnvSwitch, handleEnvSwitch)
	s.Register(toolUp, handleUp)
	s.Register(toolDown, handleDown)

	// Registry + deployment
	s.Register(toolPush, handlePush)
	s.Register(toolDeploy, handleDeploy)
	s.Register(toolRollback, handleRollback)
	s.Register(toolSync, handleSync)

	// Observability
	s.Register(toolStatus, handleStatus)
	s.Register(toolLogs, handleLogs)

	// Config + secrets
	s.Register(toolConfigGet, handleConfigGet)
	s.Register(toolConfigSet, handleConfigSet)
	s.Register(toolSecretsInject, handleSecretsInject)

	// VPS setup
	s.Register(toolSetup, handleSetup)
}

// ──────────────────── context + infra generation ────────────────────

var toolContext = Tool{
	Name: "kaal_context",
	Description: `Return the complete project context for this kaal project.
Call this FIRST before generating any infrastructure files.
Returns: kaal.yaml, file tree, detected stack, existing Dockerfiles/compose files,
service definitions, missing file list, and a ready-to-use agent_prompt.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment to collect context for (defaults to active env)"},
		},
	},
}

var toolGenerateDockerfile = Tool{
	Name: "kaal_generate_dockerfile",
	Description: `Write a Dockerfile to the project directory.
Call kaal_context first to understand the project stack and requirements.
The agent is responsible for generating the Dockerfile content — kaal writes it to disk.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"content": {Type: "string", Description: "Full Dockerfile content to write"},
			"path":    {Type: "string", Description: "Destination path (default: Dockerfile)"},
		},
		Required: []string{"content"},
	},
}

var toolGenerateCompose = Tool{
	Name: "kaal_generate_compose",
	Description: `Write a docker-compose.<env>.yml to the project directory.
Call kaal_context first to understand the services and environment configuration.
The agent is responsible for generating the compose file content — kaal writes it to disk.
After writing, the agent should tell the user to run 'kaal up'.`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"content": {Type: "string", Description: "Full docker-compose YAML content to write"},
			"env":     {Type: "string", Description: "Environment name (default: active env) — determines filename docker-compose.<env>.yml"},
			"path":    {Type: "string", Description: "Override destination path (optional)"},
		},
		Required: []string{"content"},
	},
}

// ──────────────────────── environment lifecycle ────────────────────────

var toolInit = Tool{
	Name:        "kaal_init",
	Description: "Initialize a new kaal project with scaffold, Dockerfiles, and kaal.yaml",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"name":         {Type: "string", Description: "Project name"},
			"stack":        {Type: "string", Description: "Language stack", Enum: []string{"go", "node", "python", "rust"}},
			"registry":     {Type: "string", Description: "Registry provider", Enum: []string{"ghcr", "dockerhub", "custom"}},
			"envs":         {Type: "string", Description: "Comma-separated list of environments (default: dev,staging,prod)"},
			"orchestrator": {Type: "string", Description: "Orchestrator type", Enum: []string{"compose", "k8s"}},
		},
		Required: []string{"name", "stack"},
	},
}

var toolEnvSwitch = Tool{
	Name:        "kaal_env_switch",
	Description: "Switch the active kaal environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Target environment name (e.g. dev, staging, prod)"},
		},
		Required: []string{"env"},
	},
}

var toolUp = Tool{
	Name:        "kaal_up",
	Description: "Start local services for the active environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment to start (defaults to active env)"},
			"services": {Type: "string", Description: "Comma-separated list of services to start (defaults to all)"},
		},
	},
}

var toolDown = Tool{
	Name:        "kaal_down",
	Description: "Stop and remove local services for the active environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment to stop (defaults to active env)"},
		},
	},
}

var toolPush = Tool{
	Name:        "kaal_push",
	Description: "Build the Docker image and push it to the configured registry. Defaults to linux/amd64 for VPS compatibility.",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"tag":      {Type: "string", Description: "Image tag (defaults to git short SHA)"},
			"no_cache": {Type: "string", Description: "Disable build cache (true/false)"},
			"platform": {Type: "string", Description: "Target platform (default: linux/amd64). Use linux/arm64 for ARM VPS, linux/amd64,linux/arm64 for multi-arch."},
		},
	},
}

var toolDeploy = Tool{
	Name:        "kaal_deploy",
	Description: "Deploy the application to a remote target (VPS or cloud)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment to deploy (defaults to active env)"},
			"tag":      {Type: "string", Description: "Image tag to deploy (defaults to git short SHA)"},
			"target":   {Type: "string", Description: "Target name from kaal.yaml (overrides env default)"},
			"strategy": {Type: "string", Description: "Deployment strategy", Enum: []string{"rolling", "blue-green", "canary"}},
			"dry_run":  {Type: "string", Description: "Show what would happen without executing (true/false)"},
		},
	},
}

var toolRollback = Tool{
	Name:        "kaal_rollback",
	Description: "Roll back to a previous deployment version",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":     {Type: "string", Description: "Environment to roll back"},
			"target":  {Type: "string", Description: "Target name"},
			"version": {Type: "string", Description: "Version tag to roll back to (defaults to previous)"},
		},
		Required: []string{"env"},
	},
}

var toolSync = Tool{
	Name:        "kaal_sync",
	Description: "Synchronize local kaal.yaml and compose files to the remote target",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"target": {Type: "string", Description: "Target name from kaal.yaml"},
		},
	},
}

var toolStatus = Tool{
	Name:        "kaal_status",
	Description: "Return the complete project state as JSON (local containers + remote services)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Filter by environment (optional)"},
		},
	},
}

var toolLogs = Tool{
	Name:        "kaal_logs",
	Description: "Return logs for a service (local or remote based on active env)",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"service": {Type: "string", Description: "Service name"},
			"lines":   {Type: "string", Description: "Number of lines to return (default 100)"},
			"since":   {Type: "string", Description: "Return logs since this duration (e.g. 5m, 1h)"},
		},
	},
}

var toolConfigGet = Tool{
	Name:        "kaal_config_get",
	Description: "Read a value from kaal.yaml using dot-notation key",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"key": {Type: "string", Description: "Dot-notation key (e.g. project.name, registry.provider)"},
		},
		Required: []string{"key"},
	},
}

var toolConfigSet = Tool{
	Name:        "kaal_config_set",
	Description: "Set a value in kaal.yaml using dot-notation key",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"key":   {Type: "string", Description: "Dot-notation key"},
			"value": {Type: "string", Description: "New value"},
		},
		Required: []string{"key", "value"},
	},
}

var toolSetup = Tool{
	Name: "kaal_setup",
	Description: `Run one-time VPS setup tasks required before the first deploy.
Connects via SSH and adds the deploy user to the docker group.
Call this when kaal_deploy fails with a docker permission error.
Requires password-less sudo on the VPS (standard on Hetzner, DigitalOcean, OVH with cloud-init).`,
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env": {Type: "string", Description: "Environment whose target VPS to configure (defaults to active env)"},
		},
	},
}

var toolSecretsInject = Tool{
	Name:        "kaal_secrets_inject",
	Description: "Inject secrets from the configured secret manager into the target environment",
	InputSchema: InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"env":      {Type: "string", Description: "Environment (dev, staging, prod)"},
			"provider": {Type: "string", Description: "Secret provider override (local, aws_sm, gcp_sm)"},
		},
		Required: []string{"env"},
	},
}
