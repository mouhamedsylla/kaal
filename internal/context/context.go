// Package context collects the full project context.
// This is used by kaal up (when files are missing) and by the MCP server
// so that AI agents have everything they need to generate infrastructure files.
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/scaffold"
	"gopkg.in/yaml.v3"
)

// ProjectContext is the complete picture of a project at a given moment.
// It is serializable to JSON for the MCP response and printable for humans.
type ProjectContext struct {
	// From kaal.yaml
	KaalYAML string `json:"kaal_yaml"`

	// Detected project info
	Stack           string `json:"stack"`
	LanguageVersion string `json:"language_version"`
	IsExistingProject bool `json:"is_existing_project"`

	// File structure
	FileTree    string   `json:"file_tree"`
	KeyFiles    []string `json:"key_files"`   // files relevant to infra generation

	// Existing infra files
	ExistingDockerfiles []string `json:"existing_dockerfiles"`
	ExistingComposeFiles []string `json:"existing_compose_files"`
	ExistingEnvFiles    []string `json:"existing_env_files"`

	// What's missing (populated by kaal up)
	MissingDockerfile bool   `json:"missing_dockerfile"`
	MissingCompose    bool   `json:"missing_compose"`
	ActiveEnv         string `json:"active_env"`

	// The parsed config (for structured access)
	Config *config.Config `json:"config"`
}

// Collect gathers the full project context from the current directory.
func Collect(activeEnv string) (*ProjectContext, error) {
	cfg, err := config.Load(".")
	if err != nil {
		return nil, err
	}

	detected := scaffold.Detect(".")

	ctx := &ProjectContext{
		Stack:             cfg.Project.Stack,
		LanguageVersion:   cfg.Project.LanguageVersion,
		IsExistingProject: detected.IsExisting,
		ActiveEnv:         activeEnv,
		Config:            cfg,
	}

	if ctx.Stack == "" {
		ctx.Stack = detected.Stack
	}
	if ctx.LanguageVersion == "" {
		ctx.LanguageVersion = detected.LanguageVersion
	}

	// Read kaal.yaml as raw string
	raw, err := os.ReadFile(config.FileName)
	if err != nil {
		return nil, err
	}
	ctx.KaalYAML = string(raw)

	// File tree (max 3 levels deep, skip common noise)
	ctx.FileTree = buildFileTree(".", 0, 3)

	// Scan for relevant files
	ctx.KeyFiles = scanKeyFiles(".")
	ctx.ExistingDockerfiles = glob(".", "Dockerfile*")
	ctx.ExistingComposeFiles = glob(".", "docker-compose*.yml")
	ctx.ExistingEnvFiles = globEnvFiles(".")

	// Determine what's missing for the active env
	composeFile := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
	ctx.MissingDockerfile = !fileExists("Dockerfile") && !hasCustomDockerfile(cfg)
	ctx.MissingCompose = !fileExists(composeFile)

	return ctx, nil
}

// Summary returns a human-readable summary of the context.
func (c *ProjectContext) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Project:  %s\n", c.Config.Project.Name))
	b.WriteString(fmt.Sprintf("Stack:    %s %s\n", c.Stack, c.LanguageVersion))
	b.WriteString(fmt.Sprintf("Env:      %s\n", c.ActiveEnv))

	b.WriteString("\nServices:\n")
	for name, svc := range c.Config.Services {
		if svc.Port > 0 {
			b.WriteString(fmt.Sprintf("  %-12s type=%-10s port=%d\n", name, svc.Type, svc.Port))
		} else {
			b.WriteString(fmt.Sprintf("  %-12s type=%s\n", name, svc.Type))
		}
	}

	if len(c.ExistingDockerfiles) > 0 {
		b.WriteString(fmt.Sprintf("\nDockerfiles: %s\n", strings.Join(c.ExistingDockerfiles, ", ")))
	}
	if len(c.ExistingComposeFiles) > 0 {
		b.WriteString(fmt.Sprintf("Compose:     %s\n", strings.Join(c.ExistingComposeFiles, ", ")))
	}

	return b.String()
}

