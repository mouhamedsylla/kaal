package ui

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
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
