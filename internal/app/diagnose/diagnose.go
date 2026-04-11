// Package diagnose implements the "pilot diagnose" snapshot.
//
// Six check categories, all best-effort (one failure never blocks others):
//
//	System   — Docker, Docker Compose, Go
//	Project  — pilot.yaml, .env files, Dockerfile, compose files
//	Ports    — declared ports free locally (active env only)
//	Registry — TCP reachability of the registry host
//	SSH      — key permissions + TCP reachability of VPS targets
//	Git      — branch, dirty state, last commit
package diagnose

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mouhamedsylla/pilot/internal/config"
)

// Check is one diagnostic line.
type Check struct {
	Category string // System | Project | Ports | Registry | SSH | Git
	Name     string
	OK       bool
	Value    string // observed value when OK
	Issue    string // reason when not OK
}

// Report is the full diagnose output.
type Report struct {
	Checks    []Check
	Suspended *SuspendedInfo // non-nil when .pilot/suspended.json exists
	ActiveEnv string
}

// SuspendedInfo is a brief summary of a pending TypeC suspension.
type SuspendedInfo struct {
	ErrorCode string
	Command   string
	Since     time.Time
}

type UseCase struct{}

func New() *UseCase { return &UseCase{} }

// Execute runs all checks and returns the report.
func (uc *UseCase) Execute(ctx context.Context, cfg *config.Config, activeEnv string) Report {
	r := Report{ActiveEnv: activeEnv}

	// ── 1. System ─────────────────────────────────────────────────────────────
	r.add(checkCLI("Docker", "docker", "--version"))
	r.add(checkCLI("Docker Compose", "docker", "compose", "version"))
	r.add(checkCLI("Go", "go", "version"))

	// ── 2. Project ────────────────────────────────────────────────────────────
	r.add(checkPilotYAML(cfg))
	r.add(checkDockerfile())
	for env, envCfg := range cfg.Environments {
		envFile := envCfg.EnvFile
		if envFile == "" {
			envFile = fmt.Sprintf(".env.%s", env)
		}
		r.add(checkEnvFile(env, envFile))
		r.add(checkComposeFile(env))
	}

	// ── 3. Ports (active local env only) ─────────────────────────────────────
	if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.Target == "" {
		composePath := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
		for svc, port := range parseComposePorts(composePath) {
			r.add(checkPort(svc, port))
		}
	}

	// ── 4. Registry ───────────────────────────────────────────────────────────
	if cfg.Registry.Provider != "" {
		r.add(checkRegistry(ctx, cfg.Registry))
	}

	// ── 5. SSH / VPS ─────────────────────────────────────────────────────────
	for env, envCfg := range cfg.Environments {
		if envCfg.Target == "" {
			continue
		}
		target, ok := cfg.Targets[envCfg.Target]
		if !ok {
			continue
		}
		r.add(checkSSHKey(env, target))
		r.add(checkSSHReachability(ctx, env, target))
	}

	// ── 6. Git ────────────────────────────────────────────────────────────────
	for _, c := range checkGit() {
		r.add(c)
	}

	// ── 7. Suspended state ────────────────────────────────────────────────────
	r.Suspended = loadSuspension()

	return r
}

func (r *Report) add(c Check) { r.Checks = append(r.Checks, c) }

// ── check implementations ─────────────────────────────────────────────────────

func checkCLI(name, bin string, args ...string) Check {
	c := Check{Category: "System", Name: name}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		c.Issue = "not found or not running"
		return c
	}
	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	if len(line) > 60 {
		line = line[:60]
	}
	c.OK = true
	c.Value = line
	return c
}

func checkPilotYAML(cfg *config.Config) Check {
	c := Check{Category: "Project", Name: "pilot.yaml"}
	if cfg == nil {
		c.Issue = "not loaded"
		return c
	}
	c.OK = true
	c.Value = fmt.Sprintf("project=%s  stack=%s", cfg.Project.Name, cfg.Project.Stack)
	return c
}

func checkDockerfile() Check {
	c := Check{Category: "Project", Name: "Dockerfile"}
	for _, f := range []string{"Dockerfile", "dockerfile", "Dockerfile.prod"} {
		if _, err := os.Stat(f); err == nil {
			c.OK = true
			c.Value = f
			return c
		}
	}
	c.Issue = "not found (required for pilot push)"
	return c
}

func checkEnvFile(env, path string) Check {
	c := Check{Category: "Project", Name: fmt.Sprintf(".env.%s", env)}
	vars, err := parseEnvFile(path)
	if err != nil {
		c.Issue = fmt.Sprintf("not found (%s)", path)
		return c
	}
	empty := 0
	for _, v := range vars {
		if v == "" {
			empty++
		}
	}
	c.OK = empty == 0
	c.Value = fmt.Sprintf("%d vars", len(vars))
	if empty > 0 {
		c.Issue = fmt.Sprintf("%d empty value(s)", empty)
	}
	return c
}

func checkComposeFile(env string) Check {
	path := fmt.Sprintf("docker-compose.%s.yml", env)
	c := Check{Category: "Project", Name: path}
	if _, err := os.Stat(path); err != nil {
		c.Issue = "not found"
		return c
	}
	c.OK = true
	return c
}

