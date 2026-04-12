// Package version holds the build-time version string injected via ldflags.
package version

import (
	"fmt"
	"strings"
)

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
	// Proper semver release (v1.2.3): show commit + build date.
	if strings.HasPrefix(Version, "v") {
		return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, BuildDate)
	}
	// git describe fallback (SHA or SHA-dirty): Version and Commit are the
	// same thing, so don't repeat them. Show dirty status in plain English.
	if strings.HasSuffix(Version, "-dirty") {
		return fmt.Sprintf("dev build — commit %s, built %s (uncommitted changes)", Commit, BuildDate)
	}
	return fmt.Sprintf("dev build — commit %s, built %s", Commit, BuildDate)
}
