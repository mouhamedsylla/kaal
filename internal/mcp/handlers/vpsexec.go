package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mouhamedsylla/pilot/internal/adapters/vps"
	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	"github.com/mouhamedsylla/pilot/internal/app/runtime"
)

// denyPatterns are command fragments that are unconditionally blocked.
// This is a safety net — human approval is the primary safeguard.
var denyPatterns = []string{
	"rm -rf /",
	"rm -rf ~",
	"dd if=",
	"mkfs",
	"> /dev/sd",
	":(){ :|:& };:",  // fork bomb
	"/etc/passwd",
	"/etc/shadow",
	"chmod 777 /",
	"chown root /",
}

// HandleVpsExec runs a single command on the remote VPS via SSH.
// Every execution is logged to .pilot/vps-audit.log.
func HandleVpsExec(ctx context.Context, params map[string]any) (any, error) {
	cmd := strParam(params, "command")
	if cmd == "" {
		return nil, fmt.Errorf("vps_exec: command is required")
	}
	targetOverride := strParam(params, "target")

	// ── Deny check ────────────────────────────────────────────────────────────
	cmdLower := strings.ToLower(cmd)
	for _, pattern := range denyPatterns {
		if strings.Contains(cmdLower, strings.ToLower(pattern)) {
			_ = auditLog("DENIED", cmd, fmt.Sprintf("matches deny pattern: %q", pattern), 0)
			return nil, fmt.Errorf(
				"vps_exec: command blocked — matches deny pattern %q\n"+
					"This command is not allowed for safety reasons.",
				pattern,
			)
		}
	}

	// ── Resolve target ────────────────────────────────────────────────────────
	cfg, err := config.Load(".")
	if err != nil {
		return nil, fmt.Errorf("vps_exec: load config: %w", err)
	}

	activeEnv := env.Active("")
	targetName := targetOverride
	if targetName == "" {
		if envCfg, ok := cfg.Environments[activeEnv]; ok {
			targetName = envCfg.Target
		}
	}
	if targetName == "" {
		return nil, fmt.Errorf("vps_exec: no target found for env %q — set target in pilot.yaml or pass target param", activeEnv)
	}

	provider, err := runtime.NewDeployProvider(cfg, targetName)
	if err != nil {
		return nil, fmt.Errorf("vps_exec: provider: %w", err)
	}

	vpsProvider, ok := provider.(*vps.Provider)
	if !ok {
		return nil, fmt.Errorf("vps_exec: target %q is not a VPS — only SSH targets are supported", targetName)
	}

	// ── Execute ───────────────────────────────────────────────────────────────
	start := time.Now()
	output, execErr := vpsProvider.Exec(ctx, cmd)
	elapsed := time.Since(start)

	exitCode := 0
	if execErr != nil {
		exitCode = 1
	}

	_ = auditLog(targetName, cmd, output, exitCode)

	if execErr != nil {
		return nil, fmt.Errorf("vps_exec: %w\n\nOutput:\n%s", execErr, output)
	}

	return map[string]any{
		"target":   targetName,
		"command":  cmd,
		"output":   strings.TrimSpace(output),
		"elapsed":  elapsed.String(),
	}, nil
}

// auditLog appends a structured entry to .pilot/vps-audit.log.
func auditLog(target, command, output string, exitCode int) error {
	if err := os.MkdirAll(".pilot", 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(
		filepath.Join(".pilot", "vps-audit.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0600,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := fmt.Sprintf(
		"[%s] target=%s exit=%d\n  cmd: %s\n  out: %s\n\n",
		time.Now().UTC().Format(time.RFC3339),
		target,
		exitCode,
		command,
		strings.TrimSpace(output),
	)
	_, err = f.WriteString(entry)
	return err
}
