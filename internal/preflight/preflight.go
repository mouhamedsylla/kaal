// Package preflight verifies all prerequisites before push / deploy.
// It returns a structured report that AI agents use to decide what to fix
// themselves, what to ask the human for, and in what order.
package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mouhamedsylla/pilot/internal/config"
	"github.com/mouhamedsylla/pilot/internal/env"
	pilotSSH "github.com/mouhamedsylla/pilot/pkg/ssh"
	"gopkg.in/yaml.v3"
)

// Target is the goal of a preflight run.
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

// FixType tells the agent who needs to act.
type FixType string

const (
	FixNone           FixType = ""
	FixAgent          FixType = "agent"           // agent calls a tool
	FixHuman          FixType = "human"            // human must act first
	FixHumanThenAgent FixType = "human_then_agent" // human acts, then agent calls a tool
)

// Check is the result of one verification step.
type Check struct {
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	Status           Status  `json:"status"`
	Message          string  `json:"message"`
	FixType          FixType `json:"fix_type,omitempty"`
	AgentTool        string  `json:"agent_tool,omitempty"`        // MCP tool to call
	HumanInstruction string  `json:"human_instruction,omitempty"` // exact text for the user
}

// Report is the full preflight result.
type Report struct {
	Target       Target   `json:"target"`
	Env          string   `json:"env"`
	AllOK        bool     `json:"all_ok"`
	BlockerCount int      `json:"blocker_count"`
	Checks       []Check  `json:"checks"`
	NextSteps    []string `json:"next_steps"` // ordered action plan for the agent
}

