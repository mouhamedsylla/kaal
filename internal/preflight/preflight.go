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

	"github.com/mouhamedsylla/kaal/internal/config"
	"github.com/mouhamedsylla/kaal/internal/env"
	kaalSSH "github.com/mouhamedsylla/kaal/pkg/ssh"
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
			Name:             "kaal_yaml",
			Description:      "kaal.yaml exists and is valid",
			Status:           StatusError,
			Message:          err.Error(),
			FixType:          FixHuman,
			HumanInstruction: "Run 'kaal init' to create kaal.yaml, or fix the YAML syntax error.",
		})
		r.finalize()
		return r, nil // can't continue without config
	}
	r.add(Check{
		Name:        "kaal_yaml",
		Description: "kaal.yaml exists and is valid",
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
			HumanInstruction: fmt.Sprintf("Edit kaal.yaml:\n  registry:\n    image: ghcr.io/YOUR_USER/%s\nReplace YOUR_USER with your real username.", cfg.Project.Name),
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
				AgentTool:        "kaal_generate_dockerfile",
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
				AgentTool:        "kaal_generate_compose",
				HumanInstruction: "Ask your AI agent to generate the compose file, or create it manually.",
			})
		} else {
			r.add(Check{
				Name:        "compose_file",
				Description: fmt.Sprintf("%s exists", composeFile),
				Status:      StatusOK,
				Message:     composeFile,
			})
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
		if tgt == nil || tgt.Host == "" {
			r.add(Check{
				Name:             "target_host",
				Description:      "Deploy target host is configured",
				Status:           StatusError,
				Message:          fmt.Sprintf("no host set for target %q in kaal.yaml", targetName),
				FixType:          FixHuman,
				HumanInstruction: fmt.Sprintf("Edit kaal.yaml:\n  targets:\n    %s:\n      host: \"YOUR_VPS_IP\"\nThen run: kaal setup --env %s", targetName, activeEnv),
			})
		} else {
			r.add(Check{
				Name:        "target_host",
				Description: "Deploy target host is configured",
				Status:      StatusOK,
				Message:     fmt.Sprintf("%s (%s)", tgt.Host, targetName),
			})

			// 8. SSH key exists
			keyPath := expandHome(tgt.Key)
			if keyPath == "" {
				keyPath = expandHome("~/.ssh/id_kaal")
			}
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				r.add(Check{
					Name:             "ssh_key",
					Description:      "SSH key file exists",
					Status:           StatusError,
					Message:          fmt.Sprintf("%s not found", keyPath),
					FixType:          FixHuman,
					HumanInstruction: fmt.Sprintf("Generate a key and add it to your VPS:\n  ssh-keygen -t ed25519 -f %s\n  ssh-copy-id -i %s.pub %s@%s", keyPath, keyPath, tgt.User, tgt.Host),
				})
			} else {
				r.add(Check{
					Name:        "ssh_key",
					Description: "SSH key file exists",
					Status:      StatusOK,
					Message:     keyPath,
				})

				// 9. VPS SSH connectivity (5s timeout)
				vpsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				port := tgt.Port
				if port == 0 {
					port = 22
				}
				client, sshErr := kaalSSH.NewClient(kaalSSH.Config{
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
							AgentTool:        fmt.Sprintf("kaal_setup {\"env\": \"%s\"}", activeEnv),
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
						// kaal sync copies files to ~/kaal/ using only the basename
						base := envCfg.EnvFile
						if idx := strings.LastIndex(base, "/"); idx >= 0 {
							base = base[idx+1:]
						}
						remoteEnvPath := fmt.Sprintf("~/kaal/%s", base)
						checkOut, _ := client.Run(vpsCtx, fmt.Sprintf("test -f %s && echo ok || echo missing", remoteEnvPath))
						if strings.TrimSpace(checkOut) != "ok" {
							r.add(Check{
								Name:             "vps_env_file",
								Description:      fmt.Sprintf("%s present on VPS", envCfg.EnvFile),
								Status:           StatusError,
								Message:          fmt.Sprintf("%s not found at %s on the VPS", envCfg.EnvFile, remoteEnvPath),
								FixType:          FixAgent,
								AgentTool:        fmt.Sprintf("kaal_sync {\"env\": \"%s\"}", activeEnv),
								HumanInstruction: fmt.Sprintf("Run: kaal sync --env %s\nThis copies kaal.yaml, compose files and env files to ~/kaal/ on the VPS.", activeEnv),
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
			r.NextSteps = []string{"[AGENT] call kaal_push"}
		case TargetDeploy:
			r.NextSteps = []string{"[AGENT] call kaal_push", fmt.Sprintf("[AGENT] call kaal_deploy {\"env\": \"%s\"}", r.Env)}
		case TargetUp:
			r.NextSteps = []string{"[AGENT] call kaal_up"}
		}
	} else {
		// Add deploy step at the end so agent knows what the goal is
		switch r.Target {
		case TargetPush:
			r.NextSteps = append(r.NextSteps, "[AGENT] call kaal_push  ← final goal")
		case TargetDeploy:
			r.NextSteps = append(r.NextSteps, "[AGENT] call kaal_push + kaal_deploy  ← final goal")
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

// JSON returns the report serialized to indented JSON.
func (r *Report) JSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}