// AgentPrompt returns a ready-to-use prompt for an AI agent to generate missing files.
func (c *ProjectContext) AgentPrompt() string {
	var b strings.Builder

	b.WriteString("Here is the full context of this kaal project.\n\n")

	b.WriteString("## kaal.yaml\n\n```yaml\n")
	b.WriteString(c.KaalYAML)
	b.WriteString("```\n\n")

	b.WriteString("## Project structure\n\n```\n")
	b.WriteString(c.FileTree)
	b.WriteString("```\n\n")

	if len(c.KeyFiles) > 0 {
		b.WriteString("## Key files detected\n\n")
		for _, f := range c.KeyFiles {
			b.WriteString(fmt.Sprintf("- %s\n", f))
		}
		b.WriteString("\n")
	}

	if len(c.ExistingDockerfiles) > 0 {
		b.WriteString("## Existing Dockerfiles\n\n")
		for _, f := range c.ExistingDockerfiles {
			content, err := os.ReadFile(f)
			if err == nil {
				b.WriteString(fmt.Sprintf("### %s\n\n```dockerfile\n%s```\n\n", f, string(content)))
			}
		}
	}

	b.WriteString("## Stack\n\n")
	b.WriteString(fmt.Sprintf("- Language: %s %s\n", c.Stack, c.LanguageVersion))
	b.WriteString(fmt.Sprintf("- Active environment: %s\n", c.ActiveEnv))
	b.WriteString("\n")

	b.WriteString("## Services defined in kaal.yaml\n\n")
	// Print services as YAML for clarity
	data, _ := yaml.Marshal(c.Config.Services)
	b.WriteString("```yaml\n")
	b.WriteString(string(data))
	b.WriteString("```\n\n")

	if c.MissingDockerfile {
		b.WriteString("## What is needed\n\n")
		b.WriteString("- **Dockerfile** is missing. ")
		b.WriteString("Please generate a production-ready Dockerfile for this project.\n")
	}
	if c.MissingCompose {
		b.WriteString(fmt.Sprintf("- **docker-compose.%s.yml** is missing. ", c.ActiveEnv))
		b.WriteString("Please generate a docker-compose file for this environment,\n")
		b.WriteString("  using the services and resources defined in kaal.yaml.\n")
	}

	// Warn the agent about unconfigured deploy targets.
	var unconfiguredTargets []string
	for name, t := range c.Config.Targets {
		if t.Host == "" {
			unconfiguredTargets = append(unconfiguredTargets, name)
		}
	}
	if len(unconfiguredTargets) > 0 {
		b.WriteString("\n## ⚠ Unconfigured deploy targets\n\n")
		b.WriteString("The following targets have no `host` set in kaal.yaml.\n")
		b.WriteString("`kaal deploy` will fail until these are filled in:\n\n")
		for _, name := range unconfiguredTargets {
			b.WriteString(fmt.Sprintf("- **%s** — set `targets.%s.host` to the VPS IP or hostname\n", name, name))
		}
		b.WriteString("\nAsk the user for the VPS IP, then update kaal.yaml or run `kaal setup --env <env>`.\n")
	}

	return b.String()
}

// ──────────────────────── helpers ────────────────────────

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true,
	".cache": true, "dist": true, "build": true, "__pycache__": true,
	".kaal-current-env": true,
}

func buildFileTree(dir string, depth, maxDepth int) string {
	if depth > maxDepth {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var lines []string
	prefix := strings.Repeat("  ", depth)

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && depth == 0 && name != ".env.example" {
			continue // skip hidden at root except .env.example
		}
		if skipDirs[name] {
			continue
		}
		if e.IsDir() {
			lines = append(lines, prefix+name+"/")
			sub := buildFileTree(filepath.Join(dir, name), depth+1, maxDepth)
			if sub != "" {
				lines = append(lines, sub)
			}
		} else {
			lines = append(lines, prefix+name)
		}
	}
	return strings.Join(lines, "\n")
}

func scanKeyFiles(dir string) []string {
	candidates := []string{
		"go.mod", "go.sum", "package.json", "package-lock.json",
		"Cargo.toml", "requirements.txt", "pyproject.toml",
		"pom.xml", "build.gradle", "Makefile",
	}
	var found []string
	for _, f := range candidates {
		if fileExists(filepath.Join(dir, f)) {
			found = append(found, f)
		}
	}
	sort.Strings(found)
	return found
}

func glob(dir, pattern string) []string {
	matches, _ := filepath.Glob(filepath.Join(dir, pattern))
	var results []string
	for _, m := range matches {
		results = append(results, filepath.Base(m))
	}
	return results
}

func globEnvFiles(dir string) []string {
	var files []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".env") {
			files = append(files, name)
		}
	}
	return files
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasCustomDockerfile(cfg *config.Config) bool {
	for _, svc := range cfg.Services {
		if svc.Type == config.ServiceTypeApp && svc.Dockerfile != "" {
			return fileExists(svc.Dockerfile)
		}
	}
	return false
}
