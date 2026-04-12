package tui

import "github.com/mouhamedsylla/pilot/internal/scaffold/catalog"

// buildServicesSelect returns the service multi-select list driven by the catalog.
// "app" is always selected and disabled. All other entries are toggleable.
func buildServicesSelect() MultiSelect {
	var items []MultiSelectItem
	for _, svc := range catalog.Services {
		item := MultiSelectItem{
			Key:         svc.Key,
			Label:       svc.Label,
			Description: svc.Description,
		}
		if svc.Key == "app" {
			item.Disabled = true
			item.Selected = true
		}
		items = append(items, item)
	}
	return NewMultiSelect(items)
}

// buildManagedSelect returns a multi-select for the subset of selected services
// that can be hosted externally. The user toggles which ones are managed vs container.
//
// selectedKeys is the list of service keys chosen in the services step.
func buildManagedSelect(selectedKeys []string) MultiSelect {
	var items []MultiSelectItem
	for _, key := range selectedKeys {
		if !catalog.CanBeManaged(key) {
			continue
		}
		svc, ok := catalog.Get(key)
		if !ok {
			continue
		}
		items = append(items, MultiSelectItem{
			Key:         svc.Key,
			Label:       svc.Label,
			Description: buildManagedDescription(svc),
		})
	}
	return NewMultiSelect(items)
}

// buildManagedDescription builds the subtitle shown in the managed-services step.
// It lists the managed provider names so the user knows what "managed" means.
func buildManagedDescription(svc catalog.ServiceDef) string {
	providers := catalog.ManagedProviders(svc.Key)
	if len(providers) == 0 {
		return svc.Description
	}
	names := make([]string, 0, len(providers))
	for _, p := range providers {
		names = append(names, p.Label)
	}
	return joinStrings(names, " · ")
}

// buildProviderSelect returns a single-select list of managed providers
// for a given service. Used when the user has marked a service as managed
// and needs to pick which provider.
func buildProviderSelect(serviceKey string) MultiSelect {
	providers := catalog.ManagedProviders(serviceKey)
	items := make([]MultiSelectItem, 0, len(providers))
	for _, p := range providers {
		items = append(items, MultiSelectItem{
			Key:   p.Key,
			Label: p.Label,
		})
	}
	return NewMultiSelect(items)
}

// buildEnvsSelect returns the environment multi-select list.
// "dev" is always selected.
func buildEnvsSelect() MultiSelect {
	return NewMultiSelect([]MultiSelectItem{
		{Key: "dev", Label: "dev", Description: "local development", Disabled: true, Selected: true},
		{Key: "staging", Label: "staging", Description: "pre-production validation"},
		{Key: "prod", Label: "prod", Description: "production"},
		{Key: "test", Label: "test", Description: "automated testing / CI"},
		{Key: "preview", Label: "preview", Description: "ephemeral preview environments"},
	})
}

// buildTargetsSelect returns the deployment target single-select list.
func buildTargetsSelect() MultiSelect {
	return NewMultiSelect([]MultiSelectItem{
		{Key: "vps", Label: "VPS / bare-metal", Description: "SSH + Docker Compose  (Hetzner, DigitalOcean, OVH...)"},
		{Key: "hetzner", Label: "Hetzner Cloud", Description: "VPS + k3s  — best price/perf ratio"},
		{Key: "aws", Label: "AWS", Description: "ECS / EKS  (stub — coming soon)"},
		{Key: "gcp", Label: "GCP", Description: "Cloud Run / GKE  (stub — coming soon)"},
		{Key: "azure", Label: "Azure", Description: "Container Apps / AKS  (stub — coming soon)"},
		{Key: "do", Label: "DigitalOcean", Description: "App Platform / DOKS  (stub — coming soon)"},
	})
}

// buildRegistriesSelect returns the registry single-select list.
func buildRegistriesSelect() MultiSelect {
	return NewMultiSelect([]MultiSelectItem{
		{Key: "ghcr", Label: "GitHub Container Registry", Description: "ghcr.io — free for public repos"},
		{Key: "dockerhub", Label: "Docker Hub", Description: "docker.io — public / private"},
		{Key: "custom", Label: "Custom registry", Description: "self-hosted or any other registry"},
	})
}

// joinStrings joins a slice of strings with a separator (avoids importing strings).
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}
