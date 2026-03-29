package mcp

import (
	"context"

	"github.com/mouhamedsylla/kaal/internal/mcp/handlers"
)

// Context + infra generation — implemented
var handleContext HandlerFunc = handlers.HandleContext
var handleGenerateDockerfile HandlerFunc = handlers.HandleGenerateDockerfile
var handleGenerateCompose HandlerFunc = handlers.HandleGenerateCompose

// All other handlers below delegate to stubs until features are built.

var handleInit HandlerFunc = handlers.Stub("kaal_init")
var handleEnvSwitch HandlerFunc = handlers.Stub("kaal_env_switch")
var handleUp HandlerFunc = handlers.Stub("kaal_up")
var handleDown HandlerFunc = handlers.Stub("kaal_down")
var handlePush HandlerFunc = handlers.Stub("kaal_push")
var handleDeploy HandlerFunc = handlers.Stub("kaal_deploy")
var handleRollback HandlerFunc = handlers.Stub("kaal_rollback")
var handleSync HandlerFunc = handlers.Stub("kaal_sync")
var handleStatus HandlerFunc = handlers.Stub("kaal_status")
var handleLogs HandlerFunc = handlers.Stub("kaal_logs")
var handleConfigGet HandlerFunc = handlers.Stub("kaal_config_get")
var handleConfigSet HandlerFunc = handlers.Stub("kaal_config_set")
var handleSecretsInject HandlerFunc = handlers.Stub("kaal_secrets_inject")

// ensure context is used
var _ = context.Background
