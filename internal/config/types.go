package config

// Config is the in-memory representation of kaal.yaml.
type Config struct {
	APIVersion   string                 `yaml:"apiVersion"`
	Project      Project                `yaml:"project"`
	Registry     RegistryConfig         `yaml:"registry"`
	Environments map[string]Environment `yaml:"environments"`
	Targets      map[string]Target      `yaml:"targets"`
}

type Project struct {
	Name            string `yaml:"name"`
	Stack           string `yaml:"stack"`           // go, node, python, rust...
	LanguageVersion string `yaml:"language_version"` // "1.23", "20", ...
}

type RegistryConfig struct {
	Provider string `yaml:"provider"` // ghcr, dockerhub, ecr, gcr, acr, custom
	Image    string `yaml:"image"`    // full image name e.g. ghcr.io/user/app
	URL      string `yaml:"url"`      // for custom registries
}

type Environment struct {
	ComposeFile string         `yaml:"compose_file"`
	EnvFile     string         `yaml:"env_file"`
	Ports       map[string]int `yaml:"ports"`
	Target      string         `yaml:"target"`      // reference to Targets map key
	Orchestrator string        `yaml:"orchestrator"` // compose, k8s
	Namespace   string         `yaml:"namespace"`   // for k8s
	Secrets     SecretsRef     `yaml:"secrets"`
}

type SecretsRef struct {
	Provider string            `yaml:"provider"` // local, aws_sm, gcp_sm, azure_kv
	Refs     map[string]string `yaml:"refs"`     // ENV_VAR: secret/path
}

type Target struct {
	Type         string `yaml:"type"`          // vps, aws, gcp, azure, do
	Host         string `yaml:"host"`          // VPS IP or hostname
	User         string `yaml:"user"`          // SSH user
	Key          string `yaml:"key"`           // SSH key path
	Port         int    `yaml:"port"`          // SSH port (default 22)
	Project      string `yaml:"project"`       // GCP project, AWS account alias
	Region       string `yaml:"region"`        // cloud region
	Cluster      string `yaml:"cluster"`       // k8s cluster name
	Orchestrator string `yaml:"orchestrator"`  // compose, k8s
}
