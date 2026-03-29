package scaffold

// Options holds all inputs needed to scaffold a project.
type Options struct {
	Name            string
	Stack           string // go, node, python, rust
	LanguageVersion string // auto-set from stack defaults
	Registry        string // ghcr, dockerhub, custom
	RegistryImage   string // e.g. ghcr.io/user/name
	RegistryURL     string // for custom registries
	Environments    []string // e.g. ["dev", "staging", "prod"]
	HasDB           bool
	Port            int
	OutputDir       string // directory to scaffold into (default: ./<name>)
}

// stackDefaults maps stacks to sensible defaults.
var stackDefaults = map[string]struct {
	version    string
	port       int
	entryPoint string
}{
	"go":     {version: "1.23", port: 8080},
	"node":   {version: "20", port: 3000, entryPoint: "dist/index.js"},
	"python": {version: "3.12", port: 8000},
	"rust":   {version: "1.82", port: 8080},
}

func (o *Options) applyDefaults() {
	if d, ok := stackDefaults[o.Stack]; ok {
		if o.LanguageVersion == "" {
			o.LanguageVersion = d.version
		}
		if o.Port == 0 {
			o.Port = d.port
		}
	}
	if len(o.Environments) == 0 {
		o.Environments = []string{"dev", "staging", "prod"}
	}
	if o.OutputDir == "" {
		o.OutputDir = o.Name
	}
}
