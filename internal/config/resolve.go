package config

// ServiceForEnv returns the effective Service config for a given environment,
// applying any service_overrides declared in the environment block.
//
// This is the single source of truth for "how does service X behave in env Y?"
// Use this everywhere instead of directly accessing cfg.Services[name].
//
// Example:
//
//	svc, ok := cfg.ServiceForEnv("postgres", "prod")
//	// svc.Hosting == "managed", svc.Provider == "neon"  (from service_overrides)
func (c *Config) ServiceForEnv(name, env string) (Service, bool) {
	svc, ok := c.Services[name]
	if !ok {
		return Service{}, false
	}

	envCfg, envOk := c.Environments[env]
	if !envOk || envCfg.ServiceOverrides == nil {
		return svc, true
	}

	override, hasOverride := envCfg.ServiceOverrides[name]
	if !hasOverride {
		return svc, true
	}

	// Apply only non-zero override fields — zero means "inherit from global"
	if override.Hosting != "" {
		svc.Hosting = override.Hosting
	}
	if override.Provider != "" {
		svc.Provider = override.Provider
	}
	if override.Version != "" {
		svc.Version = override.Version
	}
	if override.Image != "" {
		svc.Image = override.Image
	}
	if override.Port != 0 {
		svc.Port = override.Port
	}

	return svc, true
}

// ServicesForEnv returns all services with their effective config for a given environment.
// Managed services that are absent from compose (hosting=managed) are still returned —
// callers decide whether to include them.
func (c *Config) ServicesForEnv(env string) map[string]Service {
	result := make(map[string]Service, len(c.Services))
	for name := range c.Services {
		svc, _ := c.ServiceForEnv(name, env)
		result[name] = svc
	}
	return result
}
