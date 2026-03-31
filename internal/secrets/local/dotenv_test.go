package local_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mouhamedsylla/pilot/internal/secrets/local"
)

func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, _ := os.Getwd()
	os.Chdir(dir)
	t.Cleanup(func() { os.Chdir(orig) })
}

func writeEnv(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGet_FromEnvironment(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "secret-value")
	sm := local.New()
	val, err := sm.Get(context.Background(), "TEST_SECRET_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret-value" {
		t.Errorf("Get = %q, want %q", val, "secret-value")
	}
}

func TestGet_NotFound(t *testing.T) {
	os.Unsetenv("KAAL_NO_SUCH_KEY_XYZ")
	sm := local.New()
	_, err := sm.Get(context.Background(), "KAAL_NO_SUCH_KEY_XYZ")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestInject_FromEnvFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeEnv(t, dir, ".env.dev", "DB_URL=postgres://localhost/dev\nAPI_KEY=abc123\n")

	sm := local.New()
	refs := map[string]string{
		"DATABASE_URL": "DB_URL",
		"MY_API_KEY":   "API_KEY",
	}
	result, err := sm.Inject(context.Background(), "dev", refs)
	if err != nil {
		t.Fatalf("Inject error: %v", err)
	}
	if result["DATABASE_URL"] != "postgres://localhost/dev" {
		t.Errorf("DATABASE_URL = %q, want %q", result["DATABASE_URL"], "postgres://localhost/dev")
	}
	if result["MY_API_KEY"] != "abc123" {
		t.Errorf("MY_API_KEY = %q, want %q", result["MY_API_KEY"], "abc123")
	}
}

func TestInject_FallsBackToEnvVar(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir) // no .env.dev file

	t.Setenv("FALLBACK_SECRET", "from-env")
	sm := local.New()
	result, err := sm.Inject(context.Background(), "dev", map[string]string{
		"MY_VAR": "FALLBACK_SECRET",
	})
	if err != nil {
		t.Fatalf("Inject error: %v", err)
	}
	if result["MY_VAR"] != "from-env" {
		t.Errorf("MY_VAR = %q, want %q", result["MY_VAR"], "from-env")
	}
}

func TestInject_MissingRefReturnsError(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeEnv(t, dir, ".env.dev", "EXISTING=value\n")
	os.Unsetenv("MISSING_REF")

	sm := local.New()
	_, err := sm.Inject(context.Background(), "dev", map[string]string{
		"MY_VAR": "MISSING_REF",
	})
	if err == nil {
		t.Fatal("expected error for missing secret ref, got nil")
	}
}

func TestInject_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeEnv(t, dir, ".env.dev", `DB_URL="postgres://user:pass@host/db"`)

	sm := local.New()
	result, err := sm.Inject(context.Background(), "dev", map[string]string{
		"DATABASE_URL": "DB_URL",
	})
	if err != nil {
		t.Fatalf("Inject error: %v", err)
	}
	// Quotes should be stripped
	if result["DATABASE_URL"] != "postgres://user:pass@host/db" {
		t.Errorf("DATABASE_URL = %q, want unquoted value", result["DATABASE_URL"])
	}
}

func TestSetInFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.test")

	if err := local.SetInFile(path, "MY_KEY", "my-value"); err != nil {
		t.Fatalf("SetInFile error: %v", err)
	}

	vars, err := local.ListFile(path)
	if err != nil {
		t.Fatalf("ListFile error: %v", err)
	}
	if vars["MY_KEY"] != "my-value" {
		t.Errorf("MY_KEY = %q, want %q", vars["MY_KEY"], "my-value")
	}
}

func TestSetInFile_UpdatesExistingKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.test")
	writeEnv(t, dir, ".env.test", "FOO=old\nBAR=keep\n")

	if err := local.SetInFile(path, "FOO", "new"); err != nil {
		t.Fatalf("SetInFile error: %v", err)
	}

	vars, _ := local.ListFile(path)
	if vars["FOO"] != "new" {
		t.Errorf("FOO = %q, want \"new\"", vars["FOO"])
	}
	if vars["BAR"] != "keep" {
		t.Errorf("BAR = %q, want \"keep\"", vars["BAR"])
	}
}

func TestSetInFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.secret")

	if err := local.SetInFile(path, "K", "v"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestListFile_IgnoresCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := writeEnv(t, dir, ".env.dev", `
# this is a comment
FOO=bar

# another comment
BAZ=qux
`)

	vars, err := local.ListFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(vars) != 2 {
		t.Errorf("got %d keys, want 2: %v", len(vars), vars)
	}
}

func TestListFile_MissingFile(t *testing.T) {
	_, err := local.ListFile("/tmp/pilot-no-such-file-xyz.env")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
