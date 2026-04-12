// Package catalog defines every service type that pilot knows about.
//
// The catalog is the single source of truth for:
//   - What services can be selected in the wizard
//   - Which providers exist for each service (container vs managed)
//   - What environment variables each managed provider requires
//   - What hints to show in .env.example
//
// Adding a new service or provider means adding it here.
// The wizard, AgentPrompt(), and .env.example generation all read from this catalog.
package catalog

// Category groups services by function.
type Category string

const (
	CategoryApp      Category = "app"
	CategoryDatabase Category = "database"
	CategoryCache    Category = "cache"
	CategoryQueue    Category = "queue"
	CategorySearch   Category = "search"
	CategoryStorage  Category = "storage"
	CategoryProxy    Category = "proxy"
)

// ProviderDef describes one hosting option for a service type.
type ProviderDef struct {
	// Key is the identifier used in pilot.yaml: "container" | "neon" | "supabase" | ...
	Key string

	// Label is shown to the user in the wizard.
	Label string

	// IsManaged is true for external services that require no container.
	IsManaged bool

	// EnvVars lists the environment variables this provider requires at runtime.
	// Used to generate .env.example placeholders and to instruct the agent.
	EnvVars []string

	// EnvHints maps each env var to a realistic example value.
	EnvHints map[string]string

	// DefaultImage is the Docker image for container providers.
	DefaultImage string
}

// ServiceDef describes a service type in the catalog.
type ServiceDef struct {
	// Key is the identifier used in pilot.yaml (e.g. "postgres", "redis").
	Key string

	// Label is shown in the wizard list.
	Label string

	// Description is the subtitle shown in the wizard.
	Description string

	// Category groups the service by function.
	Category Category

	// CanBeManaged indicates whether this service can be hosted externally.
	// Only services with CanBeManaged=true appear in the "managed services" wizard step.
	CanBeManaged bool

	// Providers lists all known hosting options for this service.
	// The first provider is the default.
	Providers []ProviderDef
}

