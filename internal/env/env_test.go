package env_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mouhamedsylla/pilot/internal/env"
)

// chdir changes the working directory to dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

func TestActive_Override(t *testing.T) {
	got := env.Active("staging")
	if got != "staging" {
		t.Errorf("Active(staging) = %q, want %q", got, "staging")
	}
}

func TestActive_DefaultIsDev(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir) // no .pilot-current-env file

	got := env.Active("")
	if got != "dev" {
		t.Errorf("Active(\"\") = %q, want \"dev\"", got)
	}
}

func TestActive_ReadsStateFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(".pilot-current-env", []byte("prod\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got := env.Active("")
	if got != "prod" {
		t.Errorf("Active(\"\") = %q, want \"prod\"", got)
	}
}

func TestUse_WritesStateFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := env.Use("staging"); err != nil {
		t.Fatalf("Use(staging) error: %v", err)
	}

	data, err := os.ReadFile(".pilot-current-env")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "staging" {
		t.Errorf(".pilot-current-env = %q, want %q", string(data), "staging")
	}
}

func TestUse_EmptyEnvReturnsError(t *testing.T) {
	if err := env.Use(""); err == nil {
		t.Fatal("Use(\"\") should return an error")
	}
}

func TestCurrent_DefaultsToDevWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	got := env.Current()
	if got != "dev" {
		t.Errorf("Current() = %q, want \"dev\"", got)
	}
}

func TestCurrent_ReadsAfterUse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := env.Use("prod"); err != nil {
		t.Fatal(err)
	}
	got := env.Current()
	if got != "prod" {
		t.Errorf("Current() = %q, want \"prod\"", got)
	}
}

func TestStateFilePath_IsAbsolute(t *testing.T) {
	p := env.StateFilePath()
	if !filepath.IsAbs(p) {
		t.Errorf("StateFilePath() = %q is not absolute", p)
	}
}
