package scaffold

import (
	"os"
	"path/filepath"
)

// DetectedProject holds information inferred from an existing codebase.
type DetectedProject struct {
	Name            string
	Stack           string
	LanguageVersion string
	HasKaalYAML     bool
	IsExisting      bool // true if a known project file was found
}

// Detect inspects dir for known project files and returns what it finds.
func Detect(dir string) DetectedProject {
	d := DetectedProject{
		Name: filepath.Base(dir),
	}

	if _, err := os.Stat(filepath.Join(dir, "kaal.yaml")); err == nil {
		d.HasKaalYAML = true
	}

	// Detect stack from project files
	switch {
	case exists(dir, "go.mod"):
		d.Stack = "go"
		d.LanguageVersion = readGoVersion(dir)
		d.IsExisting = true
	case exists(dir, "package.json"):
		d.Stack = "node"
		d.LanguageVersion = "20"
		d.IsExisting = true
	case exists(dir, "Cargo.toml"):
		d.Stack = "rust"
		d.IsExisting = true
	case exists(dir, "pyproject.toml"), exists(dir, "requirements.txt"), exists(dir, "setup.py"):
		d.Stack = "python"
		d.LanguageVersion = "3.12"
		d.IsExisting = true
	case exists(dir, "pom.xml"), exists(dir, "build.gradle"):
		d.Stack = "java"
		d.IsExisting = true
	}

	return d
}

func exists(dir, file string) bool {
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

func readGoVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "1.23"
	}
	// parse "go X.XX" line
	for _, line := range splitLines(string(data)) {
		var version string
		if n, _ := parseGoDirective(line, &version); n {
			return version
		}
	}
	return "1.23"
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func parseGoDirective(line string, out *string) (bool, error) {
	var prefix, version string
	n, _ := splitTwo(line, &prefix, &version)
	if n && prefix == "go" && version != "" {
		*out = version
		return true, nil
	}
	return false, nil
}

func splitTwo(s string, a, b *string) (bool, error) {
	for i, c := range s {
		if c == ' ' || c == '\t' {
			*a = s[:i]
			rest := s[i+1:]
			// trim leading whitespace
			for len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t') {
				rest = rest[1:]
			}
			*b = rest
			return true, nil
		}
	}
	return false, nil
}
