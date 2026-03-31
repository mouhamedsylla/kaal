package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	bold   = lipgloss.NewStyle().Bold(true)
	dim    = lipgloss.NewStyle().Faint(true)
)

func Success(msg string) { fmt.Println(green.Render("✓ " + msg)) }
func Error(msg string)   { fmt.Fprintln(os.Stderr, red.Render("✗ "+msg)) }
func Warn(msg string)    { fmt.Println(yellow.Render("⚠ " + msg)) }
func Info(msg string)    { fmt.Println(cyan.Render("→ " + msg)) }
func Dim(msg string)     { fmt.Println(dim.Render(msg)) }
func Bold(msg string)    { fmt.Println(bold.Render(msg)) }

// GreenText / RedText return coloured strings (no newline) for inline use.
func GreenText(s string) string { return green.Render(s) }
func RedText(s string) string   { return red.Render(s) }

// JSON prints v as indented JSON to stdout.
func JSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Fatal prints an error and exits with code 1.
func Fatal(err error) {
	Error(err.Error())
	os.Exit(1)
}

// IsTerminal reports whether stdout is connected to an interactive terminal.
// Returns false in CI pipelines, MCP mode (stdin/stdout = JSON-RPC pipe), or
// when output is redirected/piped. Used to decide whether to stream remote
// command output or silently capture it.
func IsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// prefixWriter is a line-buffered io.Writer that prepends a fixed prefix to
// every line. Handles \r\n and bare \r (e.g. docker pull progress output).
type prefixWriter struct {
	w      io.Writer
	prefix string
	buf    bytes.Buffer
}

func (pw *prefixWriter) Write(p []byte) (n int, err error) {
	// Normalise carriage returns so docker progress lines become clean lines.
	s := strings.ReplaceAll(string(p), "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	pw.buf.WriteString(s)

	for {
		line, rest, found := strings.Cut(pw.buf.String(), "\n")
		if !found {
			break
		}
		pw.buf.Reset()
		pw.buf.WriteString(rest)
		if strings.TrimSpace(line) != "" {
			fmt.Fprintf(pw.w, "%s%s\n", pw.prefix, line)
		}
	}
	return len(p), nil
}

// PrefixWriter returns an io.Writer that prepends prefix to every output line.
// Safe to use with docker build / docker pull output (handles \r progress lines).
func PrefixWriter(w io.Writer, prefix string) io.Writer {
	return &prefixWriter{w: w, prefix: prefix}
}
