package mcp

import (
	"context"

	"github.com/mouhamedsylla/kaal/internal/mcp/handlers"
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

// Config — implemented
var handleConfigGet HandlerFunc = handlers.HandleConfigGet
var handleConfigSet HandlerFunc = handlers.HandleConfigSet

// Secrets — implemented
var handleSecretsInject HandlerFunc = handlers.HandleSecretsInject

// kaal_init — non-interactive, accepts name/stack/services/envs/registry params.
var handleInit HandlerFunc = handlers.HandleInit

// ensure context is used
var _ = context.Background
