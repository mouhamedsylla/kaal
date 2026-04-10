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

// ExecutionProvider abstracts the local runtime (Docker Compose, Podman, ...).
// Used by pilot up / down / status / logs.
type ExecutionProvider interface {
	Up(ctx context.Context, env string) error
	Down(ctx context.Context, env string) error
	Status(ctx context.Context, env string) ([]ServiceStatus, error)
	Logs(ctx context.Context, env string, service string) (<-chan string, error)
}

// DeployProvider abstracts the remote deployment target (VPS, AWS, ...).
// Used by pilot deploy / rollback / sync.
type DeployProvider interface {
	Sync(ctx context.Context, env string) error
	Deploy(ctx context.Context, env string, opts DeployOptions) error
	Rollback(ctx context.Context, env string, toTag string) (restoredTag string, err error)
	Status(ctx context.Context, env string) ([]ServiceStatus, error)
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
