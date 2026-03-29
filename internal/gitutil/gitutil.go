// Package gitutil provides small helpers for interacting with the local git repo.
package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// ShortSHA returns the short SHA of HEAD (e.g. "abc1234").
func ShortSHA() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("could not determine git SHA (are you in a git repo?): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
