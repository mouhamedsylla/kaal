// Package meta manages the .pilot/ metadata directory.
//
// This directory stores internal state that pilot generates at runtime:
//   - compose-meta.json  — records the pilot.yaml hash at the time each
//                          compose file was generated (staleness detection)
//   - suspended.json     — TypeC suspended operation state
//
// All files in .pilot/ are local state and should NOT be committed.
// pilot ensures .gitignore contains .pilot/ on first write.
package meta

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	metaDir      = ".pilot"
	composeMetaFile = "compose-meta.json"
)

// ── ComposeMeta ───────────────────────────────────────────────────────────────

// ComposeMeta tracks the pilot.yaml hash at the time each compose file was
// generated. Used to detect when pilot.yaml has changed since last generation.
type ComposeMeta struct {
	Envs map[string]EnvComposeMeta `json:"envs"`
}

// EnvComposeMeta holds the staleness data for one environment.
type EnvComposeMeta struct {
	// PilotYAMLHash is the SHA-256 hex digest of pilot.yaml content at the time
	// the compose file was generated.
	PilotYAMLHash string `json:"pilot_yaml_hash"`

	// GeneratedAt is an RFC3339 timestamp for human reference only.
	GeneratedAt string `json:"generated_at"`

	// ComposeFile is the relative path that was written (e.g. docker-compose.dev.yml).
	ComposeFile string `json:"compose_file"`
}

// ── Hash ──────────────────────────────────────────────────────────────────────

// HashPilotYAML computes the SHA-256 hex digest of pilot.yaml in dir.
func HashPilotYAML(dir string) (string, error) {
	path := filepath.Join(dir, "pilot.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read pilot.yaml for hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ── Read / Write ──────────────────────────────────────────────────────────────

// ReadComposeMeta reads .pilot/compose-meta.json.
// Returns an empty ComposeMeta (not an error) when the file doesn't exist yet.
func ReadComposeMeta(dir string) (*ComposeMeta, error) {
	path := filepath.Join(dir, metaDir, composeMetaFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ComposeMeta{Envs: make(map[string]EnvComposeMeta)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", composeMetaFile, err)
	}
	var m ComposeMeta
	if err := json.Unmarshal(data, &m); err != nil {
		// Corrupted file — start fresh rather than blocking the user.
		return &ComposeMeta{Envs: make(map[string]EnvComposeMeta)}, nil
	}
	if m.Envs == nil {
		m.Envs = make(map[string]EnvComposeMeta)
	}
	return &m, nil
}

// RecordCompose writes the current pilot.yaml hash for env into
// .pilot/compose-meta.json. Creates the .pilot/ directory if needed.
// Non-fatal: errors are returned but should only be logged, not propagated.
func RecordCompose(dir, env, composeFile string) error {
	hash, err := HashPilotYAML(dir)
	if err != nil {
		return err
	}

	m, err := ReadComposeMeta(dir)
	if err != nil {
		m = &ComposeMeta{Envs: make(map[string]EnvComposeMeta)}
	}

	m.Envs[env] = EnvComposeMeta{
		PilotYAMLHash: hash,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ComposeFile:   composeFile,
	}

	return writeComposeMeta(dir, m)
}

func writeComposeMeta(dir string, m *ComposeMeta) error {
	metaPath := filepath.Join(dir, metaDir)
	if err := os.MkdirAll(metaPath, 0755); err != nil {
		return fmt.Errorf("create %s: %w", metaDir, err)
	}

	// Ensure .pilot/ is gitignored.
	_ = ensureGitignored(dir, metaDir+"/")

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", composeMetaFile, err)
	}

	path := filepath.Join(metaPath, composeMetaFile)
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ── Staleness check ───────────────────────────────────────────────────────────

// StalenessResult describes whether a compose file is stale.
type StalenessResult struct {
	IsStale     bool
	CurrentHash string // current pilot.yaml hash
	RecordedHash string // hash when compose was last generated
	Env         string
	ComposeFile string
}

// CheckStaleness compares the current pilot.yaml hash with the recorded hash
// for env. Returns IsStale=false when no record exists (first run).
func CheckStaleness(dir, env string) (*StalenessResult, error) {
	currentHash, err := HashPilotYAML(dir)
	if err != nil {
		return nil, err
	}

	m, err := ReadComposeMeta(dir)
	if err != nil {
		return nil, err
	}

	entry, ok := m.Envs[env]
	if !ok {
		// No record yet — compose was never generated via pilot, or
		// .pilot/ was deleted. Not stale (let pilot up proceed normally).
		return &StalenessResult{
			IsStale:     false,
			CurrentHash: currentHash,
			Env:         env,
		}, nil
	}

	return &StalenessResult{
		IsStale:      currentHash != entry.PilotYAMLHash,
		CurrentHash:  currentHash,
		RecordedHash: entry.PilotYAMLHash,
		Env:          env,
		ComposeFile:  entry.ComposeFile,
	}, nil
}

// ── .gitignore helper ─────────────────────────────────────────────────────────

func ensureGitignored(dir, pattern string) error {
	path := filepath.Join(dir, ".gitignore")
	data, _ := os.ReadFile(path)
	existing := string(data)

	for _, line := range splitLines(existing) {
		if line == pattern {
			return nil // already present
		}
	}

	entry := "\n# pilot internal state\n" + pattern + "\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
