package analyze

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// scanDependencies reads package.json, go.mod, requirements.txt, and pyproject.toml
// to infer which services are used. Detection is limited to driver-level packages —
// we never look for ORMs because an ORM doesn't reveal the hosting mode.
//
// These hints are lower confidence than env file analysis. They signal that a
// service type is used but say nothing about whether it's container or managed.
func scanDependencies(dir string, h *Hints) {
	scanPackageJSON(dir, h)
	scanGoMod(dir, h)
	scanPythonDeps(dir, h)
	scanCargoToml(dir, h)
}

// ── Node / package.json ───────────────────────────────────────────────────────

// nodeDriverRule maps a package name (or prefix) to a service key.
// Only drivers and client libraries — no ORMs.
type nodeDriverRule struct {
	prefix  bool
	pkg     string
	service string
}

var nodeDriverRules = []nodeDriverRule{
	// PostgreSQL drivers (ORM-agnostic)
	{false, "pg", "postgres"},
	{false, "postgres", "postgres"},
	{false, "@vercel/postgres", "postgres"},
	{false, "pg-native", "postgres"},

	// MySQL drivers
	{false, "mysql2", "mysql"},
	{false, "mysql", "mysql"},
	{false, "@planetscale/database", "mysql"},

	// MongoDB driver
	{false, "mongodb", "mongodb"},

	// Redis drivers (any client)
	{false, "redis", "redis"},
	{false, "ioredis", "redis"},
	{false, "@redis/client", "redis"},
	{false, "@upstash/redis", "redis"},

	// Message brokers
	{false, "amqplib", "rabbitmq"},
	{false, "amqp-connection-manager", "rabbitmq"},
	{false, "kafkajs", "kafka"},
	{false, "@confluentinc/kafka-javascript", "kafka"},
	{false, "nats", "nats"},

	// Search
	{false, "@elastic/elasticsearch", "elasticsearch"},
	{false, "@opensearch-project/opensearch", "elasticsearch"},

	// Storage
	{false, "@aws-sdk/client-s3", "storage"},
	{false, "aws-sdk", "storage"},
	{false, "@cloudflare/workers-types", "storage"}, // weak signal
	{prefix: true, pkg: "minio", service: "storage"},
}

// packageJSON is a minimal representation for dependency scanning.
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func scanPackageJSON(dir string, h *Hints) {
	data, err := os.ReadFile(joinPath(dir, "package.json"))
	if err != nil {
		return
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return
	}

	// Merge deps + devDeps.
	allDeps := make(map[string]bool)
	for name := range pkg.Dependencies {
		allDeps[name] = true
	}
	for name := range pkg.DevDependencies {
		allDeps[name] = true
	}

	for _, rule := range nodeDriverRules {
		if rule.prefix {
			for dep := range allDeps {
				if strings.HasPrefix(dep, rule.pkg) {
					h.addHint(ServiceHint{
						ServiceKey: rule.service,
						Provider:   "",
						Confidence: ConfidenceLow,
						Evidence:   dep + " in package.json",
					})
					break
				}
			}
		} else if allDeps[rule.pkg] {
			h.addHint(ServiceHint{
				ServiceKey: rule.service,
				Provider:   "",
				Confidence: ConfidenceLow,
				Evidence:   rule.pkg + " in package.json",
			})
		}
	}
}

// ── Go / go.mod ───────────────────────────────────────────────────────────────

// goModRule maps a module path fragment to a service key.
type goModRule struct {
	fragment string
	service  string
}

var goModRules = []goModRule{
	// PostgreSQL drivers
	{"jackc/pgx", "postgres"},
	{"lib/pq", "postgres"},
	{"jackc/pgconn", "postgres"},

	// MySQL drivers
	{"go-sql-driver/mysql", "mysql"},

	// MongoDB driver
	{"mongo-driver", "mongodb"},

	// Redis clients
	{"go-redis/redis", "redis"},
	{"redis/go-redis", "redis"},
	{"gomodule/redigo", "redis"},
	{"upstash/go-redis", "redis"},

	// Message brokers
	{"rabbitmq/amqp091-go", "rabbitmq"},
	{"streadway/amqp", "rabbitmq"},
	{"IBM/sarama", "kafka"},
	{"segmentio/kafka-go", "kafka"},
	{"confluentinc/confluent-kafka-go", "kafka"},
	{"nats-io/nats.go", "nats"},

	// Search
	{"elastic/go-elasticsearch", "elasticsearch"},
	{"opensearch-project/opensearch-go", "elasticsearch"},

	// Storage
	{"aws/aws-sdk-go-v2/service/s3", "storage"},
	{"aws/aws-sdk-go/service/s3", "storage"},
	{"minio/minio-go", "storage"},
}

