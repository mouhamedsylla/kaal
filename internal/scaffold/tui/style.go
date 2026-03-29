package tui

import "github.com/charmbracelet/lipgloss"

const width = 64

var (
	ColorPrimary = lipgloss.Color("6")   // cyan
	ColorSuccess = lipgloss.Color("2")   // green
	ColorMuted   = lipgloss.Color("240") // grey
	ColorError   = lipgloss.Color("1")   // red
	ColorAccent  = lipgloss.Color("5")   // magenta

	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	StyleSubtitle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleSelected = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleCursor = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	StyleDim = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(ColorError)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 2).
			Width(width)

	StyleHeader = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("238")).
			Width(width).
			Padding(0, 0, 1, 0)

	StyleFooter = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(lipgloss.Color("238")).
			Width(width).
			Padding(1, 0, 0, 0).
			Foreground(ColorMuted)

	StyleStep = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleStepActive = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	StyleTag = lipgloss.NewStyle().
			Background(lipgloss.Color("238")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

	StyleDetected = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Italic(true)
)
