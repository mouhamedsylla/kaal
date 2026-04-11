package vps

import (
	"encoding/json"
	"strings"

	domain "github.com/mouhamedsylla/pilot/internal/domain"
)

type remoteServiceEntry struct {
	Name   string `json:"Name"`
	State  string `json:"State"`
	Health string `json:"Health"`
	Image  string `json:"Image"`
}

func parseRemotePS(output string) []domain.ServiceStatus {
	var statuses []domain.ServiceStatus
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var entry remoteServiceEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		statuses = append(statuses, domain.ServiceStatus{
			Name:   entry.Name,
			State:  entry.State,
			Health: entry.Health,
		})
	}
	return statuses
}
