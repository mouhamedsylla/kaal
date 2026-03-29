// Package composer generates docker-compose files and Dockerfiles
// from the service definitions in kaal.yaml.
// It is invoked by kaal up when the required files are absent.
package composer

import (
	"bytes"
	"fmt"
	"os"
	"text/template"

	"github.com/mouhamedsylla/kaal/internal/config"
)

// ComposeOptions controls compose file generation.
type ComposeOptions struct {
	Env     string
	EnvFile string
	IsDev   bool // dev envs get volume mounts for hot reload
}

// GenerateCompose writes docker-compose.<env>.yml from cfg services.
// Returns the path written.
func GenerateCompose(cfg *config.Config, opts ComposeOptions) (string, error) {
	dest := fmt.Sprintf("docker-compose.%s.yml", opts.Env)

	var buf bytes.Buffer
	if err := composeTmpl.Execute(&buf, buildComposeData(cfg, opts)); err != nil {
		return "", fmt.Errorf("render compose: %w", err)
	}

	if err := os.WriteFile(dest, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", dest, err)
	}
	return dest, nil
}

// ──────────────────────── template data ────────────────────────

type composeData struct {
	ProjectName string
	EnvFile     string
	IsDev       bool
	Services    []serviceData
	HasVolumes  bool
	Resources   *config.Resources
}

type serviceData struct {
	Name        string
	Type        string
	Image       string
	Build       bool   // true for app services without a custom image
	Port        int
	InternalPort int
	EnvVars     map[string]string
	HealthCmd   string
	VolumeName  string // non-empty for stateful services
	CPUs        string
	Memory      string
}

func buildComposeData(cfg *config.Config, opts ComposeOptions) composeData {
	d := composeData{
		ProjectName: cfg.Project.Name + "-" + opts.Env,
		EnvFile:     opts.EnvFile,
		IsDev:       opts.IsDev,
	}

	envCfg := cfg.Environments[opts.Env]
	if envCfg.Resources != nil {
		d.Resources = envCfg.Resources
	}

	for name, svc := range cfg.Services {
		sd := buildServiceData(name, svc, opts)
		if sd.VolumeName != "" {
			d.HasVolumes = true
		}
		d.Services = append(d.Services, sd)
	}
	return d
}

func buildServiceData(name string, svc config.Service, opts ComposeOptions) serviceData {
	sd := serviceData{
		Name: name,
		Type: svc.Type,
	}

	switch svc.Type {
	case config.ServiceTypeApp:
		sd.Build = svc.Image == "" && svc.Dockerfile == ""
		sd.Image = svc.Image
		sd.Port = svc.Port
		sd.InternalPort = svc.Port
		sd.HealthCmd = fmt.Sprintf("wget -qO- http://localhost:%d/health || exit 1", svc.Port)

	case config.ServiceTypePostgres:
		v := svc.Version
		if v == "" {
			v = "16"
		}
		sd.Image = fmt.Sprintf("postgres:%s-alpine", v)
		sd.Port = 5432
		sd.InternalPort = 5432
		sd.VolumeName = name + "_data"
		sd.EnvVars = map[string]string{
			"POSTGRES_DB":       "${DB_NAME:-" + opts.Env + "_db}",
			"POSTGRES_USER":     "${DB_USER:-postgres}",
			"POSTGRES_PASSWORD": "${DB_PASSWORD:-postgres}",
		}
		sd.HealthCmd = "pg_isready -U postgres"

	case config.ServiceTypeMySQL:
		v := svc.Version
		if v == "" {
			v = "8"
		}
		sd.Image = fmt.Sprintf("mysql:%s", v)
		sd.Port = 3306
		sd.InternalPort = 3306
		sd.VolumeName = name + "_data"
		sd.EnvVars = map[string]string{
			"MYSQL_DATABASE":      "${DB_NAME:-" + opts.Env + "_db}",
			"MYSQL_USER":          "${DB_USER:-mysql}",
			"MYSQL_PASSWORD":      "${DB_PASSWORD:-mysql}",
			"MYSQL_ROOT_PASSWORD": "${DB_ROOT_PASSWORD:-root}",
		}
		sd.HealthCmd = "mysqladmin ping -h localhost"

	case config.ServiceTypeMongoDB:
		v := svc.Version
		if v == "" {
			v = "7"
		}
		sd.Image = fmt.Sprintf("mongo:%s", v)
		sd.Port = 27017
		sd.InternalPort = 27017
		sd.VolumeName = name + "_data"
		sd.HealthCmd = "mongosh --eval 'db.runCommand({ ping: 1 })'"

	case config.ServiceTypeRedis:
		v := svc.Version
		if v == "" {
			v = "7"
		}
		sd.Image = fmt.Sprintf("redis:%s-alpine", v)
		sd.Port = 6379
		sd.InternalPort = 6379
		sd.HealthCmd = "redis-cli ping"

	case config.ServiceTypeRabbitMQ:
		v := svc.Version
		if v == "" {
			v = "3"
		}
		sd.Image = fmt.Sprintf("rabbitmq:%s-management-alpine", v)
		sd.Port = 5672
		sd.InternalPort = 5672
		sd.VolumeName = name + "_data"
		sd.HealthCmd = "rabbitmq-diagnostics ping"

	case config.ServiceTypeNATS:
		sd.Image = "nats:alpine"
		sd.Port = 4222
		sd.InternalPort = 4222
		sd.HealthCmd = "nats-server --help"

	case config.ServiceTypeNginx:
		sd.Image = "nginx:alpine"
		sd.Port = 80
		sd.InternalPort = 80
		sd.HealthCmd = "wget -qO- http://localhost/health || exit 1"

	default:
		if svc.Image != "" {
			sd.Image = svc.Image
		}
		sd.Port = svc.Port
		sd.InternalPort = svc.Port
	}

	return sd
}

