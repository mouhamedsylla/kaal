package tui

// buildServicesSelect returns the service multi-select list.
// The "app" entry is disabled (always selected) — it represents the user's own application.
func buildServicesSelect() MultiSelect {
	return NewMultiSelect([]MultiSelectItem{
		{Key: "app", Label: "App", Description: "your application service", Disabled: true, Selected: true},
		{Key: "postgres", Label: "PostgreSQL 16", Description: "relational database"},
		{Key: "mysql", Label: "MySQL 8", Description: "relational database"},
		{Key: "mongodb", Label: "MongoDB 7", Description: "document store"},
		{Key: "redis", Label: "Redis 7", Description: "cache / session / queue"},
		{Key: "rabbitmq", Label: "RabbitMQ 3", Description: "message broker"},
		{Key: "nats", Label: "NATS", Description: "lightweight message bus"},
		{Key: "nginx", Label: "Nginx", Description: "reverse proxy / static files"},
	})
}

// buildEnvsSelect returns the environment multi-select list.
// "dev" is disabled (always selected).
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
		{Key: "aws", Label: "AWS", Description: "ECS / EKS  (stub — coming soon)"},
		{Key: "gcp", Label: "GCP", Description: "Cloud Run / GKE  (stub — coming soon)"},
		{Key: "azure", Label: "Azure", Description: "Container Apps / AKS  (stub — coming soon)"},
		{Key: "do", Label: "DigitalOcean", Description: "App Platform / DOKS  (stub — coming soon)"},
		{Key: "hetzner", Label: "Hetzner Cloud", Description: "VPS + k3s  — best price/perf ratio"},
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
