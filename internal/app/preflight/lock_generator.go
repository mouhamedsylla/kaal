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

	// migrations — only added when explicitly declared in pilot.yaml
	migCfg := resolveMigrationConfig(cfg, env)
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
	composeBaseName := composeFileNameForEnv(envCfg, env)
	composeFile := filepath.Join(projectDir, composeBaseName)
	if _, err := os.Stat(composeFile); err == nil {
		sources = append(sources, composeFile)
	}
	// When migrations are declared in pilot.yaml, pilot.yaml is already in sources.
	// No additional source file to track — DetectedFrom is always "pilot.yaml".

	hash, err := hashFiles(sources)
	if err != nil {
		return nil, fmt.Errorf("lock: compute hash: %w", err)
	}

	// ── Execution provider ─────────────────────────────────────────────────
	provider := "compose"
	if envCfg.Runtime != "" {
		provider = envCfg.Runtime
	}

	// When running under compose, migrations must run inside the container —
	// not on the VPS host (alembic, prisma, etc. live in the image, not on the host).
	// Wrap the raw migration command: docker compose run --rm <app-service> <cmd>
	if provider == "compose" && migCfg != nil {
		// Resolve the service name: explicit > auto-detected > error.
		appService := ""
		if envCfg.Migrations != nil && envCfg.Migrations.Service != "" {
			appService = envCfg.Migrations.Service
		} else {
			svc, err := appServiceName(cfg)
			if err != nil {
				return nil, fmt.Errorf("lock: migrations: %w", err)
			}
			appService = svc
		}

		// The compose file name on the remote: <deployDir>/<basename>.
		remoteCompose := fmt.Sprintf("%s/%s", remoteDeployDir(cfg, env), filepath.Base(composeBaseName))
		migCfg.Command = fmt.Sprintf(
			"docker compose -f %s run --rm %s %s",
			remoteCompose, appService, migCfg.Command,
		)
		if migCfg.RollbackCommand != "" {
			migCfg.RollbackCommand = fmt.Sprintf(
				"docker compose -f %s run --rm %s %s",
				remoteCompose, appService, migCfg.RollbackCommand,
			)
		}
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

// ── migration resolution ──────────────────────────────────────────────────────

// resolveMigrationConfig returns a MigrationConfig ONLY from the explicitly
// declared config in pilot.yaml. Returns nil when no migration config is declared.
// Auto-detection is intentionally excluded from the execution path — every project
// structures migrations differently, so guessing produces wrong commands.
func resolveMigrationConfig(cfg *config.Config, env string) *lock.MigrationConfig {
	envCfg := cfg.Environments[env]
	if envCfg.Migrations == nil {
		return nil
	}
	m := envCfg.Migrations
	if m.Tool == "" || m.Command == "" {
		return nil
	}
	return &lock.MigrationConfig{
		Tool:            m.Tool,
		Command:         m.Command,
		RollbackCommand: m.RollbackCommand,
		Reversible:      m.Reversible,
		DetectedFrom:    "pilot.yaml",
	}
}

// ── migration suggestion (informational only) ─────────────────────────────────

// suggestMigrationConfig analyses the project's dependency and config files to
// propose a migration command. It is NEVER used to generate the lock — only to
// produce a helpful hint when no migration is explicitly declared in pilot.yaml.
func suggestMigrationConfig(projectDir string) *lock.MigrationConfig {
	// Dependency-based suggestion — highest confidence, language-agnostic.
	if m := suggestFromDependencies(projectDir); m != nil {
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

// ── dependency-based migration suggestion ─────────────────────────────────────

// migRule maps a dependency name fragment to a migration tool config.
type migRule struct {
	fragment string
	tool     string
	command  string
}

// suggestFromDependencies scans the project's dependency files to suggest
// a migration tool and command. Used only for informational hints — never
// for generating the lock.
func suggestFromDependencies(projectDir string) *lock.MigrationConfig {
	// Python — pyproject.toml, requirements*.txt, setup.cfg
	if m := suggestPython(projectDir); m != nil {
		return m
	}
	// Go — go.mod
	if m := suggestGo(projectDir); m != nil {
		return m
	}
	// Node — package.json
	if m := suggestNode(projectDir); m != nil {
		return m
	}
	// Ruby — Gemfile
	if m := suggestRuby(projectDir); m != nil {
		return m
	}
	return nil
}

var pythonMigRules = []migRule{
	// NOTE: alembic command is post-processed in detectPython to add -c if alembic.ini
	// lives in a subdirectory (e.g. migrations/alembic.ini instead of project root).
	{"alembic", "alembic", "alembic upgrade head"},
	{"django", "django", "python manage.py migrate"},
	{"peewee-migrate", "peewee-migrate", "python -m peewee_migrate migrate"},
	{"tortoise-orm", "aerich", "aerich upgrade"},
	{"aerich", "aerich", "aerich upgrade"},
	{"sqlmodel", "alembic", "alembic upgrade head"}, // sqlmodel usually pairs with alembic
	{"piccolo", "piccolo", "piccolo migrations forwards all"},
}

func suggestPython(projectDir string) *lock.MigrationConfig {
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
				cmd := r.command
				// If alembic.ini lives in a subdirectory (e.g. migrations/), add -c so
				// alembic can find it from the container's WORKDIR. Without -c, alembic
				// looks for alembic.ini in the current directory (usually /app).
				if r.tool == "alembic" {
					cmd = alembicCommandWithConfig(projectDir, cmd)
				}

				return &lock.MigrationConfig{
					Tool:         r.tool,
					Command:      cmd,
					Reversible:   false,
					DetectedFrom: f,
				}
			}
		}
	}
	return nil
}