// ──────────────────────── template ────────────────────────

var composeFuncMap = template.FuncMap{
	"upper": func(s string) string {
		result := make([]byte, len(s))
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c >= 'a' && c <= 'z' {
				result[i] = c - 32
			} else {
				result[i] = c
			}
		}
		return string(result)
	},
	"volumePath": func(svcType string) string {
		paths := map[string]string{
			config.ServiceTypePostgres: "var/lib/postgresql/data",
			config.ServiceTypeMySQL:    "var/lib/mysql",
			config.ServiceTypeMongoDB:  "data/db",
			config.ServiceTypeRedis:    "data",
			config.ServiceTypeRabbitMQ: "var/lib/rabbitmq",
		}
		if p, ok := paths[svcType]; ok {
			return p
		}
		return "data"
	},
}

var composeTmpl = template.Must(template.New("compose").Funcs(composeFuncMap).Parse(`# Generated by kaal up — edit if needed, or delete to regenerate.
# Source of truth: kaal.yaml

name: {{.ProjectName}}

services:
{{- range .Services}}
  {{.Name}}:
{{- if .Build}}
    build:
      context: .
{{- else if .Image}}
    image: {{.Image}}
{{- end}}
{{- if .Port}}
    ports:
      - "${PORT:-{{.Port}}}:{{.InternalPort}}"
{{- end}}
{{- if .EnvVars}}
    environment:
{{- range $k, $v := .EnvVars}}
      {{$k}}: {{$v}}
{{- end}}
{{- end}}
{{- if and $.EnvFile (eq .Type "app")}}
    env_file:
      - {{$.EnvFile}}
{{- end}}
{{- if and $.IsDev (eq .Type "app")}}
    volumes:
      - .:/app
{{- end}}
    restart: unless-stopped
{{- if .VolumeName}}
    volumes:
      - {{.VolumeName}}:/{{volumePath .Type}}
{{- end}}
{{- if .HealthCmd}}
    healthcheck:
      test: ["CMD-SHELL", "{{.HealthCmd}}"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 10s
{{- end}}
{{- if $.Resources}}
    deploy:
      resources:
        limits:
{{- if $.Resources.CPUs}}
          cpus: "{{$.Resources.CPUs}}"
{{- end}}
{{- if $.Resources.Memory}}
          memory: {{$.Resources.Memory}}
{{- end}}
{{- end}}
{{end}}
{{- if .HasVolumes}}
volumes:
{{- range .Services}}
{{- if .VolumeName}}
  {{.VolumeName}}:
{{- end}}
{{- end}}
{{- end}}
`))
