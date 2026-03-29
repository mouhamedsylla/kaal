package vps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxDeployHistory = 10

// DeployRecord captures one completed deployment.
type DeployRecord struct {
	Tag     string    `json:"tag"`
	Env     string    `json:"env"`
	At      time.Time `json:"at"`
	OK      bool      `json:"ok"`
	Message string    `json:"message,omitempty"`
}

// DeployState is the full state written to deployments.json on the VPS.
type DeployState struct {
	Deployments []DeployRecord `json:"deployments"`
}

// recordDeploy appends a DeployRecord to deployments.json on the remote VPS
// and trims the history to maxDeployHistory entries.
func (p *Provider) recordDeploy(ctx context.Context, client interface {
	Run(context.Context, string) (string, error)
}, env, tag string, ok bool, msg string) {
	rec := DeployRecord{
		Tag:     tag,
		Env:     env,
		At:      time.Now().UTC(),
		OK:      ok,
		Message: msg,
	}

	stateDir := p.stateDir()
	stateFile := stateDir + "/deployments.json"

	// Read existing state (best-effort — empty if not present).
	existing, _ := client.Run(ctx, fmt.Sprintf("cat %s 2>/dev/null || echo '{}'", stateFile))
	var state DeployState
	json.Unmarshal([]byte(strings.TrimSpace(existing)), &state) //nolint:errcheck

	// Prepend new record and cap history.
	state.Deployments = append([]DeployRecord{rec}, state.Deployments...)
	if len(state.Deployments) > maxDeployHistory {
		state.Deployments = state.Deployments[:maxDeployHistory]
	}

	data, err := json.Marshal(state)
	if err != nil {
		return // never fail a deploy because of state recording
	}

	// Write atomically via temp file + move.
	tmp := stateFile + ".tmp"
	writeCmd := fmt.Sprintf("echo '%s' > %s && mv %s %s",
		strings.ReplaceAll(string(data), "'", `'\''`), // escape single quotes
		tmp, tmp, stateFile,
	)
	client.Run(ctx, writeCmd) //nolint:errcheck
}

// ReadHistory retrieves the deployment history from the remote VPS.
func (p *Provider) ReadHistory(ctx context.Context) ([]DeployRecord, error) {
	client, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	out, err := client.Run(ctx, fmt.Sprintf("cat %s/deployments.json 2>/dev/null || echo '{}'", p.stateDir()))
	if err != nil {
		return nil, fmt.Errorf("read history: %w", err)
	}

	var state DeployState
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &state); err != nil {
		return nil, fmt.Errorf("parse history: %w", err)
	}
	return state.Deployments, nil
}
