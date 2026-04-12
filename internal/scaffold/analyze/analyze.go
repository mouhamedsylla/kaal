// Package analyze inspects an existing project to infer which services it uses
// and how they are hosted (container vs managed external service).
//
// Priority order (highest confidence first):
//
//  1. Env file URL values — database URLs reveal the exact provider
//     e.g. DATABASE_URL=postgresql://...neon.tech → postgres managed by Neon
//
//  2. Known env var names — some vars unambiguously identify a provider
//     e.g. SUPABASE_URL → postgres managed by Supabase
//
//  3. Dependency files — only at driver level, not ORM level
//     e.g. "pg" in package.json → postgres is used (hosting unknown)
//
// The analyzer never guesses the ORM (Prisma, Drizzle, GORM, SQLAlchemy…)
// because ORMs don't reveal the hosting mode. The connection URL does.
package analyze

import "path/filepath"

// Confidence levels for service hints.
const (
	ConfidenceCertain = 1.00 // URL contains a known provider domain
	ConfidenceHigh    = 0.95 // var name unambiguously identifies provider
	ConfidenceMedium  = 0.85 // localhost in URL → probably container
	ConfidenceLow     = 0.50 // driver dep found, hosting unknown
)

// ServiceHint describes an inferred service and its probable provider.
type ServiceHint struct {
	// ServiceKey matches a key in catalog.Services (e.g. "postgres", "redis").
	ServiceKey string

	// Provider is the catalog provider key: "neon" | "supabase" | "container" | ...
	// Empty string means hosting is unknown (only service type was detected).
	Provider string

	// Confidence is a score between 0 and 1.
	Confidence float64

	// Evidence is a human-readable explanation shown in the wizard.
	// e.g. "DATABASE_URL → neon.tech in .env.example"
	Evidence string

	// FoundEnvVars lists the env var names already present in env files.
	// The wizard uses this to skip asking for vars the user already has.
	FoundEnvVars []string
}

// Hints is the result of analyzing a project directory.
type Hints struct {
	// Services contains one entry per detected service type.
	// Multiple hints for the same service are deduplicated: highest confidence wins.
	Services []ServiceHint

	// MigrationTool is the detected migration tool, if any.
	// Values: "prisma" | "drizzle" | "alembic" | "goose" | "flyway" | "sql-migrate"
	MigrationTool string

	// HasMigrations is true when a migration tool or migration directory was found.
	HasMigrations bool
}

// Analyze scans dir and returns inferred hints about services and hosting.
// It never returns an error — missing files are silently skipped.
func Analyze(dir string) *Hints {
	h := &Hints{}

	scanEnvFiles(dir, h)
	scanDependencies(dir, h)
	scanMigrationTools(dir, h)

	deduplicateServices(h)

	return h
}

// GetService returns the hint for a given service key, or nil if not found.
func (h *Hints) GetService(serviceKey string) *ServiceHint {
	for i := range h.Services {
		if h.Services[i].ServiceKey == serviceKey {
			return &h.Services[i]
		}
	}
	return nil
}

// HasService returns true if a hint exists for the given service key.
func (h *Hints) HasService(serviceKey string) bool {
	return h.GetService(serviceKey) != nil
}

// ── internals ─────────────────────────────────────────────────────────────────

// addHint adds a ServiceHint to h.Services.
// If a hint for the same service already exists with equal or higher confidence,
// the new hint is ignored. Otherwise it replaces the existing one.
func (h *Hints) addHint(hint ServiceHint) {
	for i, existing := range h.Services {
		if existing.ServiceKey == hint.ServiceKey {
			if hint.Confidence > existing.Confidence {
				h.Services[i] = hint
			}
			return
		}
	}
	h.Services = append(h.Services, hint)
}

// deduplicateServices ensures at most one hint per service key,
// keeping the one with the highest confidence.
// (addHint already does this incrementally; this is a final safety pass.)
func deduplicateServices(h *Hints) {
	seen := make(map[string]int) // key → index in h.Services
	var deduped []ServiceHint
	for _, hint := range h.Services {
		if idx, ok := seen[hint.ServiceKey]; ok {
			if hint.Confidence > deduped[idx].Confidence {
				deduped[idx] = hint
			}
		} else {
			seen[hint.ServiceKey] = len(deduped)
			deduped = append(deduped, hint)
		}
	}
	h.Services = deduped
}

// envFileCandidates returns the ordered list of env file paths to scan.
// Earlier files take priority (highest confidence first).
func envFileCandidates(dir string) []string {
	names := []string{
		".env.example",
		".env.local",
		".env",
		".env.development",
		".env.dev",
		".env.staging",
		".env.prod",
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(dir, name))
	}
	return paths
}
