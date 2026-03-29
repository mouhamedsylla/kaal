package providers

import "context"

// Provider abstracts deployment targets: VPS (SSH), AWS, GCP, Azure, DigitalOcean.
type Provider interface {
	// Deploy builds and deploys the application to the target environment.
	Deploy(ctx context.Context, env string, opts DeployOptions) error

	// Sync copies kaal.yaml and related config files to the remote target.
	Sync(ctx context.Context, target string) error

	// Status returns the runtime state of services on the remote target.
	Status(ctx context.Context, env string) ([]ServiceStatus, error)

	// Logs streams log lines from a remote service over SSH.
	Logs(ctx context.Context, env string, opts LogOptions) (<-chan string, error)

	// Rollback reverts to the previous (or specified) deployment version.
	// Returns the tag that was actually deployed (useful when version is auto-resolved).
	Rollback(ctx context.Context, env string, version string) (string, error)
}

// LogOptions controls log streaming behavior.
type LogOptions struct {
	Service string
	Follow  bool
	Since   string
	Lines   int
}

// DeployOptions controls deployment behavior.
type DeployOptions struct {
	Tag      string   // image tag to deploy
	Strategy string   // rolling, blue-green, canary
	DryRun   bool
	EnvFiles []string // local paths to .env files to transfer and pass to docker compose
}

// ServiceStatus represents the state of a remote service.
type ServiceStatus struct {
	Name    string
	State   string
	Health  string
	Version string
}
