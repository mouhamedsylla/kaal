// Package preflight verifies all prerequisites before push / deploy and
// generates pilot.lock when all checks pass.
//
// PreflightUseCase accepts a loaded *config.Config (cmd/ loads it) and
// two injectable checkers for system I/O — making all logic unit-testable
// without Docker or a real SSH server.
package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/config"
)

// ── ports ─────────────────────────────────────────────────────────────────────

// DockerChecker checks whether the Docker daemon is reachable.
type DockerChecker interface {
	IsRunning(ctx context.Context) error
}

// SSHChecker performs remote checks over SSH without exposing a full client.
type SSHChecker interface {
	CheckConnectivity(ctx context.Context, host, user, keyPath string, port int) error
	HasDockerAccess(ctx context.Context, host, user, keyPath string, port int) (bool, error)
	FileExists(ctx context.Context, host, user, keyPath string, port int, remotePath string) (bool, error)
}

// ── types ─────────────────────────────────────────────────────────────────────

// Target is the goal of a preflight run — determines which checks are active.
type Target string

const (
	TargetUp     Target = "up"
	TargetPush   Target = "push"
	TargetDeploy Target = "deploy"
)

// Status of a single check.
type Status string

const (
	StatusOK      Status = "ok"
	StatusWarning Status = "warning"
	StatusError   Status = "error"
	StatusSkipped Status = "skipped"
)

// FixType tells the agent (or human) who needs to act — maps to error taxonomy.
type FixType string

const (
	FixNone           FixType = ""
	FixAgent          FixType = "agent"            // TypeA/B — pilot or agent can fix
	FixHuman          FixType = "human"             // TypeD — human must act
	FixHumanThenAgent FixType = "human_then_agent"  // TypeD then TypeA/B
)

// Check is the result of one verification step.
type Check struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Status           Status  `json:"status"`
	Message          string  `json:"message"`
	FixType          FixType `json:"fix_type,omitempty"`
	AgentTool        string  `json:"agent_tool,omitempty"`
	HumanInstruction string  `json:"human_instruction,omitempty"`
}

// Report is the full preflight result, structured for both humans and agents.
type Report struct {
	Target       Target   `json:"target"`
	Env          string   `json:"env"`
	AllOK        bool     `json:"all_ok"`
	BlockerCount int      `json:"blocker_count"`
	Checks       []Check  `json:"checks"`
	NextSteps    []string `json:"next_steps"`
}

// JSON serialises the report for the MCP agent.
func (r *Report) JSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

// Input for PreflightUseCase.Execute.
type Input struct {
	Target     Target
	Env        string
	Config     *config.Config
	ProjectDir string // base dir for compose / Dockerfile checks; "" = "."
}

// Output of PreflightUseCase.Execute.
type Output struct {
	Report *Report
}

// ── use case ──────────────────────────────────────────────────────────────────

// PreflightUseCase runs all prerequisite checks.
type PreflightUseCase struct {
	docker DockerChecker
	ssh    SSHChecker
}

// New constructs a PreflightUseCase with the given checkers.
func New(docker DockerChecker, ssh SSHChecker) *PreflightUseCase {
	return &PreflightUseCase{docker: docker, ssh: ssh}
}

