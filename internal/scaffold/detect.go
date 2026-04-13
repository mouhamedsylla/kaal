package scaffold

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mouhamedsylla/pilot/internal/scaffold/analyze"
)

// DetectedProject holds information inferred from an existing codebase.
type DetectedProject struct {
	Name            string
	Stack           string
	LanguageVersion string
	HasKaalYAML     bool
	IsExisting      bool // true if a known project file was found

	// Hints contains inferred service and hosting information.
	// Populated only for existing projects (IsExisting == true).
	// Used by the wizard to pre-fill the managed-services step.
	Hints *analyze.Hints
}

// Detect inspects dir for known project files and returns what it finds.
func Detect(dir string) DetectedProject {
	d := DetectedProject{
		Name: filepath.Base(dir),
	}

	if _, err := os.Stat(filepath.Join(dir, "pilot.yaml")); err == nil {
		d.HasKaalYAML = true
	}

	// Detect stack from project files.
	switch {
	case exists(dir, "go.mod"):
		d.Stack = "go"
		d.LanguageVersion = readGoVersion(dir)
		d.IsExisting = true
	case exists(dir, "package.json"):
		d.Stack = "node"
		d.LanguageVersion = readNodeVersion(dir)
		d.IsExisting = true
	case exists(dir, "Cargo.toml"):
		d.Stack = "rust"
		d.IsExisting = true
	case exists(dir, "pyproject.toml"), exists(dir, "requirements.txt"), exists(dir, "setup.py"):
		d.Stack = "python"
		d.LanguageVersion = readPythonVersion(dir)
		d.IsExisting = true
	case exists(dir, "pom.xml"), exists(dir, "build.gradle"):
		d.Stack = "java"
		d.IsExisting = true
	}

	// Run dependency and env analysis for existing projects.
	// For new (empty) directories this is a no-op — analyze.Analyze() skips
	// missing files silently.
	if d.IsExisting {
		d.Hints = analyze.Analyze(dir)
	}

	return d
}

func exists(dir, file string) bool {
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

// readNodeVersion détecte la version Node depuis .nvmrc, .node-version,
// ou le champ "engines.node" dans package.json.
func readNodeVersion(dir string) string {
	// 1. .nvmrc ou .node-version (ex: "20", "20.11.0", "lts/iron")
	for _, f := range []string{".nvmrc", ".node-version"} {
		if data, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
			v := strings.TrimSpace(string(data))
			// extrait le major : "20.11.0" → "20", "lts/iron" → ""
			if v != "" && !strings.HasPrefix(v, "lts/") {
				parts := strings.SplitN(v, ".", 2)
				return strings.TrimPrefix(parts[0], "v")
			}
		}
	}
	// 2. package.json engines.node (ex: ">=20.0.0")
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		// cherche "node": ">=X" ou "node": "X"
		re := regexp.MustCompile(`"node"\s*:\s*"[>=~^]*(\d+)`)
		if m := re.FindSubmatch(data); len(m) > 1 {
			return string(m[1])
		}
	}
	return "20"
}

// readPythonVersion détecte la version Python depuis .python-version,
// pyproject.toml (requires-python) ou runtime.txt.
func readPythonVersion(dir string) string {
	// 1. .python-version (ex: "3.12.2")
	if data, err := os.ReadFile(filepath.Join(dir, ".python-version")); err == nil {
		v := strings.TrimSpace(string(data))
		if v != "" {
			// garde major.minor uniquement : "3.12.2" → "3.12"
			parts := strings.SplitN(v, ".", 3)
			if len(parts) >= 2 {
				return parts[0] + "." + parts[1]
			}
			return v
		}
	}
	// 2. pyproject.toml requires-python (ex: ">=3.12", "~=3.11")
	if data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil {
		re := regexp.MustCompile(`requires-python\s*=\s*"[>=~^!]*(\d+\.\d+)`)
		if m := re.FindSubmatch(data); len(m) > 1 {
			return string(m[1])
		}
	}
	// 3. runtime.txt (Heroku/Render, ex: "python-3.12.2")
	if data, err := os.ReadFile(filepath.Join(dir, "runtime.txt")); err == nil {
		re := regexp.MustCompile(`python-(\d+\.\d+)`)
		if m := re.FindSubmatch(data); len(m) > 1 {
			return string(m[1])
		}
	}
	return "3.12"
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
