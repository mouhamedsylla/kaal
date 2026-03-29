package orchestrator

import "context"

// Orchestrator abstracts Docker Compose, k3s, and Kubernetes.
// Each implementation handles the lifecycle of services for a given environment.
type Orchestrator interface {
	// Up starts all services (or a subset) for the given environment.
	Up(ctx context.Context, env string, services []string) error

	// Down stops and removes services for the given environment.
	Down(ctx context.Context, env string) error

	// Logs streams log output for a specific service.
	Logs(ctx context.Context, service string, opts LogOptions) (<-chan string, error)

	// Status returns the current state of all services.
	Status(ctx context.Context) ([]ServiceStatus, error)
}

// LogOptions controls log streaming behavior.
type LogOptions struct {
	Follow bool
	Since  string
	Lines  int
}

// ServiceStatus represents the runtime state of a single service.
type ServiceStatus struct {
	Name    string
	Image   string
	State   string // running, exited, restarting...
	Health  string // healthy, unhealthy, starting, none
	Ports   []string
	Created string
}