// Execute runs checks appropriate for the target and returns a structured report.
func (uc *PreflightUseCase) Execute(ctx context.Context, in Input) (Output, error) {
	if in.ProjectDir == "" {
		in.ProjectDir = "."
	}
	r := &Report{Target: in.Target, Env: in.Env}
	cfg := in.Config

	// ── 1. Registry image ─────────────────────────────────────────────────
	if cfg.Registry.Image == "" || isPlaceholder(cfg.Registry.Image) {
		r.add(Check{
			Name:        "registry_image",
			Description: "registry.image is set and not a placeholder",
			Status:      StatusError,
			Message:     fmt.Sprintf("image is a placeholder: %q", cfg.Registry.Image),
			FixType:     FixHuman,
			HumanInstruction: fmt.Sprintf(
				"Edit pilot.yaml:\n  registry:\n    image: ghcr.io/YOUR_USER/%s",
				cfg.Project.Name,
			),
		})
	} else {
		r.add(Check{
			Name:        "registry_image",
			Description: "registry.image is set and not a placeholder",
			Status:      StatusOK,
			Message:     cfg.Registry.Image,
		})
	}

	// ── 2. Dockerfile (push / deploy only) ────────────────────────────────
	if in.Target == TargetPush || in.Target == TargetDeploy {
		dockerfile := resolveDockerfile(cfg, in.ProjectDir)
		if _, err := os.Stat(dockerfile); os.IsNotExist(err) {
			r.add(Check{
				Name:             "dockerfile",
				Description:      "Dockerfile exists",
				Status:           StatusError,
				Message:          fmt.Sprintf("%s not found", dockerfile),
				FixType:          FixAgent,
				AgentTool:        "pilot_generate_dockerfile",
				HumanInstruction: "Ask your AI agent to generate the Dockerfile, or create it manually.",
			})
		} else {
			r.add(Check{
				Name:        "dockerfile",
				Description: "Dockerfile exists",
				Status:      StatusOK,
				Message:     dockerfile,
			})
		}
	}

	// ── 3. Docker daemon (up / push / deploy) ─────────────────────────────
	if in.Target == TargetUp || in.Target == TargetPush || in.Target == TargetDeploy {
		if err := uc.docker.IsRunning(ctx); err != nil {
			r.add(Check{
				Name:             "docker_daemon",
				Description:      "Docker daemon is running",
				Status:           StatusError,
				Message:          err.Error(),
				FixType:          FixHuman,
				HumanInstruction: "Start Docker Desktop (macOS) or run: sudo systemctl start docker",
			})
		} else {
			r.add(Check{
				Name:        "docker_daemon",
				Description: "Docker daemon is running",
				Status:      StatusOK,
				Message:     "docker daemon reachable",
			})
		}
	}

	// ── 4. Registry credentials (push / deploy) ───────────────────────────
	if in.Target == TargetPush || in.Target == TargetDeploy {
		r.add(checkRegistryCreds(cfg))
	}

	// ── 5. Compose file (up / deploy) ─────────────────────────────────────
	if in.Target == TargetUp || in.Target == TargetDeploy {
		composeFile := filepath.Join(in.ProjectDir, fmt.Sprintf("docker-compose.%s.yml", in.Env))
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			r.add(Check{
				Name:             "compose_file",
				Description:      fmt.Sprintf("docker-compose.%s.yml exists", in.Env),
				Status:           StatusError,
				Message:          fmt.Sprintf("%s not found", composeFile),
				FixType:          FixAgent,
				AgentTool:        "pilot_generate_compose",
				HumanInstruction: "Ask your AI agent to generate the compose file, or create it manually.",
			})
		} else {
			r.add(Check{
				Name:        "compose_file",
				Description: fmt.Sprintf("docker-compose.%s.yml exists", in.Env),
				Status:      StatusOK,
				Message:     composeFile,
			})
		}
	}

	// ── 6. Remote checks (deploy only) ────────────────────────────────────
	if in.Target == TargetDeploy {
		uc.remoteChecks(ctx, r, in, cfg)
	}

	r.finalize()
	return Output{Report: r}, nil
}