// Run executes all checks appropriate for the given target and env.
func Run(ctx context.Context, target Target, activeEnv string) (*Report, error) {
	r := &Report{Target: target, Env: activeEnv}

	// ── 1. Config ──────────────────────────────────────────────────────────
	cfg, err := config.Load(".")
	if err != nil {
		r.add(Check{
			Name:             "pilot_yaml",
			Description:      "pilot.yaml exists and is valid",
			Status:           StatusError,
			Message:          err.Error(),
			FixType:          FixHuman,
			HumanInstruction: "Run 'pilot init' to create pilot.yaml, or fix the YAML syntax error.",
		})
		r.finalize()
		return r, nil // can't continue without config
	}
	r.add(Check{
		Name:        "pilot_yaml",
		Description: "pilot.yaml exists and is valid",
		Status:      StatusOK,
		Message:     fmt.Sprintf("project: %s", cfg.Project.Name),
	})

	// ── 2. Image placeholder ───────────────────────────────────────────────
	if cfg.Registry.Image == "" || isPlaceholder(cfg.Registry.Image) {
		r.add(Check{
			Name:             "registry_image",
			Description:      "registry.image is set and not a placeholder",
			Status:           StatusError,
			Message:          fmt.Sprintf("image is a placeholder: %q", cfg.Registry.Image),
			FixType:          FixHuman,
			HumanInstruction: fmt.Sprintf("Edit pilot.yaml:\n  registry:\n    image: ghcr.io/YOUR_USER/%s\nReplace YOUR_USER with your real username.", cfg.Project.Name),
		})
	} else {
		r.add(Check{
			Name:        "registry_image",
			Description: "registry.image is set and not a placeholder",
			Status:      StatusOK,
			Message:     cfg.Registry.Image,
		})
	}

	// ── 3. Dockerfile ──────────────────────────────────────────────────────
	if target == TargetPush || target == TargetDeploy {
		dockerfile := resolveDockerfile(cfg)
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

	// ── 4. Docker daemon ───────────────────────────────────────────────────
	if target == TargetUp || target == TargetPush || target == TargetDeploy {
		if err := checkDockerDaemon(ctx); err != nil {
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

	// ── 5. Registry credentials ────────────────────────────────────────────
	if target == TargetPush || target == TargetDeploy {
		if c := checkRegistryCreds(cfg); c.Status != StatusOK {
			r.add(c)
		} else {
			r.add(c)
		}
	}

	// ── 6. Compose file ────────────────────────────────────────────────────
	if target == TargetUp || target == TargetDeploy {
		composeFile := fmt.Sprintf("docker-compose.%s.yml", activeEnv)
		if _, err := os.Stat(composeFile); os.IsNotExist(err) {
			r.add(Check{
				Name:             "compose_file",
				Description:      fmt.Sprintf("%s exists", composeFile),
				Status:           StatusError,
				Message:          fmt.Sprintf("%s not found", composeFile),
				FixType:          FixAgent,
				AgentTool:        "pilot_generate_compose",
				HumanInstruction: "Ask your AI agent to generate the compose file, or create it manually.",
			})
		} else {
			r.add(Check{
				Name:        "compose_file",
				Description: fmt.Sprintf("%s exists", composeFile),
				Status:      StatusOK,
				Message:     composeFile,
			})

			// ── 6a. App services must have env_file ────────────────────────
			// Without env_file, Docker image ENV vars (baked as "" when no
			// --build-arg is passed) override .env.* files at runtime.
			// This silently breaks all VITE_*, NEXT_PUBLIC_*, etc. vars.
			if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.EnvFile != "" {
				if missing := composeServicesWithoutEnvFile(composeFile, envCfg.EnvFile); len(missing) > 0 {
					r.add(Check{
						Name:        "compose_env_file",
						Description: "All app services declare env_file in compose",
						Status:      StatusWarning,
						Message: fmt.Sprintf(
							"service(s) %s in %s have no env_file — runtime vars will be empty if the Dockerfile uses ARG/ENV pattern",
							strings.Join(missing, ", "), composeFile,
						),
						FixType: FixAgent,
						AgentTool: "pilot_generate_compose",
						HumanInstruction: fmt.Sprintf(
							"Add env_file: %s to each app service in %s.\n"+
								"Without it, VITE_*/NEXT_PUBLIC_*/REACT_APP_* vars will be empty in the container.",
							envCfg.EnvFile, composeFile,
						),
					})
				} else {
					r.add(Check{
						Name:        "compose_env_file",
						Description: "All app services declare env_file in compose",
						Status:      StatusOK,
						Message:     fmt.Sprintf("env_file: %s", envCfg.EnvFile),
					})
				}
			}
		}
	}

	// ── 6b. Build-args gap (push / deploy, node stack) ─────────────────────
	// Detect compile-time vars (VITE_*, NEXT_PUBLIC_*, etc.) present in the
	// env file but absent from registry.build_args. These will be silently
	// empty in the built image — the most common silent failure for frontend apps.
	if (target == TargetPush || target == TargetDeploy) && len(cfg.Registry.BuildArgs) > 0 {
		if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.EnvFile != "" {
			if unlisted := buildArgsGap(cfg, envCfg.EnvFile); len(unlisted) > 0 {
				r.add(Check{
					Name:        "build_args_gap",
					Description: "All compile-time vars are in registry.build_args",
					Status:      StatusWarning,
					Message: fmt.Sprintf(
						"%d var(s) in %s look like compile-time vars but are not in pilot.yaml registry.build_args: %s — they will be empty in the built image",
						len(unlisted), envCfg.EnvFile, strings.Join(unlisted, ", "),
					),
					FixType: FixHuman,
					HumanInstruction: fmt.Sprintf(
						"Add to pilot.yaml under registry.build_args:\n%s",
						formatBuildArgsSuggestion(unlisted),
					),
				})
			} else {
				r.add(Check{
					Name:        "build_args_gap",
					Description: "All compile-time vars are in registry.build_args",
					Status:      StatusOK,
					Message:     "no unlisted compile-time vars detected",
				})
			}
		}
	}

	// ── 7–10. Remote checks (deploy only) ──────────────────────────────────
	if target == TargetDeploy {
		envCfg, ok := cfg.Environments[activeEnv]
		targetName := ""
		if ok {
			targetName = envCfg.Target
		}

		var tgt *config.Target
		if targetName != "" {
			if t, ok := cfg.Targets[targetName]; ok {
				tgt = &t
			}
		}

		// 7. Target host configured
		if targetName == "" {
			// The active env has no deploy target at all — it's a local-only environment.
			// Give a clear actionable message instead of the confusing empty-string error.
			remoteEnv := firstRemoteEnv(cfg, activeEnv)
			hint := fmt.Sprintf("Environment %q has no deploy target — it is a local environment.\n", activeEnv)
			if remoteEnv != "" {
				hint += fmt.Sprintf("Run: pilot preflight --target deploy --env %s", remoteEnv)
			} else {
				hint += "Add a target to pilot.yaml:\n  environments:\n    prod:\n      target: vps-prod\n  targets:\n    vps-prod:\n      type: vps\n      host: \"YOUR_VPS_IP\"\n      user: deploy\n      key: ~/.ssh/id_pilot"
			}
			r.add(Check{
				Name:             "target_host",
				Description:      "Deploy target host is configured",
				Status:           StatusError,
				Message:          fmt.Sprintf("environment %q has no deploy target (it is a local-only env)", activeEnv),
				FixType:          FixHuman,
				HumanInstruction: hint,
			})
		} else if tgt == nil || tgt.Host == "" {
			r.add(Check{
				Name:             "target_host",
				Description:      "Deploy target host is configured",
				Status:           StatusError,
				Message:          fmt.Sprintf("target %q exists in pilot.yaml but has no host set", targetName),
				FixType:          FixHuman,
				HumanInstruction: fmt.Sprintf("Edit pilot.yaml:\n  targets:\n    %s:\n      host: \"YOUR_VPS_IP\"\nThen run: pilot setup --env %s", targetName, activeEnv),
			})
		} else {
			r.add(Check{
				Name:        "target_host",
				Description: "Deploy target host is configured",
				Status:      StatusOK,
				Message:     fmt.Sprintf("%s (%s)", tgt.Host, targetName),
			})

			// 8. SSH key — file or PILOT_SSH_KEY env var
			keyPath := expandHome(tgt.Key)
			if keyPath == "" {
				keyPath = expandHome("~/.ssh/id_pilot")
			}
			pilotSSHKeyEnv := os.Getenv("PILOT_SSH_KEY")
			sshKeyOK := false
			if pilotSSHKeyEnv != "" {
				// Env var takes priority over file — valid for CI/CD pipelines.
				sshKeyOK = true
				r.add(Check{
					Name:        "ssh_key",
					Description: "SSH key available",
					Status:      StatusOK,
					Message:     "PILOT_SSH_KEY env var set (CI/CD mode)",
				})
			} else if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				r.add(Check{
					Name:    "ssh_key",
					Description: "SSH key file exists",
					Status:  StatusError,
					Message: fmt.Sprintf("%s not found", keyPath),
					FixType: FixHuman,
					HumanInstruction: fmt.Sprintf(
						"Option A — generate and register a key:\n"+
							"  ssh-keygen -t ed25519 -f %s\n"+
							"  ssh-copy-id -i %s.pub %s@%s\n\n"+
							"Option B — use env var (CI/CD):\n"+
							"  export PILOT_SSH_KEY=\"$(cat %s)\"",
						keyPath, keyPath, tgt.User, tgt.Host, keyPath,
					),
				})
			} else {
				sshKeyOK = true
				r.add(Check{
					Name:        "ssh_key",
					Description: "SSH key file exists",
					Status:      StatusOK,
					Message:     keyPath,
				})
			}
			if sshKeyOK {

				// 9. VPS SSH connectivity (5s timeout)
				vpsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				port := tgt.Port
				if port == 0 {
					port = 22
				}
				client, sshErr := pilotSSH.NewClient(pilotSSH.Config{
					Host:    tgt.Host,
					User:    tgt.User,
					KeyPath: tgt.Key,
					Port:    port,
				})
				if sshErr != nil {
					r.add(Check{
						Name:             "vps_connectivity",
						Description:      "Can connect to VPS via SSH",
						Status:           StatusError,
						Message:          sshErr.Error(),
						FixType:          FixHuman,
						HumanInstruction: fmt.Sprintf("Verify:\n  1. VPS is running at %s\n  2. Port %d is open\n  3. Test: ssh -i %s %s@%s\n  4. Make sure the SSH public key is in ~/.ssh/authorized_keys on the VPS", tgt.Host, port, keyPath, tgt.User, tgt.Host),
					})
				} else {
					r.add(Check{
						Name:        "vps_connectivity",
						Description: "Can connect to VPS via SSH",
						Status:      StatusOK,
						Message:     fmt.Sprintf("connected to %s@%s", tgt.User, tgt.Host),
					})

					// 10. VPS docker group membership
					out, _ := client.Run(vpsCtx, "docker ps 2>&1")
					if isDockerPermErr(out) {
						r.add(Check{
							Name:             "vps_docker_group",
							Description:      fmt.Sprintf("User %q has docker access on VPS", tgt.User),
							Status:           StatusError,
							Message:          fmt.Sprintf("user %q is not in the docker group on %s", tgt.User, tgt.Host),
							FixType:          FixAgent,
							AgentTool:        fmt.Sprintf("pilot_setup {\"env\": \"%s\"}", activeEnv),
							HumanInstruction: fmt.Sprintf("Or manually on VPS:\n  sudo usermod -aG docker %s\nThen reconnect.", tgt.User),
						})
					} else {
						r.add(Check{
							Name:        "vps_docker_group",
							Description: fmt.Sprintf("User %q has docker access on VPS", tgt.User),
							Status:      StatusOK,
							Message:     fmt.Sprintf("%s can run docker commands", tgt.User),
						})
					}
					// 11. Env file synced to VPS (only if env_file is declared for this env)
					if envCfg, ok := cfg.Environments[activeEnv]; ok && envCfg.EnvFile != "" {
						// pilot sync copies files to ~/pilot/ using only the basename
						base := envCfg.EnvFile
						if idx := strings.LastIndex(base, "/"); idx >= 0 {
							base = base[idx+1:]
						}
						remoteEnvPath := fmt.Sprintf("~/pilot/%s", base)
						checkOut, _ := client.Run(vpsCtx, fmt.Sprintf("test -f %s && echo ok || echo missing", remoteEnvPath))
						if strings.TrimSpace(checkOut) != "ok" {
							r.add(Check{
								Name:             "vps_env_file",
								Description:      fmt.Sprintf("%s present on VPS", envCfg.EnvFile),
								Status:           StatusError,
								Message:          fmt.Sprintf("%s not found at %s on the VPS", envCfg.EnvFile, remoteEnvPath),
								FixType:          FixAgent,
								AgentTool:        fmt.Sprintf("pilot_sync {\"env\": \"%s\"}", activeEnv),
								HumanInstruction: fmt.Sprintf("Run: pilot sync --env %s\nThis copies pilot.yaml, compose files and env files to ~/pilot/ on the VPS.", activeEnv),
							})
						} else {
							r.add(Check{
								Name:        "vps_env_file",
								Description: fmt.Sprintf("%s present on VPS", envCfg.EnvFile),
								Status:      StatusOK,
								Message:     fmt.Sprintf("%s synced at %s", envCfg.EnvFile, remoteEnvPath),
							})
						}
					}

					client.Close()
				}
			}
		}
	}

	r.finalize()
	return r, nil
}

// ─────────────────────────── helpers ───────────────────────────────────────

func (r *Report) add(c Check) {
	r.Checks = append(r.Checks, c)
}

func (r *Report) finalize() {
	humanSteps := []string{}
	agentSteps := []string{}
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

	// Build ordered next_steps: human actions first, then agent, then deploy
	r.NextSteps = append(humanSteps, agentSteps...)
	if r.AllOK {
		switch r.Target {
		case TargetPush:
			r.NextSteps = []string{"[AGENT] call pilot_push"}
		case TargetDeploy:
			r.NextSteps = []string{"[AGENT] call pilot_push", fmt.Sprintf("[AGENT] call pilot_deploy {\"env\": \"%s\"}", r.Env)}
		case TargetUp:
			r.NextSteps = []string{"[AGENT] call pilot_up"}
		}
	} else {
		// Add deploy step at the end so agent knows what the goal is
		switch r.Target {
		case TargetPush:
			r.NextSteps = append(r.NextSteps, "[AGENT] call pilot_push  ← final goal")
		case TargetDeploy:
			r.NextSteps = append(r.NextSteps, "[AGENT] call pilot_push + pilot_deploy  ← final goal")
		}
	}
}

func checkDockerDaemon(ctx context.Context) error {
	dCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(dCtx, "docker", "info").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker daemon not reachable: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func checkRegistryCreds(cfg *config.Config) Check {
	c := Check{
		Name:        "registry_creds",
		Description: fmt.Sprintf("Credentials for %s are available", cfg.Registry.Provider),
	}

	switch cfg.Registry.Provider {
	case "dockerhub":
		user := os.Getenv("DOCKER_USERNAME")
		pass := os.Getenv("DOCKER_PASSWORD")
		if user == "" || pass == "" {
			missing := missingVars(map[string]string{
				"DOCKER_USERNAME": user,
				"DOCKER_PASSWORD": pass,
			})
			c.Status = StatusError
			c.Message = fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", "))
			c.FixType = FixHuman
			c.HumanInstruction = "Set the missing env vars (use a Docker Hub Access Token for DOCKER_PASSWORD):\n" +
				"  export DOCKER_USERNAME=your-username\n" +
				"  export DOCKER_PASSWORD=your-access-token\n\n" +
				"Create a token at: https://hub.docker.com/settings/security\n" +
				"Required scope: Read & Write (or Read, Write, Delete)"
		} else {
			c.Status = StatusOK
			c.Message = fmt.Sprintf("DOCKER_USERNAME=%s ✓", user)
		}

	case "ghcr":
		token := os.Getenv("GITHUB_TOKEN")
		actor := os.Getenv("GITHUB_ACTOR")
		if token == "" || actor == "" {
			missing := missingVars(map[string]string{
				"GITHUB_TOKEN": token,
				"GITHUB_ACTOR": actor,
			})
			c.Status = StatusError
			c.Message = fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", "))
			c.FixType = FixHuman
			c.HumanInstruction = "Set the missing env vars:\n" +
				"  export GITHUB_ACTOR=your-github-username\n" +
				"  export GITHUB_TOKEN=ghp_xxxxxxxxxxxx\n\n" +
				"Create a token at: https://github.com/settings/tokens\n" +
				"Required scope: write:packages (includes read:packages)"
		} else {
			c.Status = StatusOK
			c.Message = fmt.Sprintf("GITHUB_ACTOR=%s ✓", actor)
		}

	case "custom":
		user := os.Getenv("REGISTRY_USERNAME")
		pass := os.Getenv("REGISTRY_PASSWORD")
		if user == "" || pass == "" {
			missing := missingVars(map[string]string{
				"REGISTRY_USERNAME": user,
				"REGISTRY_PASSWORD": pass,
			})
			c.Status = StatusError
			c.Message = fmt.Sprintf("missing env vars: %s", strings.Join(missing, ", "))
			c.FixType = FixHuman
			c.HumanInstruction = "Set the missing env vars for your custom registry:\n" +
				"  export REGISTRY_USERNAME=your-username\n" +
				"  export REGISTRY_PASSWORD=your-password-or-token"
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

func missingVars(vars map[string]string) []string {
	var missing []string
	for k, v := range vars {
		if v == "" {
			missing = append(missing, k)
		}
	}
	return missing
}

func isPlaceholder(image string) bool {
	return strings.Contains(image, "YOUR_") ||
		strings.Contains(image, "your-user") ||
		strings.Contains(image, "your-github")
}

func isDockerPermErr(output string) bool {
	lower := strings.ToLower(output)
	return (strings.Contains(lower, "permission denied") && strings.Contains(lower, "docker")) ||
		strings.Contains(lower, "got permission denied while trying to connect to the docker daemon")
}

func resolveDockerfile(cfg *config.Config) string {
	for _, svc := range cfg.Services {
		if svc.Type == "app" && svc.Dockerfile != "" {
			return svc.Dockerfile
		}
	}
	return "Dockerfile"
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

// ActiveEnv resolves the active environment.
func ActiveEnv(override string) string {
	return env.Active(override)
}

// DetectRemoteEnv loads pilot.yaml and returns the first environment that has a
// deploy target configured, excluding the currentEnv (which is assumed to be
// a local-only environment). Returns "" if no remote env is found.
// Used by cmd/preflight to auto-switch away from local envs when --target deploy
// is requested without an explicit --env flag.
func DetectRemoteEnv(currentEnv string) string {
	cfg, err := config.Load(".")
	if err != nil {
		return ""
	}
	return firstRemoteEnv(cfg, currentEnv)
}

// firstRemoteEnv returns the first environment name (other than skip) that has
// a deploy target with a non-empty host. Prefers "prod" over other names.
func firstRemoteEnv(cfg *config.Config, skip string) string {
	// Check "prod" first — most common remote env name
	for _, preferred := range []string{"prod", "production", "staging"} {
		if envCfg, ok := cfg.Environments[preferred]; ok && preferred != skip {
			if envCfg.Target != "" {
				if t, ok := cfg.Targets[envCfg.Target]; ok && t.Host != "" {
					return preferred
				}
			}
		}
	}
	// Fall back to any env with a target host
	for name, envCfg := range cfg.Environments {
		if name == skip || envCfg.Target == "" {
			continue
		}
		if t, ok := cfg.Targets[envCfg.Target]; ok && t.Host != "" {
			return name
		}
	}
	return ""
}

// JSON returns the report serialized to indented JSON.
func (r *Report) JSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

// composeServicesWithoutEnvFile parses a compose file and returns the names of
// app services (services with a build block) that don't declare env_file.
func composeServicesWithoutEnvFile(composeFile, expectedEnvFile string) []string {
	data, err := os.ReadFile(composeFile)
	if err != nil {
		return nil
	}
	var compose struct {
		Services map[string]struct {
			Build   any    `yaml:"build"`
			EnvFile any    `yaml:"env_file"` // string or []string
			Image   string `yaml:"image"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil
	}
	var missing []string
	for name, svc := range compose.Services {
		// Only check build-based services (app containers), not pre-built images.
		if svc.Build == nil {
			continue
		}
		hasEnvFile := false
		switch v := svc.EnvFile.(type) {
		case string:
			hasEnvFile = v != ""
		case []any:
			hasEnvFile = len(v) > 0
		}
		if !hasEnvFile {
			missing = append(missing, name)
		}
	}
	return missing
}

// buildArgsGap returns compile-time var names found in envFile that are not
// listed in cfg.Registry.BuildArgs. Only applies when build_args is explicitly
// set (acts as a whitelist). Returns nil when build_args is empty (auto-detect mode).
func buildArgsGap(cfg *config.Config, envFile string) []string {
	if len(cfg.Registry.BuildArgs) == 0 {
		return nil // auto-detect mode — no gap possible
	}
	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil
	}
	declared := map[string]bool{}
	for _, name := range cfg.Registry.BuildArgs {
		declared[name] = true
	}
	compilePrefixes := []string{"VITE_", "NEXT_PUBLIC_", "REACT_APP_", "PUBLIC_", "NUXT_PUBLIC_", "NG_APP_"}
	var unlisted []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		if declared[name] {
			continue
		}
		for _, prefix := range compilePrefixes {
			if strings.HasPrefix(name, prefix) {
				unlisted = append(unlisted, name)
				break
			}
		}
	}
	return unlisted
}

// formatBuildArgsSuggestion formats a list of var names as pilot.yaml build_args entries.
func formatBuildArgsSuggestion(names []string) string {
	lines := []string{"  registry:", "    build_args:"}
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("      - %s", name))
	}
	return strings.Join(lines, "\n")
}