func scanGoMod(dir string, h *Hints) {
	f, err := os.Open(joinPath(dir, "go.mod"))
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "require") && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "github.com") {
			// Only inspect require lines and their content.
			if !strings.Contains(line, "/") {
				continue
			}
		}
		for _, rule := range goModRules {
			if strings.Contains(line, rule.fragment) {
				h.addHint(ServiceHint{
					ServiceKey: rule.service,
					Provider:   "",
					Confidence: ConfidenceLow,
					Evidence:   rule.fragment + " in go.mod",
				})
				break
			}
		}
	}
}

// ── Python / requirements.txt + pyproject.toml ───────────────────────────────

// pyDepRule maps a package name fragment to a service key.
type pyDepRule struct {
	fragment string
	service  string
}

var pyDepRules = []pyDepRule{
	// PostgreSQL drivers (adapter, not ORM)
	{"psycopg2", "postgres"},
	{"psycopg", "postgres"},
	{"asyncpg", "postgres"},
	{"pg8000", "postgres"},

	// MySQL drivers
	{"mysqlclient", "mysql"},
	{"pymysql", "mysql"},
	{"aiomysql", "mysql"},

	// MongoDB driver
	{"pymongo", "mongodb"},
	{"motor", "mongodb"},

	// Redis clients
	{"redis", "redis"},
	{"aioredis", "redis"},
	{"upstash-redis", "redis"},

	// Message brokers
	{"pika", "rabbitmq"},          // pika = RabbitMQ client
	{"aio-pika", "rabbitmq"},
	{"confluent-kafka", "kafka"},
	{"kafka-python", "kafka"},
	{"nats-py", "nats"},

	// Search
	{"elasticsearch", "elasticsearch"},
	{"opensearch-py", "elasticsearch"},

	// Storage
	{"boto3", "storage"},
	{"aiobotocore", "storage"},
	{"minio", "storage"},
}

func scanPythonDeps(dir string, h *Hints) {
	lines := readAllLines(dir, "requirements.txt")
	lines = append(lines, readAllLines(dir, "pyproject.toml")...)

	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		// Strip version specifiers: psycopg2>=2.9 → psycopg2
		lower = strings.FieldsFunc(lower, func(r rune) bool {
			return r == '>' || r == '<' || r == '=' || r == '!' || r == '~' || r == '^' || r == ' '
		})[0]
		for _, rule := range pyDepRules {
			if strings.Contains(lower, rule.fragment) {
				h.addHint(ServiceHint{
					ServiceKey: rule.service,
					Provider:   "",
					Confidence: ConfidenceLow,
					Evidence:   rule.fragment + " in requirements",
				})
				break
			}
		}
	}
}

// ── Rust / Cargo.toml ─────────────────────────────────────────────────────────

type cargoRule struct {
	fragment string
	service  string
}

var cargoRules = []cargoRule{
	{"tokio-postgres", "postgres"},
	{"sqlx", "postgres"}, // sqlx supports multiple DBs but postgres is most common
	{"diesel", "postgres"},
	{"redis", "redis"},
	{"fred", "redis"},
	{"mongodb", "mongodb"},
	{"lapin", "rabbitmq"},   // lapin = AMQP client
	{"rdkafka", "kafka"},
	{"async-nats", "nats"},
	{"aws-sdk-s3", "storage"},
}

func scanCargoToml(dir string, h *Hints) {
	lines := readAllLines(dir, "Cargo.toml")
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, rule := range cargoRules {
			if strings.Contains(lower, rule.fragment) {
				h.addHint(ServiceHint{
					ServiceKey: rule.service,
					Provider:   "",
					Confidence: ConfidenceLow,
					Evidence:   rule.fragment + " in Cargo.toml",
				})
				break
			}
		}
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func readAllLines(dir, filename string) []string {
	f, err := os.Open(joinPath(dir, filename))
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}

func joinPath(dir, name string) string {
	if dir == "" || dir == "." {
		return name
	}
	return dir + "/" + name
}
