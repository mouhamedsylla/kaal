package analyze

import (
	"bufio"
	"os"
	"strings"
)

// scanEnvFiles reads all candidate env files and extracts service hints
// from URL values and known variable names.
func scanEnvFiles(dir string, h *Hints) {
	// Collect all env vars across all files.
	// Later files don't overwrite earlier ones (higher confidence first).
	vars := make(map[string]envEntry) // varName → {value, file}

	for _, path := range envFileCandidates(dir) {
		parseEnvFile(path, vars)
	}

	if len(vars) == 0 {
		return
	}

	// Pass 1 — analyse URL values (highest confidence).
	for name, entry := range vars {
		if hints := hintsFromURL(name, entry.value, entry.file); hints != nil {
			for _, hint := range hints {
				h.addHint(hint)
			}
		}
	}

	// Pass 2 — analyse var names alone (medium confidence, fills gaps).
	for name, entry := range vars {
		if hints := hintsFromVarName(name, entry.file, vars); hints != nil {
			for _, hint := range hints {
				h.addHint(hint)
			}
		}
	}
}

// envEntry holds a parsed env var.
type envEntry struct {
	value string
	file  string // basename of the source file
}

// parseEnvFile reads a .env file and populates vars.
// Existing entries are not overwritten (earlier files take priority).
func parseEnvFile(path string, vars map[string]envEntry) {
	f, err := os.Open(path)
	if err != nil {
		return // file doesn't exist or can't be read — silently skip
	}
	defer f.Close()

	base := baseName(path)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Support both KEY=VALUE and export KEY=VALUE
		line = strings.TrimPrefix(line, "export ")
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`) // strip optional quotes

		if key == "" {
			continue
		}
		if _, exists := vars[key]; !exists {
			vars[key] = envEntry{value: val, file: base}
		}
	}
}

// ── URL analysis ──────────────────────────────────────────────────────────────

// urlProviderRule maps a hostname substring to (serviceKey, providerKey).
type urlProviderRule struct {
	hostFragment string
	serviceKey   string
	providerKey  string
}

// urlRules is ordered: more specific rules first.
var urlRules = []urlProviderRule{
	// PostgreSQL managed
	{"neon.tech", "postgres", "neon"},
	{"supabase.co", "postgres", "supabase"},
	{"railway.app", "", "railway"},      // railway hosts many service types
	{"render.com", "", "container"},     // render is container-based
	{"planetscale.com", "mysql", "planetscale"},

	// Redis managed
	{"upstash.io", "redis", "upstash"},

	// MongoDB managed
	{"mongodb.net", "mongodb", "atlas"},

	// Message brokers managed
	{"rmq.cloudamqp.com", "rabbitmq", "cloudamqp"},
	{"confluent.cloud", "kafka", "confluent"},

	// Search managed
	{"elastic.cloud", "elasticsearch", "elastic-cloud"},
	{"opensearch.amazonaws.com", "elasticsearch", "elastic-cloud"},

	// Storage managed
	{"r2.cloudflarestorage.com", "storage", "cloudflare-r2"},
	{"amazonaws.com", "storage", "s3"},
	{"backblazeb2.com", "storage", "backblaze-b2"},

	// Local — container
	{"localhost", "", "container"},
	{"127.0.0.1", "", "container"},
	{"::1", "", "container"},
}

// urlSchemeToService maps URL schemes to service keys.
var urlSchemeToService = map[string]string{
	"postgresql": "postgres",
	"postgres":   "postgres",
	"mysql":      "mysql",
	"mongodb":    "mongodb",
	"mongodb+srv": "mongodb",
	"redis":      "redis",
	"rediss":     "redis",
	"amqp":       "rabbitmq",
	"amqps":      "rabbitmq",
	"nats":       "nats",
}

// hintsFromURL extracts hints from a URL value.
// Returns nil if the value doesn't look like a URL.
func hintsFromURL(varName, value, sourceFile string) []ServiceHint {
	if value == "" || !looksLikeURL(value) {
		return nil
	}

	scheme, host := parseURLParts(value)

	// Determine service from scheme.
	serviceKey := urlSchemeToService[strings.ToLower(scheme)]

	// Scan host rules for provider.
	providerKey := ""
	for _, rule := range urlRules {
		if strings.Contains(strings.ToLower(host), rule.hostFragment) {
			providerKey = rule.providerKey
			if serviceKey == "" && rule.serviceKey != "" {
				serviceKey = rule.serviceKey
			}
			break
		}
	}

	if serviceKey == "" {
		return nil // can't determine service type from this URL
	}

	confidence := ConfidenceCertain
	if providerKey == "" {
		// We know the service type but not the provider.
		providerKey = "container"
		confidence = ConfidenceMedium
	}
	if providerKey == "container" {
		confidence = ConfidenceMedium
	}

	hint := ServiceHint{
		ServiceKey:   serviceKey,
		Provider:     providerKey,
		Confidence:   confidence,
		Evidence:     varName + " → " + hostFragment(host) + " in " + sourceFile,
		FoundEnvVars: []string{varName},
	}
	return []ServiceHint{hint}
}

// ── Var name analysis ─────────────────────────────────────────────────────────

// varNameRule maps an env var name (or prefix) to (serviceKey, providerKey, confidence).
type varNameRule struct {
	prefix     bool   // match as prefix rather than exact
	name       string
	serviceKey string
	providerKey string
	confidence float64
}

var varNameRules = []varNameRule{
	// Supabase — these vars unambiguously identify the provider
	{false, "SUPABASE_URL", "postgres", "supabase", ConfidenceHigh},
	{false, "SUPABASE_ANON_KEY", "postgres", "supabase", ConfidenceHigh},
	{false, "SUPABASE_SERVICE_ROLE_KEY", "postgres", "supabase", ConfidenceHigh},
	{true, "NEXT_PUBLIC_SUPABASE_", "postgres", "supabase", ConfidenceHigh},
	{true, "VITE_SUPABASE_", "postgres", "supabase", ConfidenceHigh},

	// Neon
	{true, "NEON_", "postgres", "neon", ConfidenceHigh},
	{false, "PGHOST", "postgres", "", ConfidenceLow},
	{false, "PGDATABASE", "postgres", "", ConfidenceLow},

	// Upstash
	{true, "UPSTASH_REDIS_", "redis", "upstash", ConfidenceHigh},
	{true, "UPSTASH_KAFKA_", "kafka", "upstash", ConfidenceHigh},

	// Cloudflare R2
	{false, "R2_BUCKET", "storage", "cloudflare-r2", ConfidenceHigh},
	{false, "R2_ACCOUNT_ID", "storage", "cloudflare-r2", ConfidenceHigh},
	{true, "CLOUDFLARE_R2_", "storage", "cloudflare-r2", ConfidenceHigh},

	// AWS S3
	{false, "AWS_S3_BUCKET", "storage", "s3", ConfidenceHigh},
	{false, "S3_BUCKET", "storage", "s3", ConfidenceMedium},

	// MongoDB Atlas
	{false, "MONGODB_URI", "mongodb", "", ConfidenceLow},
	{false, "MONGO_URI", "mongodb", "", ConfidenceLow},

	// Generic service vars (service known, provider unknown without URL)
	{false, "DATABASE_URL", "postgres", "", ConfidenceLow},
	{false, "REDIS_URL", "redis", "", ConfidenceLow},
	{false, "KAFKA_BROKERS", "kafka", "", ConfidenceLow},
	{false, "RABBITMQ_URL", "rabbitmq", "", ConfidenceLow},
	{false, "ELASTICSEARCH_URL", "elasticsearch", "", ConfidenceLow},
}

// hintsFromVarName extracts hints from a variable name when its value
// didn't already produce a hint (or to supplement with extra found vars).
func hintsFromVarName(varName, sourceFile string, allVars map[string]envEntry) []ServiceHint {
	for _, rule := range varNameRules {
		matched := false
		if rule.prefix {
			matched = strings.HasPrefix(varName, rule.name)
		} else {
			matched = varName == rule.name
		}
		if !matched {
			continue
		}

		// Collect all vars from allVars that match the same service
		// (used to pre-populate FoundEnvVars so the wizard knows what's already set).
		found := collectRelatedVars(rule.serviceKey, rule.providerKey, allVars)

		return []ServiceHint{{
			ServiceKey:   rule.serviceKey,
			Provider:     rule.providerKey,
			Confidence:   rule.confidence,
			Evidence:     varName + " in " + sourceFile,
			FoundEnvVars: found,
		}}
	}
	return nil
}

// collectRelatedVars returns var names from allVars that belong to a given
// service/provider pair (based on catalog env var definitions).
func collectRelatedVars(serviceKey, providerKey string, allVars map[string]envEntry) []string {
	if serviceKey == "" {
		return nil
	}

	// Build set of expected vars for this service/provider from catalog rules.
	expected := make(map[string]bool)
	for _, rule := range varNameRules {
		if rule.serviceKey == serviceKey &&
			(providerKey == "" || rule.providerKey == "" || rule.providerKey == providerKey) {
			expected[rule.name] = true
		}
	}

	var found []string
	for varName := range allVars {
		if expected[varName] {
			found = append(found, varName)
		}
	}
	return found
}

// ── string helpers ────────────────────────────────────────────────────────────

func looksLikeURL(s string) bool {
	for _, scheme := range []string{
		"postgresql://", "postgres://", "mysql://", "mongodb://", "mongodb+srv://",
		"redis://", "rediss://", "amqp://", "amqps://", "nats://", "https://", "http://",
	} {
		if strings.HasPrefix(strings.ToLower(s), scheme) {
			return true
		}
	}
	return false
}

// parseURLParts extracts scheme and host from a URL string without importing net/url.
func parseURLParts(rawURL string) (scheme, host string) {
	// scheme://[user:pass@]host[:port]/path
	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd < 0 {
		return "", rawURL
	}
	scheme = rawURL[:schemeEnd]
	rest := rawURL[schemeEnd+3:]

	// Strip user:pass@
	if atIdx := strings.LastIndex(rest, "@"); atIdx >= 0 {
		rest = rest[atIdx+1:]
	}

	// Strip /path
	if slashIdx := strings.IndexByte(rest, '/'); slashIdx >= 0 {
		rest = rest[:slashIdx]
	}

	// Strip :port
	// But be careful with IPv6 [::1]:5432
	if strings.HasPrefix(rest, "[") {
		if closeIdx := strings.IndexByte(rest, ']'); closeIdx >= 0 {
			host = rest[:closeIdx+1]
			return
		}
	}
	if colonIdx := strings.LastIndexByte(rest, ':'); colonIdx >= 0 {
		rest = rest[:colonIdx]
	}

	host = rest
	return
}

// hostFragment returns the last two labels of a hostname for display.
// e.g. "ep-xxx.us-east-2.aws.neon.tech" → "neon.tech"
func hostFragment(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return host
}

// baseName returns the last path component.
func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
