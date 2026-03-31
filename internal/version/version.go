// Package version holds the build-time version string injected via ldflags.
package version

import "fmt"

// Version is set at build time via:
//
//	go build -ldflags "-X github.com/mouhamedsylla/pilot/internal/version.Version=v0.1.0"
//
// Falls back to "dev" for local builds without ldflags (e.g. go run .).
var Version = "dev"

// Commit is the git short SHA injected at build time.
var Commit = "unknown"

// BuildDate is the ISO-8601 build timestamp injected at build time.
var BuildDate = "unknown"

// String returns the full version string shown by pilot version / pilot update.
func String() string {
	if Version == "dev" {
		return "dev (local build — run 'make install' to stamp a version)"
	}
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
}
