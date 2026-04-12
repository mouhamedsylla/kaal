package analyze

import (
	"bufio"
	"os"
	"strings"
)

// scanMigrationTools detects the migration tool used by the project.
// Detection is based on known config files and directory structures.
// We never detect the ORM — only the migration runner.
func scanMigrationTools(dir string, h *Hints) {
	tool := detectMigrationTool(dir)
	if tool != "" {
		h.MigrationTool = tool
		h.HasMigrations = true
	}
}

// migrationRule describes a file or directory whose presence signals a tool.
type migrationRule struct {
	// path is relative to the project root. Can be a file or directory.
	path string
	// tool is the migration tool name.
	tool string
	// contentFragment is an optional string that must appear in the file content.
	// Empty means file presence alone is enough.
	contentFragment string
}

// migrationRules is ordered by specificity — most specific first.
var migrationRules = []migrationRule{
	// Prisma — schema file is definitive
	{"prisma/schema.prisma", "prisma", ""},

	// Drizzle — config file
	{"drizzle.config.ts", "drizzle", ""},
	{"drizzle.config.js", "drizzle", ""},
	{"drizzle.config.mjs", "drizzle", ""},

	// Alembic (Python)
	{"alembic.ini", "alembic", ""},
	{"alembic/env.py", "alembic", ""},

	// Flyway
	{"flyway.conf", "flyway", ""},
	{"flyway.toml", "flyway", ""},
	{"src/main/resources/db/migration", "flyway", ""}, // Java convention

	// Liquibase
	{"liquibase.properties", "liquibase", ""},
	{"liquibase.yml", "liquibase", ""},

	// golang-migrate (migrations directory convention)
	{"migrations", "golang-migrate", ""},
	{"db/migrations", "golang-migrate", ""},

	// Goose (check go.mod for the import)
	// Detected separately via go.mod scan below.

	// sql-migrate
	{"dbconfig.yml", "sql-migrate", ""},
	{"database.yml", "sql-migrate", ""},
}

// detectMigrationTool returns the first matching tool name, or "".
func detectMigrationTool(dir string) string {
	for _, rule := range migrationRules {
		path := joinPath(dir, rule.path)
		info, err := os.Stat(path)
		if err != nil {
			continue // not found
		}

		if rule.contentFragment == "" {
			// Presence is enough.
			return rule.tool
		}

		// Must also check content (only for regular files).
		if info.IsDir() {
			continue
		}
		if fileContains(path, rule.contentFragment) {
			return rule.tool
		}
	}

	// Fallback: scan go.mod for goose or golang-migrate imports.
	if tool := detectGoMigrationTool(dir); tool != "" {
		return tool
	}

	return ""
}

// goMigrationRule maps a go.mod import fragment to a migration tool name.
type goMigrationRule struct {
	fragment string
	tool     string
}

var goMigrationRules = []goMigrationRule{
	{"pressly/goose", "goose"},
	{"golang-migrate/migrate", "golang-migrate"},
	{"rubenv/sql-migrate", "sql-migrate"},
	{"amacneil/dbmate", "dbmate"},
	{"uptrace/bun/migrate", "bun-migrate"},
}

func detectGoMigrationTool(dir string) string {
	f, err := os.Open(joinPath(dir, "go.mod"))
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		for _, rule := range goMigrationRules {
			if strings.Contains(line, rule.fragment) {
				return rule.tool
			}
		}
	}
	return ""
}

// fileContains returns true if the file at path contains fragment.
func fileContains(path, fragment string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), fragment) {
			return true
		}
	}
	return false
}
