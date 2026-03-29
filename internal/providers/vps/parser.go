package vps

import (
	"encoding/json"
	"strings"

	"github.com/mouhamedsylla/kaal/internal/providers"
)

type remoteServiceEntry struct {
	Name   string `json:"Name"`
	State  string `json:"State"`
	Health string `json:"Health"`
	Image  string `json:"Image"`
}

func parseRemotePS(output string) []providers.ServiceStatus {
	var statuses []providers.ServiceStatus
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var entry remoteServiceEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		statuses = append(statuses, providers.ServiceStatus{
			Name:    entry.Name,
			State:   entry.State,
			Health:  entry.Health,
			Version: entry.Image,
		})
	}
	return statuses
}
