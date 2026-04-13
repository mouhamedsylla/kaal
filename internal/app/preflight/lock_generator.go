package preflight

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
// or auto-detects it by analysing the project's dependency files.
// Returns nil when no migration tool is found.
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

	// Dependency-based detection — highest confidence, language-agnostic.
	if m := detectFromDependencies(projectDir); m != nil {
		return m
	}

	// Fallback: well-known config files that unambiguously identify a tool.
	type fileDetector struct {
		file    string
		tool    string
		command string
	}
	fileDetectors := []fileDetector{
		{"prisma/schema.prisma", "prisma", "npx prisma migrate deploy"},
		{"alembic.ini", "alembic", "alembic upgrade head"},
		{"migrations/alembic.ini", "alembic", "alembic upgrade head"},
		{"flyway.conf", "flyway", "flyway migrate"},
		// Goose: only when a go.mod is also present (Go project).
		{"db/migrations", "goose", "goose up"},
	}

	for _, d := range fileDetectors {
		candidate := filepath.Join(projectDir, d.file)
		if _, err := os.Stat(candidate); err == nil {
			return &lock.MigrationConfig{
				Tool:         d.tool,
				Command:      d.command,
				Reversible:   false,
				DetectedFrom: d.file,
			}
		}
	}
	return nil
}

// ── dependency-based migration detection ─────────────────────────────────────

// migRule maps a dependency name fragment to a migration tool config.
type migRule struct {
	fragment string
	tool     string
	command  string
}

// detectFromDependencies scans the project's dependency files to identify
// the migration tool in use. More reliable than folder heuristics because
// it reads what the project actually declares.
func detectFromDependencies(projectDir string) *lock.MigrationConfig {
	// Python — pyproject.toml, requirements*.txt, setup.cfg
	if m := detectPython(projectDir); m != nil {
		return m
	}
	// Go — go.mod
	if m := detectGo(projectDir); m != nil {
		return m
	}
	// Node — package.json
	if m := detectNode(projectDir); m != nil {
		return m
	}
	// Ruby — Gemfile
	if m := detectRuby(projectDir); m != nil {
		return m
	}
	return nil
}

var pythonMigRules = []migRule{
	{"alembic", "alembic", "alembic upgrade head"},
	{"django", "django", "python manage.py migrate"},
	{"peewee-migrate", "peewee-migrate", "python -m peewee_migrate migrate"},
	{"tortoise-orm", "aerich", "aerich upgrade"},
	{"aerich", "aerich", "aerich upgrade"},
	{"sqlmodel", "alembic", "alembic upgrade head"}, // sqlmodel usually pairs with alembic
	{"piccolo", "piccolo", "piccolo migrations forwards all"},
}

func detectPython(projectDir string) *lock.MigrationConfig {
	// Files to scan, in priority order.
	candidates := []string{
		"pyproject.toml", "requirements.txt", "requirements-base.txt",
		"requirements/base.txt", "requirements/prod.txt", "setup.cfg",
	}
	for _, f := range candidates {
		content := readFileLower(filepath.Join(projectDir, f))
		if content == "" {
			continue
		}
		for _, r := range pythonMigRules {
			if strings.Contains(content, r.fragment) {
				return &lock.MigrationConfig{
					Tool:         r.tool,
					Command:      r.command,
					Reversible:   false,
					DetectedFrom: f,
				}
			}
		}
	}
	return nil
}

var goMigRules = []migRule{
	{"pressly/goose", "goose", "goose -dir migrations up"},
	{"golang-migrate", "golang-migrate", "migrate -path migrations -database $DATABASE_URL up"},
	{"uptrace/bun", "bun", "bun db migrate"},
	{"go-gormigrate", "gormigrate", ""},
	{"gorm.io/gorm", "gorm", ""}, // gorm AutoMigrate — no CLI
}

func detectGo(projectDir string) *lock.MigrationConfig {
	content := readFileLower(filepath.Join(projectDir, "go.mod"))
	if content == "" {
		return nil
	}
	for _, r := range goMigRules {
		if r.command == "" {
			continue // no CLI command — skip
		}
		if strings.Contains(content, r.fragment) {
			return &lock.MigrationConfig{
				Tool:         r.tool,
				Command:      r.command,
				Reversible:   false,
				DetectedFrom: "go.mod",
			}
		}
	}
	return nil
}

var nodeMigRules = []migRule{
	{"prisma", "prisma", "npx prisma migrate deploy"},
	{"knex", "knex", "npx knex migrate:latest"},
	{"sequelize", "sequelize-cli", "npx sequelize-cli db:migrate"},
	{"typeorm", "typeorm", "npx typeorm migration:run -d src/data-source.ts"},
	{"@mikro-orm", "mikro-orm", "npx mikro-orm migration:up"},
	{"umzug", "umzug", ""},
	{"db-migrate", "db-migrate", "npx db-migrate up"},
}

func detectNode(projectDir string) *lock.MigrationConfig {
	content := readFileLower(filepath.Join(projectDir, "package.json"))
	if content == "" {
		return nil
	}
	for _, r := range nodeMigRules {
		if r.command == "" {
			continue
		}
		if strings.Contains(content, r.fragment) {
			return &lock.MigrationConfig{
				Tool:         r.tool,
				Command:      r.command,
				Reversible:   false,
				DetectedFrom: "package.json",
			}
		}
	}
	return nil
}

var rubyMigRules = []migRule{
	{"activerecord", "rails", "bundle exec rails db:migrate"},
	{"sequel", "sequel", "bundle exec sequel -m db/migrations $DATABASE_URL"},
	{"rom-sql", "rom", "bundle exec rom migrations migrate"},
}

func detectRuby(projectDir string) *lock.MigrationConfig {
	content := readFileLower(filepath.Join(projectDir, "Gemfile"))
	if content == "" {
		return nil
	}
	for _, r := range rubyMigRules {
		if strings.Contains(content, r.fragment) {
			return &lock.MigrationConfig{
				Tool:         r.tool,
				Command:      r.command,
				Reversible:   false,
				DetectedFrom: "Gemfile",
			}
		}
	}
	return nil
}

// readFileLower reads a file and returns its content lowercased,
// or "" if the file does not exist or cannot be read.
func readFileLower(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToLower(string(data))
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
