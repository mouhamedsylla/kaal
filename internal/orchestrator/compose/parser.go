package compose

import (
	"encoding/json"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/orchestrator"
)

// composePSEntry maps docker compose ps --format json output.
type composePSEntry struct {
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	State   string `json:"State"`
	Health  string `json:"Health"`
	Publishers []struct {
		PublishedPort int    `json:"PublishedPort"`
		Protocol      string `json:"Protocol"`
		URL           string `json:"URL"`
	} `json:"Publishers"`
	CreatedAt string `json:"CreatedAt"`
}

func parseComposePS(data []byte) ([]orchestrator.ServiceStatus, error) {
	// docker compose ps --format json outputs one JSON object per line (NDJSON)
	var statuses []orchestrator.ServiceStatus
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry composePSEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		var ports []string
		for _, p := range entry.Publishers {
			if p.PublishedPort > 0 {
				ports = append(ports, p.URL)
			}
		}
		statuses = append(statuses, orchestrator.ServiceStatus{
			Name:    entry.Name,
			Image:   entry.Image,
			State:   entry.State,
			Health:  entry.Health,
			Ports:   ports,
			Created: entry.CreatedAt,
		})
	}
	return statuses, nil
}