func checkPort(service, port string) Check {
	c := Check{Category: "Ports", Name: fmt.Sprintf("%s (:%s)", service, port)}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		c.Issue = fmt.Sprintf("OCCUPIED — %s", portOwner(port))
		return c
	}
	_ = ln.Close()
	c.OK = true
	c.Value = "free"
	return c
}

func checkRegistry(ctx context.Context, reg config.RegistryConfig) Check {
	c := Check{Category: "Registry", Name: reg.Provider}
	host := registryHost(reg)
	if host == "" {
		c.OK = true
		c.Value = "custom (no TCP check)"
		return c
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	t := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", host+":443")
	if err != nil {
		c.Issue = fmt.Sprintf("unreachable: %v", err)
		return c
	}
	conn.Close()
	c.OK = true
	c.Value = fmt.Sprintf("reachable (%dms)", time.Since(t).Milliseconds())
	return c
}

func checkSSHKey(env string, target config.Target) Check {
	c := Check{Category: "SSH", Name: fmt.Sprintf("%s — key", env)}
	keyPath := target.Key
	if keyPath == "" {
		c.Issue = "no key configured"
		return c
	}
	if strings.HasPrefix(keyPath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			keyPath = filepath.Join(home, keyPath[2:])
		}
	}
	info, err := os.Stat(keyPath)
	if err != nil {
		c.Issue = fmt.Sprintf("not found: %s", keyPath)
		return c
	}
	perm := info.Mode().Perm()
	if perm > 0600 {
		c.Issue = fmt.Sprintf("permissions %04o (run: chmod 600 %s)", perm, keyPath)
		return c
	}
	c.OK = true
	c.Value = fmt.Sprintf("%s (%04o)", filepath.Base(keyPath), perm)
	return c
}

func checkSSHReachability(ctx context.Context, env string, target config.Target) Check {
	port := target.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", target.Host, port)
	c := Check{Category: "SSH", Name: fmt.Sprintf("%s — %s", env, addr)}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	t := time.Now()
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	if err != nil {
		c.Issue = fmt.Sprintf("unreachable: %v", err)
		return c
	}
	conn.Close()
	c.OK = true
	c.Value = fmt.Sprintf("reachable (%dms)", time.Since(t).Milliseconds())
	return c
}

func checkGit() []Check {
	var checks []Check

	branchC := Check{Category: "Git", Name: "branch"}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branchC.OK = true
		branchC.Value = strings.TrimSpace(string(out))
	} else {
		branchC.Issue = "not a git repository"
	}
	checks = append(checks, branchC)

	dirtyC := Check{Category: "Git", Name: "working tree"}
	if out, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		n := 0
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l != "" {
				n++
			}
		}
		if n == 0 {
			dirtyC.OK = true
			dirtyC.Value = "clean"
		} else {
			dirtyC.Issue = fmt.Sprintf("dirty (%d file(s) modified)", n)
		}
	} else {
		dirtyC.Issue = "not a git repository"
	}
	checks = append(checks, dirtyC)

	commitC := Check{Category: "Git", Name: "last commit"}
	if out, err := exec.Command("git", "log", "-1", "--format=%h %s").Output(); err == nil {
		commitC.OK = true
		commitC.Value = strings.TrimSpace(string(out))
	} else {
		commitC.Issue = "no commits yet"
	}
	checks = append(checks, commitC)

	return checks
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	vars := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		vars[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
	}
	return vars, scanner.Err()
}

// parseComposePorts reads published ports from a compose file (service → host port).
func parseComposePorts(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var compose struct {
		Services map[string]struct {
			Ports []json.RawMessage `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil
	}
	result := map[string]string{}
	for name, svc := range compose.Services {
		for _, raw := range svc.Ports {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				// "8080:80" or "8080"
				result[name] = strings.SplitN(s, ":", 2)[0]
				break
			}
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err == nil {
				if pub, ok := m["published"]; ok {
					result[name] = fmt.Sprintf("%v", pub)
					break
				}
			}
		}
	}
	return result
}

func portOwner(port string) string {
	out, err := exec.Command("lsof", "-i", ":"+port, "-sTCP:LISTEN", "-n", "-P").Output()
	if err != nil {
		return "unknown process"
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "unknown process"
	}
	fields := strings.Fields(lines[1])
	if len(fields) >= 2 {
		return fmt.Sprintf("%s (pid %s)", fields[0], fields[1])
	}
	return "unknown process"
}

func registryHost(reg config.RegistryConfig) string {
	switch reg.Provider {
	case "ghcr":
		return "ghcr.io"
	case "dockerhub":
		return "registry-1.docker.io"
	case "gcr":
		return "gcr.io"
	case "ecr":
		return "amazonaws.com"
	case "acr":
		return "azurecr.io"
	default:
		return ""
	}
}

func loadSuspension() *SuspendedInfo {
	data, err := os.ReadFile(".pilot/suspended.json")
	if err != nil {
		return nil
	}
	var s struct {
		ErrorCode   string    `json:"error_code"`
		Command     string    `json:"command"`
		SuspendedAt time.Time `json:"suspended_at"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &SuspendedInfo{ErrorCode: s.ErrorCode, Command: s.Command, Since: s.SuspendedAt}
}
