package config

// Config is the in-memory representation of pilot.yaml.
type Config struct {
	APIVersion   string                 `yaml:"apiVersion"`
	Project      Project                `yaml:"project"`
	Services     map[string]Service     `yaml:"services"`
	Environments map[string]Environment `yaml:"environments"`
	Targets      map[string]Target      `yaml:"targets"`
	Registry     RegistryConfig         `yaml:"registry"`
}

// Project holds project-level metadata.
type Project struct {
	Name            string `yaml:"name"`
	Stack           string `yaml:"stack"`            // detected or declared: go, node, python...
	LanguageVersion string `yaml:"language_version"` // "1.23", "20", ...
}

// Service describes one logical service in the application topology.
// pilot does not generate Dockerfiles at init — it generates them at runtime (pilot up).
type Service struct {
	Type       string `yaml:"type"`                  // app | postgres | mysql | mongodb | redis | rabbitmq | nats | nginx | custom
	Port       int    `yaml:"port,omitempty"`        // exposed port (app services)
	Version    string `yaml:"version,omitempty"`     // image version for managed services (postgres:16, redis:7...)
	Dockerfile string `yaml:"dockerfile,omitempty"`  // path to existing Dockerfile (optional, app services only)
	Image      string `yaml:"image,omitempty"`       // custom image (optional override)
}

// Environment describes how a set of services runs in a specific context.
type Environment struct {
	Runtime    string      `yaml:"runtime,omitempty"`    // compose | lima | k3d
	EnvFile    string      `yaml:"env_file,omitempty"`
	Target     string      `yaml:"target,omitempty"`     // reference to Targets key (non-dev envs)
	Resources  *Resources  `yaml:"resources,omitempty"`  // local resource constraints mirroring prod
	Secrets    *SecretsRef `yaml:"secrets,omitempty"`
	Hooks      *Hooks      `yaml:"hooks,omitempty"`
	Migrations *Migrations `yaml:"migrations,omitempty"`
}

// Hooks declares shell commands to run before and after a deploy.
// Commands run on the remote target via SSH (for remote envs) or locally (for local envs).
type Hooks struct {
	PreDeploy  []HookCommand `yaml:"pre_deploy,omitempty"`
	PostDeploy []HookCommand `yaml:"post_deploy,omitempty"`
}

// HookCommand is one hook step.
type HookCommand struct {
	Command     string `yaml:"command"`
	Description string `yaml:"description,omitempty"`
}

// Migrations describes how to apply and (optionally) roll back database schema changes.
// If nil, pilot auto-detects from well-known project files (prisma, alembic, goose, flyway).
type Migrations struct {
	Tool            string `yaml:"tool"`                       // prisma | alembic | goose | flyway | sql-migrate
	Command         string `yaml:"command"`                    // command to apply migrations
	RollbackCommand string `yaml:"rollback_command,omitempty"` // required when reversible: true
	Reversible      bool   `yaml:"reversible"`                 // false by default
}

// Resources describes compute constraints — used to mirror production locally.
type Resources struct {
	CPUs   string `yaml:"cpus,omitempty"`   // e.g. "2", "0.5"
	Memory string `yaml:"memory,omitempty"` // e.g. "4GB", "512MB"
}

// SecretsRef points to a secret manager backend.
type SecretsRef struct {
	Provider string            `yaml:"provider,omitempty"` // local | aws_sm | gcp_sm | azure_kv
	Refs     map[string]string `yaml:"refs,omitempty"`     // ENV_VAR: secret/path
}

// Target describes a remote deployment destination.
type Target struct {
	Type      string     `yaml:"type"`                  // vps | aws | gcp | azure | do | hetzner
	Host      string     `yaml:"host"`                  // VPS IP or hostname
	User      string     `yaml:"user,omitempty"`        // SSH user
	Key       string     `yaml:"key,omitempty"`         // SSH key path (~/.ssh/id_pilot)
	Port      int        `yaml:"port,omitempty"`        // SSH port (default 22)
	Resources *Resources `yaml:"resources,omitempty"`  // actual prod resources (for local simulation)
	// Cloud-specific fields
	Project string `yaml:"project,omitempty"` // GCP project, AWS account alias
	Region  string `yaml:"region,omitempty"`  // cloud region
	Cluster string `yaml:"cluster,omitempty"` // k8s cluster name
}

// RegistryConfig describes where Docker images are pushed and pulled.
type RegistryConfig struct {
	Provider  string   `yaml:"provider,omitempty"`   // ghcr | dockerhub | ecr | gcr | acr | custom
	Image     string   `yaml:"image,omitempty"`      // full image name e.g. ghcr.io/user/app
	URL       string   `yaml:"url,omitempty"`        // for custom registries
	BuildArgs []string `yaml:"build_args,omitempty"` // env var names to inject at build time (e.g. VITE_APP_ENV)
}

// ServiceType constants.
const (
	ServiceTypeApp       = "app"
	ServiceTypePostgres  = "postgres"
	ServiceTypeMySQL     = "mysql"
	ServiceTypeMongoDB   = "mongodb"
	ServiceTypeRedis     = "redis"
	ServiceTypeRabbitMQ  = "rabbitmq"
	ServiceTypeNATS      = "nats"
	ServiceTypeNginx     = "nginx"
	ServiceTypeCustom    = "custom"
)

// RuntimeType constants.
const (
	RuntimeCompose = "compose"
	RuntimeLima    = "lima"
	RuntimeK3d     = "k3d"
)
