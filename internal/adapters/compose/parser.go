package compose

import (
	"encoding/json"
	"fmt"
	"strings"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

// composePSEntry maps docker compose ps --format json output.
type composePSEntry struct {
	Name   string `json:"Name"`
	Image  string `json:"Image"`
	State  string `json:"State"`
	Health string `json:"Health"`
	Publishers []struct {
		PublishedPort int    `json:"PublishedPort"`
		Protocol      string `json:"Protocol"`
		URL           string `json:"URL"`
	} `json:"Publishers"`
	CreatedAt string `json:"CreatedAt"`
}

func parseComposePS(data []byte) ([]domain.ServiceStatus, error) {
	// docker compose ps --format json outputs one JSON object per line (NDJSON)
	var statuses []domain.ServiceStatus
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry composePSEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		// Deduplicate by port number: docker reports the same published port
		// twice (once for 0.0.0.0 IPv4, once for :: IPv6). Show each port once.
		seen := map[int]bool{}
		var ports []string
		for _, p := range entry.Publishers {
			if p.PublishedPort > 0 && !seen[p.PublishedPort] {
				seen[p.PublishedPort] = true
				host := p.URL
				if host == "" || host == "::" {
					host = "0.0.0.0"
				}
				ports = append(ports, fmt.Sprintf("%s:%d", host, p.PublishedPort))
			}
		}
		statuses = append(statuses, domain.ServiceStatus{
			Name:  entry.Name,
			State: entry.State,
			Health: entry.Health,
		})
		_ = ports // ports not in domain.ServiceStatus — kept for future extension
	}
	return statuses, nil
}