// Services is the ordered list of all services pilot knows about.
// Order determines the display order in the wizard.
var Services = []ServiceDef{
	{
		Key:          "app",
		Label:        "App",
		Description:  "your application service",
		Category:     CategoryApp,
		CanBeManaged: false,
		Providers: []ProviderDef{
			{Key: "container", Label: "Container", IsManaged: false},
		},
	},
	{
		Key:          "worker",
		Label:        "Worker",
		Description:  "background job process (same image, different CMD)",
		Category:     CategoryApp,
		CanBeManaged: false,
		Providers: []ProviderDef{
			{Key: "container", Label: "Container", IsManaged: false},
		},
	},
	{
		Key:          "postgres",
		Label:        "PostgreSQL",
		Description:  "relational database — self-hosted or managed",
		Category:     CategoryDatabase,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "postgres:16-alpine",
			},
			{
				Key:       "neon",
				Label:     "Neon",
				IsManaged: true,
				EnvVars:   []string{"DATABASE_URL"},
				EnvHints: map[string]string{
					"DATABASE_URL": "postgresql://user:pass@ep-xxx.us-east-2.aws.neon.tech/dbname?sslmode=require",
				},
			},
			{
				Key:       "supabase",
				Label:     "Supabase",
				IsManaged: true,
				EnvVars:   []string{"DATABASE_URL", "SUPABASE_URL", "SUPABASE_ANON_KEY", "SUPABASE_SERVICE_ROLE_KEY"},
				EnvHints: map[string]string{
					"DATABASE_URL":              "postgresql://postgres:[password]@db.xxx.supabase.co:5432/postgres",
					"SUPABASE_URL":              "https://xxx.supabase.co",
					"SUPABASE_ANON_KEY":         "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
					"SUPABASE_SERVICE_ROLE_KEY": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
				},
			},
			{
				Key:       "railway",
				Label:     "Railway",
				IsManaged: true,
				EnvVars:   []string{"DATABASE_URL"},
				EnvHints: map[string]string{
					"DATABASE_URL": "postgresql://postgres:xxx@containers-us-west-xxx.railway.app:5432/railway",
				},
			},
		},
	},
	{
		Key:          "mysql",
		Label:        "MySQL / MariaDB",
		Description:  "relational database — self-hosted or managed",
		Category:     CategoryDatabase,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "mysql:8.0",
			},
			{
				Key:       "planetscale",
				Label:     "PlanetScale",
				IsManaged: true,
				EnvVars:   []string{"DATABASE_URL"},
				EnvHints: map[string]string{
					"DATABASE_URL": "mysql://xxx:pscale_pw_xxx@aws.connect.psdb.cloud/dbname?sslaccept=strict",
				},
			},
			{
				Key:       "railway",
				Label:     "Railway",
				IsManaged: true,
				EnvVars:   []string{"DATABASE_URL"},
				EnvHints: map[string]string{
					"DATABASE_URL": "mysql://root:xxx@containers-us-west-xxx.railway.app:3306/railway",
				},
			},
		},
	},
	{
		Key:          "mongodb",
		Label:        "MongoDB",
		Description:  "document store — self-hosted or managed",
		Category:     CategoryDatabase,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "mongo:7.0",
			},
			{
				Key:       "atlas",
				Label:     "MongoDB Atlas",
				IsManaged: true,
				EnvVars:   []string{"MONGODB_URI"},
				EnvHints: map[string]string{
					"MONGODB_URI": "mongodb+srv://user:pass@cluster0.xxx.mongodb.net/dbname?retryWrites=true&w=majority",
				},
			},
		},
	},
	{
		Key:          "redis",
		Label:        "Redis / Valkey",
		Description:  "cache / sessions / pub-sub — self-hosted or managed",
		Category:     CategoryCache,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "redis:7.2-alpine",
			},
			{
				Key:       "upstash",
				Label:     "Upstash",
				IsManaged: true,
				EnvVars:   []string{"REDIS_URL"},
				EnvHints: map[string]string{
					"REDIS_URL": "rediss://default:xxx@xxx-xxx.upstash.io:6379",
				},
			},
			{
				Key:       "railway",
				Label:     "Railway",
				IsManaged: true,
				EnvVars:   []string{"REDIS_URL"},
				EnvHints: map[string]string{
					"REDIS_URL": "redis://default:xxx@containers-us-west-xxx.railway.app:6379",
				},
			},
		},
	},
	{
		Key:          "rabbitmq",
		Label:        "RabbitMQ",
		Description:  "message broker — self-hosted or managed",
		Category:     CategoryQueue,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "rabbitmq:3.13-management-alpine",
			},
			{
				Key:       "cloudamqp",
				Label:     "CloudAMQP",
				IsManaged: true,
				EnvVars:   []string{"RABBITMQ_URL"},
				EnvHints: map[string]string{
					"RABBITMQ_URL": "amqps://user:pass@xxx.rmq.cloudamqp.com/vhost",
				},
			},
		},
	},
	{
		Key:          "nats",
		Label:        "NATS",
		Description:  "lightweight message bus — self-hosted or managed",
		Category:     CategoryQueue,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "nats:2.10-alpine",
			},
			{
				Key:       "synadia",
				Label:     "Synadia Cloud",
				IsManaged: true,
				EnvVars:   []string{"NATS_URL", "NATS_CREDS"},
				EnvHints: map[string]string{
					"NATS_URL":   "nats://connect.ngs.global",
					"NATS_CREDS": "/path/to/ngs.creds",
				},
			},
		},
	},
	{
		Key:          "kafka",
		Label:        "Kafka / Redpanda",
		Description:  "event streaming — self-hosted or managed",
		Category:     CategoryQueue,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "confluentinc/cp-kafka:7.6.0",
			},
			{
				Key:       "confluent",
				Label:     "Confluent Cloud",
				IsManaged: true,
				EnvVars:   []string{"KAFKA_BROKERS", "KAFKA_USERNAME", "KAFKA_PASSWORD"},
				EnvHints: map[string]string{
					"KAFKA_BROKERS":  "xxx.us-east-2.aws.confluent.cloud:9092",
					"KAFKA_USERNAME": "xxx",
					"KAFKA_PASSWORD": "xxx",
				},
			},
			{
				Key:       "upstash",
				Label:     "Upstash Kafka",
				IsManaged: true,
				EnvVars:   []string{"KAFKA_BROKERS", "KAFKA_USERNAME", "KAFKA_PASSWORD"},
				EnvHints: map[string]string{
					"KAFKA_BROKERS":  "xxx.upstash.io:9092",
					"KAFKA_USERNAME": "xxx",
					"KAFKA_PASSWORD": "xxx",
				},
			},
		},
	},
	{
		Key:          "elasticsearch",
		Label:        "Elasticsearch / OpenSearch",
		Description:  "full-text search — self-hosted or managed",
		Category:     CategorySearch,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Self-hosted container",
				IsManaged:    false,
				DefaultImage: "elasticsearch:8.13.4",
			},
			{
				Key:       "elastic-cloud",
				Label:     "Elastic Cloud",
				IsManaged: true,
				EnvVars:   []string{"ELASTICSEARCH_URL", "ELASTIC_API_KEY"},
				EnvHints: map[string]string{
					"ELASTICSEARCH_URL": "https://xxx.es.us-east-1.aws.elastic.cloud:443",
					"ELASTIC_API_KEY":   "xxx==",
				},
			},
		},
	},
	{
		Key:          "storage",
		Label:        "Object Storage",
		Description:  "S3-compatible — self-hosted (MinIO) or managed",
		Category:     CategoryStorage,
		CanBeManaged: true,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "MinIO (self-hosted)",
				IsManaged:    false,
				DefaultImage: "minio/minio:RELEASE.2024-03-15T01-07-19Z",
			},
			{
				Key:       "cloudflare-r2",
				Label:     "Cloudflare R2",
				IsManaged: true,
				EnvVars:   []string{"R2_BUCKET", "R2_ACCOUNT_ID", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY", "R2_ENDPOINT"},
				EnvHints: map[string]string{
					"R2_BUCKET":           "my-app-bucket",
					"R2_ACCOUNT_ID":       "xxx",
					"R2_ACCESS_KEY_ID":    "xxx",
					"R2_SECRET_ACCESS_KEY": "xxx",
					"R2_ENDPOINT":         "https://<account_id>.r2.cloudflarestorage.com",
				},
			},
			{
				Key:       "s3",
				Label:     "AWS S3",
				IsManaged: true,
				EnvVars:   []string{"AWS_S3_BUCKET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"},
				EnvHints: map[string]string{
					"AWS_S3_BUCKET": "my-app-bucket",
					"AWS_REGION":    "us-east-1",
				},
			},
			{
				Key:       "backblaze-b2",
				Label:     "Backblaze B2",
				IsManaged: true,
				EnvVars:   []string{"B2_BUCKET", "B2_APPLICATION_KEY_ID", "B2_APPLICATION_KEY", "B2_ENDPOINT"},
				EnvHints: map[string]string{
					"B2_ENDPOINT": "https://s3.us-west-002.backblazeb2.com",
				},
			},
		},
	},
	{
		Key:          "nginx",
		Label:        "Nginx",
		Description:  "reverse proxy / static files",
		Category:     CategoryProxy,
		CanBeManaged: false,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Container",
				IsManaged:    false,
				DefaultImage: "nginx:1.25-alpine",
			},
		},
	},
	{
		Key:          "traefik",
		Label:        "Traefik",
		Description:  "reverse proxy with automatic TLS",
		Category:     CategoryProxy,
		CanBeManaged: false,
		Providers: []ProviderDef{
			{
				Key:          "container",
				Label:        "Container",
				IsManaged:    false,
				DefaultImage: "traefik:v3.0",
			},
		},
	},
}

