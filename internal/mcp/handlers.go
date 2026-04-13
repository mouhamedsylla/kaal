package mcp

import (
	"context"

	"github.com/mouhamedsylla/pilot/internal/mcp/handlers"
)

// Context + infra generation — implemented
var handleContext HandlerFunc = handlers.HandleContext
var handleGenerateDockerfile HandlerFunc = handlers.HandleGenerateDockerfile
var handleGenerateCompose HandlerFunc = handlers.HandleGenerateCompose

// Environment lifecycle — implemented
var handleEnvSwitch HandlerFunc = handlers.HandleEnvSwitch
var handleUp HandlerFunc = handlers.HandleUp
var handleDown HandlerFunc = handlers.HandleDown
var handlePush HandlerFunc = handlers.HandlePush
var handleDeploy HandlerFunc = handlers.HandleDeploy
var handleRollback HandlerFunc = handlers.HandleRollback
var handleSync HandlerFunc = handlers.HandleSync

// Observability — implemented
var handleStatus HandlerFunc = handlers.HandleStatus
var handleLogs HandlerFunc = handlers.HandleLogs

// Secrets — implemented
var handleSecretsInject HandlerFunc = handlers.HandleSecretsInject

// Setup — implemented (VPS/hetzner only)
var handleSetup HandlerFunc = handlers.HandleSetup

// Preflight — implemented
var handlePreflight HandlerFunc = handlers.HandlePreflight

// Credential set — injects a key=value into the running process + .env.local
var handleCredentialSet HandlerFunc = handlers.HandleCredentialSet

// Env scaffold — returns variable names from .env.example (read-only, no values)
var handleEnvScaffold HandlerFunc = handlers.HandleEnvScaffold

// Env create — creates .env.<env> with generated secrets + defaults + documented placeholders
var handleEnvCreate HandlerFunc = handlers.HandleEnvCreate

// pilot_init — non-interactive, accepts name/stack/services/envs/registry params.
var handleInit HandlerFunc = handlers.HandleInit

// pilot_vps_exec — runs a single command on the remote VPS via SSH (always destructive).
var handleVpsExec HandlerFunc = handlers.HandleVpsExec

// ensure context is used
var _ = context.Background
