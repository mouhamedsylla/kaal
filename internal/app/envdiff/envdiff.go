// Package envdiff compares two pilot environments and reports their divergences.
//
// It checks three dimensions:
//  1. Variables: keys present in one .env file but absent in the other, or empty in one
//  2. Ports: ports declared in compose files that differ between the two environments
//  3. Services: services present in one compose file but absent in the other
package envdiff

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mouhamedsylla/pilot/internal/config"
)

// Input selects which environments to compare.
type Input struct {
	EnvA    string // e.g. "dev"
	EnvB    string // e.g. "prod"
	Config  *config.Config
}

// Output is the full diff result.
type Output struct {
	EnvA string
	EnvB string

	// Variable diff
	OnlyInA     []string        // keys present in A but not in B
	OnlyInB     []string        // keys present in B but not in A
	EmptyInA    []string        // keys present in both but empty in A
	EmptyInB    []string        // keys present in both but empty in B

	// Port diff (key = service name)
	PortDiffs []PortDiff

	// Service diff
	ServicesOnlyInA []string
	ServicesOnlyInB []string
}

// PortDiff describes a port discrepancy for a single service.
type PortDiff struct {
	Service string
	PortA   string // port in env A (empty = not declared)
	PortB   string // port in env B (empty = not declared)
}

// HasDiff reports whether any divergence was found.
func (o *Output) HasDiff() bool {
	return len(o.OnlyInA) > 0 || len(o.OnlyInB) > 0 ||
		len(o.EmptyInA) > 0 || len(o.EmptyInB) > 0 ||
		len(o.PortDiffs) > 0 ||
		len(o.ServicesOnlyInA) > 0 || len(o.ServicesOnlyInB) > 0
}

// UseCase computes the diff between two environments.
type UseCase struct{}

func New() *UseCase { return &UseCase{} }

// Execute computes the diff and returns the result.
func (uc *UseCase) Execute(in Input) (Output, error) {
	cfgA, ok := in.Config.Environments[in.EnvA]
	if !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.EnvA)
	}
	cfgB, ok := in.Config.Environments[in.EnvB]
	if !ok {
		return Output{}, fmt.Errorf("environment %q not defined in pilot.yaml", in.EnvB)
	}

	out := Output{EnvA: in.EnvA, EnvB: in.EnvB}

	// ── 1. variable diff ──────────────────────────────────────────────────────
	envFileA := cfgA.EnvFile
	if envFileA == "" {
		envFileA = fmt.Sprintf(".env.%s", in.EnvA)
	}
	envFileB := cfgB.EnvFile
	if envFileB == "" {
		envFileB = fmt.Sprintf(".env.%s", in.EnvB)
	}

	varsA, _ := parseEnvFile(envFileA) // best-effort: missing file = no vars
	varsB, _ := parseEnvFile(envFileB)

	for k := range varsA {
		if _, ok := varsB[k]; !ok {
			out.OnlyInA = append(out.OnlyInA, k)
		} else if varsA[k] == "" {
			out.EmptyInA = append(out.EmptyInA, k)
		}
	}
	for k := range varsB {
		if _, ok := varsA[k]; !ok {
			out.OnlyInB = append(out.OnlyInB, k)
		} else if varsB[k] == "" {
			out.EmptyInB = append(out.EmptyInB, k)
		}
	}

	sort.Strings(out.OnlyInA)
	sort.Strings(out.OnlyInB)
	sort.Strings(out.EmptyInA)
	sort.Strings(out.EmptyInB)

	// ── 2. service + port diff ────────────────────────────────────────────────
	composeA := composeFileForEnv(in.EnvA)
	composeB := composeFileForEnv(in.EnvB)

	servicesA, portsA := parseCompose(composeA)
	servicesB, portsB := parseCompose(composeB)

	// services only in A
	for svc := range servicesA {
		if _, ok := servicesB[svc]; !ok {
			out.ServicesOnlyInA = append(out.ServicesOnlyInA, svc)
		}
	}
	// services only in B
	for svc := range servicesB {
		if _, ok := servicesA[svc]; !ok {
			out.ServicesOnlyInB = append(out.ServicesOnlyInB, svc)
		}
	}

	// port diffs for services present in both
	allServices := map[string]bool{}
	for k := range servicesA {
		allServices[k] = true
	}
	for k := range servicesB {
		allServices[k] = true
	}
	for svc := range allServices {
		portA := portsA[svc]
		portB := portsB[svc]
		if portA != portB {
			out.PortDiffs = append(out.PortDiffs, PortDiff{Service: svc, PortA: portA, PortB: portB})
		}
	}

	sort.Strings(out.ServicesOnlyInA)
	sort.Strings(out.ServicesOnlyInB)
	sort.Slice(out.PortDiffs, func(i, j int) bool {
		return out.PortDiffs[i].Service < out.PortDiffs[j].Service
	})

	return out, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func composeFileForEnv(env string) string {
	return fmt.Sprintf("docker-compose.%s.yml", env)
}

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
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		vars[k] = v
	}
	return vars, scanner.Err()
}

// parseCompose reads a docker-compose file and returns:
//   - set of service names
//   - first published port per service (e.g. "8080" from "8080:80")
func parseCompose(path string) (map[string]bool, map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}, map[string]string{}
	}

	var compose struct {
		Services map[string]struct {
			Ports []json.RawMessage `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return map[string]bool{}, map[string]string{}
	}

	services := map[string]bool{}
	ports := map[string]string{}

	for name, svc := range compose.Services {
		services[name] = true
		for _, raw := range svc.Ports {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				// short syntax: "8080:80" or "8080"
				parts := strings.SplitN(s, ":", 2)
				ports[name] = parts[0]
				break
			}
			// long syntax: {target: 80, published: 8080}
			var m map[string]interface{}
			if err := json.Unmarshal(raw, &m); err == nil {
				if pub, ok := m["published"]; ok {
					ports[name] = fmt.Sprintf("%v", pub)
					break
				}
			}
		}
	}
	return services, ports
}
