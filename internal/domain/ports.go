// Package domain defines the ports that the domain requires from the outside world.
//
// Dependency rule: adapters/ implement these interfaces — domain/ never imports adapters/.
// The direction of dependency always points inward, toward domain/.
package domain

import (
	"context"
)

// ServiceStatus describes the runtime state of a single container service.
type ServiceStatus struct {
	Name   string // service name as declared in compose
	State  string // running | exited | restarting | ...
	Health string // healthy | unhealthy | starting | "" (no healthcheck)
}

// DeployOptions controls a deploy operation.
type DeployOptions struct {
	Tag      string
	EnvFiles []string // extra env files to inject (e.g. resolved secrets)
}

// BuildOptions controls a docker build operation.
type BuildOptions struct {
	Tag        string
	Dockerfile string
	Context    string
	Platforms  []string
	BuildArgs  map[string]string
	NoCache    bool
}

// LogOptions controls log streaming behaviour.
type LogOptions struct {
	Follow bool
	Since  string
	Lines  int
}

// ExecutionProvider abstracts the local runtime (Docker Compose, Podman, ...).
// Used by pilot up / down / status / logs.
type ExecutionProvider interface {
	Up(ctx context.Context, env string, services []string) error
	Down(ctx context.Context, env string) error
	Status(ctx context.Context, env string) ([]ServiceStatus, error)
	Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}

// DeployProvider abstracts the remote deployment target (VPS, AWS, ...).
// Used by pilot deploy / rollback / sync / status / logs.
type DeployProvider interface {
	Sync(ctx context.Context, env string) error
	Deploy(ctx context.Context, env string, opts DeployOptions) error
	Rollback(ctx context.Context, env string, toTag string) (restoredTag string, err error)
	Status(ctx context.Context, env string) ([]ServiceStatus, error)
	Logs(ctx context.Context, env string, service string, opts LogOptions) (<-chan string, error)
}

// RegistryProvider abstracts image build and push (GHCR, Docker Hub, ECR, ...).
// Used by pilot push.
type RegistryProvider interface {
	Login(ctx context.Context) error
	Build(ctx context.Context, opts BuildOptions) error
	Push(ctx context.Context, tag string) error
}

// SecretManager abstracts secret resolution (local .env, AWS SM, GCP SM, ...).
// Used by pilot deploy when secrets.refs is declared in pilot.yaml.
type SecretManager interface {
	Inject(ctx context.Context, env string, refs map[string]string) (map[string]string, error)
}

// MigrationConfig describes how to run and (optionally) roll back database migrations.
// This mirrors config.Migrations and lock.MigrationConfig — kept here so domain
// packages never import config/ or domain/lock.
type MigrationConfig struct {
	Tool            string // prisma | alembic | goose | flyway | sql-migrate
	Command         string // e.g. "npx prisma migrate deploy"
	RollbackCommand string // only used when Reversible is true
	Reversible      bool
}

// HookRunner executes shell commands on the deployment target (remote or local).
// Used by pilot deploy to run pre/post-deploy hooks.
type HookRunner interface {
	RunHooks(ctx context.Context, commands []string) error
}

// MigrationRunner applies and optionally rolls back database schema changes.
// The command runs on the remote target (via SSH for VPS targets).
type MigrationRunner interface {
	RunMigrations(ctx context.Context, cfg MigrationConfig) error
	RollbackMigrations(ctx context.Context, cfg MigrationConfig) error
}
