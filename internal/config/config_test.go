package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mouhamedsylla/pilot/internal/config"
)

const validYAML = `
apiVersion: pilot/v1
project:
  name: test-app
  stack: go
  language_version: "1.23"
services:
  app:
    type: app
    port: 8080
  db:
    type: postgres
    version: "16"
  cache:
    type: redis
environments:
  dev:
    runtime: compose
    env_file: .env.dev
  prod:
    runtime: compose
    target: vps-prod
    env_file: .env.prod
targets:
  vps-prod:
    type: vps
    host: 1.2.3.4
    user: deploy
    key: ~/.ssh/id_pilot
registry:
  provider: ghcr
  image: ghcr.io/user/test-app
`

func writePilotYAML(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "pilot.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, validYAML)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Project.Name != "test-app" {
		t.Errorf("project.name = %q, want %q", cfg.Project.Name, "test-app")
	}
	if cfg.Project.Stack != "go" {
		t.Errorf("project.stack = %q, want %q", cfg.Project.Stack, "go")
	}
	if len(cfg.Services) != 3 {
		t.Errorf("services count = %d, want 3", len(cfg.Services))
	}
	if cfg.Registry.Provider != "ghcr" {
		t.Errorf("registry.provider = %q, want %q", cfg.Registry.Provider, "ghcr")
	}
}

func TestLoad_WalksUpToParent(t *testing.T) {
	root := t.TempDir()
	writePilotYAML(t, root, validYAML)

	// Load from a subdirectory — should walk up and find pilot.yaml.
	sub := filepath.Join(root, "cmd", "api")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(sub)
	if err != nil {
		t.Fatalf("walk-up failed: %v", err)
	}
	if cfg.Project.Name != "test-app" {
		t.Errorf("project.name = %q, want %q", cfg.Project.Name, "test-app")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing pilot.yaml, got nil")
	}
}

func TestValidate_MissingProjectName(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, `
apiVersion: pilot/v1
project:
  stack: go
services:
  app:
    type: app
    port: 8080
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for missing project.name")
	}
}

func TestValidate_InvalidRuntime(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, `
apiVersion: pilot/v1
project:
  name: app
services:
  app:
    type: app
    port: 8080
environments:
  dev:
    runtime: invalid-runtime
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for invalid runtime")
	}
}

func TestValidate_AppServiceMissingPort(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, `
apiVersion: pilot/v1
project:
  name: app
services:
  app:
    type: app
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for app service missing port")
	}
}

func TestValidate_UnknownTargetRef(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, `
apiVersion: pilot/v1
project:
  name: app
services:
  app:
    type: app
    port: 8080
environments:
  prod:
    target: nonexistent-target
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for unknown target reference")
	}
}

func TestValidate_WrongAPIVersion(t *testing.T) {
	dir := t.TempDir()
	writePilotYAML(t, dir, `
apiVersion: pilot/v2
project:
  name: app
services:
  app:
    type: app
    port: 8080
`)
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected validation error for wrong apiVersion")
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := writePilotYAML(t, dir, validYAML)

	cfg, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Project.Name = "updated-app"
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := config.LoadFromPath(path)
	if err != nil {
		t.Fatalf("reload after save failed: %v", err)
	}
	if reloaded.Project.Name != "updated-app" {
		t.Errorf("after save: project.name = %q, want %q", reloaded.Project.Name, "updated-app")
	}
}