// remoteChecks runs SSH-based verifications for deploy targets.
func (uc *PreflightUseCase) remoteChecks(ctx context.Context, r *Report, in Input, cfg *config.Config) {
	envCfg, ok := cfg.Environments[in.Env]
	if !ok {
		return
	}
	targetName := envCfg.Target
	if targetName == "" {
		r.add(Check{
			Name:             "target_host",
			Description:      "Deploy target host is configured",
			Status:           StatusError,
			Message:          fmt.Sprintf("environment %q has no deploy target", in.Env),
			FixType:          FixHuman,
			HumanInstruction: fmt.Sprintf("Add targets.vps-prod and environments.%s.target in pilot.yaml", in.Env),
		})
		return
	}

	tgt, ok := cfg.Targets[targetName]
	if !ok || tgt.Host == "" {
		r.add(Check{
			Name:             "target_host",
			Description:      "Deploy target host is configured",
			Status:           StatusError,
			Message:          fmt.Sprintf("target %q has no host set", targetName),
			FixType:          FixHuman,
			HumanInstruction: fmt.Sprintf("Set targets.%s.host in pilot.yaml", targetName),
		})
		return
	}

	r.add(Check{
		Name:        "target_host",
		Description: "Deploy target host is configured",
		Status:      StatusOK,
		Message:     fmt.Sprintf("%s (%s)", tgt.Host, targetName),
	})

	port := tgt.Port
	if port == 0 {
		port = 22
	}
	keyPath := expandHome(tgt.Key)

	// SSH connectivity.
	if err := uc.ssh.CheckConnectivity(ctx, tgt.Host, tgt.User, keyPath, port); err != nil {
		r.add(Check{
			Name:    "vps_connectivity",
			Description: "Can connect to VPS via SSH",
			Status:  StatusError,
			Message: err.Error(),
			FixType: FixHuman,
			HumanInstruction: fmt.Sprintf(
				"Verify VPS is running at %s and port %d is open.\nTest: ssh -i %s %s@%s",
				tgt.Host, port, keyPath, tgt.User, tgt.Host,
			),
		})
		return
	}
	r.add(Check{
		Name:        "vps_connectivity",
		Description: "Can connect to VPS via SSH",
		Status:      StatusOK,
		Message:     fmt.Sprintf("connected to %s@%s", tgt.User, tgt.Host),
	})

	// Docker group membership.
	if ok, err := uc.ssh.HasDockerAccess(ctx, tgt.Host, tgt.User, keyPath, port); err != nil || !ok {
		msg := fmt.Sprintf("user %q is not in the docker group on %s", tgt.User, tgt.Host)
		if err != nil {
			msg = err.Error()
		}
		r.add(Check{
			Name:             "vps_docker_group",
			Description:      fmt.Sprintf("User %q has docker access on VPS", tgt.User),
			Status:           StatusError,
			Message:          msg,
			FixType:          FixAgent,
			AgentTool:        fmt.Sprintf("pilot_setup {\"env\": %q}", in.Env),
			HumanInstruction: fmt.Sprintf("On VPS: sudo usermod -aG docker %s && newgrp docker", tgt.User),
		})
	} else {
		r.add(Check{
			Name:        "vps_docker_group",
			Description: fmt.Sprintf("User %q has docker access on VPS", tgt.User),
			Status:      StatusOK,
			Message:     fmt.Sprintf("%s can run docker commands", tgt.User),
		})
	}

	// Env file present on VPS.
	if envCfg.EnvFile != "" {
		base := filepath.Base(envCfg.EnvFile)
		remotePath := fmt.Sprintf("~/pilot/%s", base)
		if found, err := uc.ssh.FileExists(ctx, tgt.Host, tgt.User, keyPath, port, remotePath); err != nil || !found {
			r.add(Check{
				Name:             "vps_env_file",
				Description:      fmt.Sprintf("%s present on VPS", envCfg.EnvFile),
				Status:           StatusError,
				Message:          fmt.Sprintf("%s not found at %s on VPS", envCfg.EnvFile, remotePath),
				FixType:          FixAgent,
				AgentTool:        fmt.Sprintf("pilot_sync {\"env\": %q}", in.Env),
				HumanInstruction: fmt.Sprintf("Run: pilot sync --env %s", in.Env),
			})
		} else {
			r.add(Check{
				Name:        "vps_env_file",
				Description: fmt.Sprintf("%s present on VPS", envCfg.EnvFile),
				Status:      StatusOK,
				Message:     fmt.Sprintf("%s synced at %s", envCfg.EnvFile, remotePath),
			})
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (r *Report) add(c Check) { r.Checks = append(r.Checks, c) }

func (r *Report) finalize() {
	var humanSteps, agentSteps []string
	blockers := 0

	for _, c := range r.Checks {
		if c.Status == StatusError || c.Status == StatusWarning {
			blockers++
			switch c.FixType {
			case FixHuman:
				humanSteps = append(humanSteps, fmt.Sprintf("[HUMAN] %s: %s", c.Name, c.HumanInstruction))
			case FixHumanThenAgent:
				humanSteps = append(humanSteps, fmt.Sprintf("[HUMAN] %s: %s", c.Name, c.HumanInstruction))
				agentSteps = append(agentSteps, fmt.Sprintf("[AGENT] call %s (after human step)", c.AgentTool))
			case FixAgent:
				agentSteps = append(agentSteps, fmt.Sprintf("[AGENT] call %s", c.AgentTool))
			}
		}
	}

	r.BlockerCount = blockers
	r.AllOK = blockers == 0
	r.NextSteps = append(humanSteps, agentSteps...)

	if r.AllOK {
		switch r.Target {
		case TargetPush:
			r.NextSteps = []string{"[AGENT] call pilot_push"}
		case TargetDeploy:
			r.NextSteps = []string{"[AGENT] call pilot_push", fmt.Sprintf("[AGENT] call pilot_deploy {\"env\": %q}", r.Env)}
		case TargetUp:
			r.NextSteps = []string{"[AGENT] call pilot_up"}
		}
	}
}

func checkRegistryCreds(cfg *config.Config) Check {
	c := Check{
		Name:        "registry_creds",
		Description: fmt.Sprintf("Credentials for %s are available", cfg.Registry.Provider),
	}
	switch cfg.Registry.Provider {
	case "ghcr":
		token, actor := os.Getenv("GITHUB_TOKEN"), os.Getenv("GITHUB_ACTOR")
		if token == "" || actor == "" {
			c.Status = StatusError
			c.Message = missingVarsMsg(map[string]string{"GITHUB_TOKEN": token, "GITHUB_ACTOR": actor})
			c.FixType = FixHuman
			c.HumanInstruction = "export GITHUB_ACTOR=<username>\nexport GITHUB_TOKEN=<token with write:packages>"
		} else {
			c.Status = StatusOK
			c.Message = fmt.Sprintf("GITHUB_ACTOR=%s ✓", actor)
		}
	case "dockerhub":
		user, pass := os.Getenv("DOCKER_USERNAME"), os.Getenv("DOCKER_PASSWORD")
		if user == "" || pass == "" {
			c.Status = StatusError
			c.Message = missingVarsMsg(map[string]string{"DOCKER_USERNAME": user, "DOCKER_PASSWORD": pass})
			c.FixType = FixHuman
			c.HumanInstruction = "export DOCKER_USERNAME=<username>\nexport DOCKER_PASSWORD=<access-token>"
		} else {
			c.Status = StatusOK
			c.Message = fmt.Sprintf("DOCKER_USERNAME=%s ✓", user)
		}
	case "custom":
		user, pass := os.Getenv("REGISTRY_USERNAME"), os.Getenv("REGISTRY_PASSWORD")
		if user == "" || pass == "" {
			c.Status = StatusError
			c.Message = missingVarsMsg(map[string]string{"REGISTRY_USERNAME": user, "REGISTRY_PASSWORD": pass})
			c.FixType = FixHuman
			c.HumanInstruction = "export REGISTRY_USERNAME=<username>\nexport REGISTRY_PASSWORD=<password>"
		} else {
			c.Status = StatusOK
			c.Message = fmt.Sprintf("REGISTRY_USERNAME=%s ✓", user)
		}
	default:
		c.Status = StatusWarning
		c.Message = fmt.Sprintf("unknown registry provider %q — cannot verify credentials", cfg.Registry.Provider)
	}
	return c
}

func missingVarsMsg(vars map[string]string) string {
	var missing []string
	for k, v := range vars {
		if v == "" {
			missing = append(missing, k)
		}
	}
	return fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", "))
}

func isPlaceholder(image string) bool {
	return strings.Contains(image, "YOUR_") ||
		strings.Contains(image, "your-user") ||
		strings.Contains(image, "your-github")
}

func resolveDockerfile(cfg *config.Config, projectDir string) string {
	for _, svc := range cfg.Services {
		if svc.Type == "app" && svc.Dockerfile != "" {
			return filepath.Join(projectDir, svc.Dockerfile)
		}
	}
	return filepath.Join(projectDir, "Dockerfile")
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
