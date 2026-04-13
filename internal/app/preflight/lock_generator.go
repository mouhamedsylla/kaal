package preflight

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/domain/lock"
	"github.com/mouhamedsylla/pilot/internal/domain/plan"
)

// GenerateLock builds a pilot.lock from the project state and configuration.
// Called at the end of a successful preflight for target=deploy.
func GenerateLock(cfg *config.Config, projectDir, env string) (*lock.Lock, error) {
	if projectDir == "" {
		projectDir = "."
	}

	// ── Determine active nodes ─────────────────────────────────────────────
	active := []plan.StepName{plan.StepPreflight, plan.StepDeploy, plan.StepHealthcheck}

	envCfg := cfg.Environments[env]

	// pre_hooks
	if envCfg.Hooks != nil && len(envCfg.Hooks.PreDeploy) > 0 {
		active = insertBefore(active, plan.StepDeploy, plan.StepPreHooks)
	}

	// migrations — declared in config or auto-detected from project files
	migCfg := resolveMigrationConfig(cfg, projectDir, env)
	if migCfg != nil {
		active = insertBefore(active, plan.StepDeploy, plan.StepMigrations)
	}

	// post_hooks
	if envCfg.Hooks != nil && len(envCfg.Hooks.PostDeploy) > 0 {
		active = insertAfter(active, plan.StepDeploy, plan.StepPostHooks)
	}

	// ── Source files (for staleness hashing) ──────────────────────────────
	// Use absolute paths so the hash survives directory changes.
	pilotYaml := filepath.Join(projectDir, "pilot.yaml")
	sources := []string{pilotYaml}
	composeFile := filepath.Join(projectDir, fmt.Sprintf("docker-compose.%s.yml", env))
	if _, err := os.Stat(composeFile); err == nil {
		sources = append(sources, composeFile)
	}
	if migCfg != nil && migCfg.DetectedFrom != "" {
		// DetectedFrom is relative — make it absolute.
		detectedAbs := filepath.Join(projectDir, migCfg.DetectedFrom)
		sources = append(sources, detectedAbs)
	}

	hash, err := hashFiles(sources)
	if err != nil {
		return nil, fmt.Errorf("lock: compute hash: %w", err)
	}

	// ── Execution provider ─────────────────────────────────────────────────
	provider := "compose"
	if envCfg.Runtime != "" {
		provider = envCfg.Runtime
	}

	l := &lock.Lock{
		SchemaVersion: 1,
		GeneratedAt:   time.Now().UTC(),
		GeneratedFrom: sources,
		ProjectHash:   hash,
		ExecutionPlan: lock.ExecutionPlan{
			NodesActive: active,
			Migrations:  migCfg,
		},
		ExecutionProvider: provider,
	}
	return l, nil
}

// ── migration detection ───────────────────────────────────────────────────────

// resolveMigrationConfig returns a MigrationConfig from the declared config
// or auto-detects it from well-known project files. Returns nil when no
// migration tool is found.
func resolveMigrationConfig(cfg *config.Config, projectDir, env string) *lock.MigrationConfig {
	// User-declared takes priority.
	if envCfg := cfg.Environments[env]; envCfg.Migrations != nil {
		m := envCfg.Migrations
		if m.Tool != "" && m.Command != "" {
			return &lock.MigrationConfig{
				Tool:            m.Tool,
				Command:         m.Command,
				RollbackCommand: m.RollbackCommand,
				Reversible:      m.Reversible,
				DetectedFrom:    "pilot.yaml",
			}
		}
	}

	// Auto-detect from well-known files.
	type detector struct {
		file    string
		tool    string
		command string
	}
	detectors := []detector{
		{"prisma/schema.prisma", "prisma", "npx prisma migrate deploy"},
		// Alembic — racine ou sous-dossier migrations/
		{"alembic.ini", "alembic", "alembic upgrade head"},
		{"migrations/alembic.ini", "alembic", "alembic upgrade head"},
		{"migrations/env.py", "alembic", "alembic upgrade head"},
		{"flyway.conf", "flyway", "flyway migrate"},
		// Goose — seulement si go.mod présent (projet Go) + dossier db/migrations
		{"db/migrations", "goose", "goose up"},
		// NB: "migrations/" seul n'est PAS détecté — trop générique, trop de faux positifs.
		// Déclare l'outil dans pilot.yaml si nécessaire.
	}

	for _, d := range detectors {
		candidate := filepath.Join(projectDir, d.file)
		if _, err := os.Stat(candidate); err == nil {
			return &lock.MigrationConfig{
				Tool:         d.tool,
				Command:      d.command,
				Reversible:   false, // default — user must opt in
				DetectedFrom: d.file,
			}
		}
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// hashFiles returns the hex-encoded SHA-256 of the concatenated contents of all
// readable files. Missing files are silently skipped (stale detection will catch
// newly added files at the next preflight run).
func hashFiles(paths []string) (string, error) {
	h := sha256.New()
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if info.IsDir() {
			continue // directories are tracked by name only, not content
		}
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// insertBefore inserts newStep immediately before target in the slice.
// If target is not found, newStep is appended at the end.
func insertBefore(steps []plan.StepName, target, newStep plan.StepName) []plan.StepName {
	for i, s := range steps {
		if s == target {
			out := make([]plan.StepName, 0, len(steps)+1)
			out = append(out, steps[:i]...)
			out = append(out, newStep)
			out = append(out, steps[i:]...)
			return out
		}
	}
	return append(steps, newStep)
}

// insertAfter inserts newStep immediately after target in the slice.
// If target is not found, newStep is appended at the end.
func insertAfter(steps []plan.StepName, target, newStep plan.StepName) []plan.StepName {
	for i, s := range steps {
		if s == target {
			out := make([]plan.StepName, 0, len(steps)+1)
			out = append(out, steps[:i+1]...)
			out = append(out, newStep)
			out = append(out, steps[i+1:]...)
			return out
		}
	}
	return append(steps, newStep)
}