// alembicCommandWithConfig inspects the project directory for alembic.ini and
// injects -c <path> when the ini file is not at the project root.
// Standard locations checked in priority order:
//   - <root>/alembic.ini  → no flag needed (alembic default)
//   - <root>/migrations/alembic.ini → -c migrations/alembic.ini
//   - <root>/db/alembic.ini         → -c db/alembic.ini
func alembicCommandWithConfig(projectDir, baseCmd string) string {
	// Root-level ini — alembic's default, no flag needed.
	if _, err := os.Stat(filepath.Join(projectDir, "alembic.ini")); err == nil {
		return baseCmd
	}
	// Common subdirectory locations.
	subPaths := []string{"migrations/alembic.ini", "db/alembic.ini", "alembic/alembic.ini"}
	for _, rel := range subPaths {
		if _, err := os.Stat(filepath.Join(projectDir, rel)); err == nil {
			// Replace bare "alembic" command with "alembic -c <path>"
			return strings.Replace(baseCmd, "alembic ", "alembic -c "+rel+" ", 1)
		}
	}
	// Not found — return the base command and let the user configure it.
	return baseCmd
}

var goMigRules = []migRule{
	{"pressly/goose", "goose", "goose -dir migrations up"},
	{"golang-migrate", "golang-migrate", "migrate -path migrations -database $DATABASE_URL up"},
	{"uptrace/bun", "bun", "bun db migrate"},
	{"go-gormigrate", "gormigrate", ""},
	{"gorm.io/gorm", "gorm", ""}, // gorm AutoMigrate — no CLI
}

func suggestGo(projectDir string) *lock.MigrationConfig {
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

func suggestNode(projectDir string) *lock.MigrationConfig {
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

func suggestRuby(projectDir string) *lock.MigrationConfig {
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

// remoteDeployDir returns the directory on the VPS where pilot places project files.
// Mirrors the logic in internal/adapters/vps/ssh.go:deployDir() — both must stay in sync.
func remoteDeployDir(cfg *config.Config, env string) string {
	if envCfg, ok := cfg.Environments[env]; ok {
		if targetName := envCfg.Target; targetName != "" {
			if target, ok := cfg.Targets[targetName]; ok && target.DeployPath != "" {
				return target.DeployPath
			}
		}
	}
	if cfg.Project.Name != "" {
		return "~/pilot/" + cfg.Project.Name
	}
	return "~/pilot"
}

// appServiceName returns the name of the first service of type "app" in the config.
// Returns an error when no service of type "app" is found — callers must handle this
// by either declaring migrations.service in pilot.yaml or adding a service with type: app.
func appServiceName(cfg *config.Config) (string, error) {
	for name, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp {
			return name, nil
		}
	}
	return "", fmt.Errorf(
		"no service of type %q found in pilot.yaml — cannot determine which container to run migrations in.\n"+
			"Fix: add 'service: <name>' under environments.<env>.migrations, or add 'type: app' to the relevant service",
		config.ServiceTypeApp,
	)
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