// ── Lookup helpers ────────────────────────────────────────────────────────────

// Get returns the ServiceDef for a given key, or false if not found.
func Get(key string) (ServiceDef, bool) {
	for _, s := range Services {
		if s.Key == key {
			return s, true
		}
	}
	return ServiceDef{}, false
}

// GetProvider returns the ProviderDef for a given service key and provider key.
func GetProvider(serviceKey, providerKey string) (ProviderDef, bool) {
	svc, ok := Get(serviceKey)
	if !ok {
		return ProviderDef{}, false
	}
	for _, p := range svc.Providers {
		if p.Key == providerKey {
			return p, true
		}
	}
	return ProviderDef{}, false
}

// CanBeManaged returns true if the service supports managed hosting.
func CanBeManaged(serviceKey string) bool {
	svc, ok := Get(serviceKey)
	if !ok {
		return false
	}
	return svc.CanBeManaged
}

// ManagedProviders returns only the managed (non-container) providers for a service.
func ManagedProviders(serviceKey string) []ProviderDef {
	svc, ok := Get(serviceKey)
	if !ok {
		return nil
	}
	var managed []ProviderDef
	for _, p := range svc.Providers {
		if p.IsManaged {
			managed = append(managed, p)
		}
	}
	return managed
}

// EnvVarsFor returns the expected env vars for a service/provider combination.
// If provider is empty or "container", returns nil.
func EnvVarsFor(serviceKey, providerKey string) []string {
	p, ok := GetProvider(serviceKey, providerKey)
	if !ok || !p.IsManaged {
		return nil
	}
	return p.EnvVars
}

// EnvHintsFor returns the example values for a service/provider combination.
func EnvHintsFor(serviceKey, providerKey string) map[string]string {
	p, ok := GetProvider(serviceKey, providerKey)
	if !ok {
		return nil
	}
	return p.EnvHints
}

// DefaultImageFor returns the Docker image for a container service.
func DefaultImageFor(serviceKey string) string {
	p, ok := GetProvider(serviceKey, "container")
	if !ok {
		return ""
	}
	return p.DefaultImage
}
