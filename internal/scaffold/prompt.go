package scaffold

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

// step tracks which question we're on.
type step int

const (
	stepName step = iota
	stepStack
	stepRegistry
	stepDB
	stepDone
)

type promptModel struct {
	step      step
	opts      Options
	nameInput textinput.Model
	stackList list.Model
	regList   list.Model
	dbList    list.Model
	err       string
}

type listItem struct{ title, desc string }

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.desc }
func (i listItem) FilterValue() string { return i.title }

func newPromptModel(name string) promptModel {
	// Name input
	ti := textinput.New()
	ti.Placeholder = "my-app"
	ti.Focus()
	ti.CharLimit = 64
	if name != "" {
		ti.SetValue(name)
	}

	// Stack list
	stackItems := []list.Item{
		listItem{"go", "Go " + stackDefaults["go"].version},
		listItem{"node", "Node.js " + stackDefaults["node"].version},
		listItem{"python", "Python " + stackDefaults["python"].version},
		listItem{"rust", "Rust " + stackDefaults["rust"].version},
	}
	sl := newList(stackItems, "Stack")

	// Registry list
	regItems := []list.Item{
		listItem{"ghcr", "GitHub Container Registry — free for public repos"},
		listItem{"dockerhub", "Docker Hub — docker.io/username/image"},
		listItem{"custom", "Custom registry — self-hosted or other"},
	}
	rl := newList(regItems, "Registry")

	// DB list
	dbItems := []list.Item{
		listItem{"yes", "Yes — add PostgreSQL service to docker-compose"},
		listItem{"no", "No — I'll add services manually"},
	}
	dl := newList(dbItems, "PostgreSQL")

	return promptModel{
		step:      stepName,
		nameInput: ti,
		stackList: sl,
		regList:   rl,
		dbList:    dl,
	}
}

func newList(items []list.Item, title string) list.Model {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	d.Styles.SelectedTitle = selectedStyle
	d.Styles.SelectedDesc = dimStyle
	l := list.New(items, d, 60, len(items)*3+4)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.Styles.Title = titleStyle
	return l
}

func (m promptModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			return m.advance()
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepStack:
		m.stackList, cmd = m.stackList.Update(msg)
	case stepRegistry:
		m.regList, cmd = m.regList.Update(msg)
	case stepDB:
		m.dbList, cmd = m.dbList.Update(msg)
	}
	return m, cmd
}

func (m promptModel) advance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.err = "project name cannot be empty"
			return m, nil
		}
		if strings.ContainsAny(name, " /\\:") {
			m.err = "project name must not contain spaces or slashes"
			return m, nil
		}
		m.opts.Name = name
		m.err = ""
		m.step = stepStack
		m.nameInput.Blur()

	case stepStack:
		if item, ok := m.stackList.SelectedItem().(listItem); ok {
			m.opts.Stack = item.title
		}
		m.step = stepRegistry

	case stepRegistry:
		if item, ok := m.regList.SelectedItem().(listItem); ok {
			m.opts.Registry = item.title
		}
		m.step = stepDB

	case stepDB:
		if item, ok := m.dbList.SelectedItem().(listItem); ok {
			m.opts.HasDB = item.title == "yes"
		}
		m.step = stepDone
		return m, tea.Quit
	}
	return m, nil
}

func (m promptModel) View() string {
	var b strings.Builder

	header := titleStyle.Render("kaal init") + dimStyle.Render(" — scaffold a new project\n\n")
	b.WriteString(header)

	switch m.step {
	case stepName:
		b.WriteString(titleStyle.Render("Project name") + "\n")
		b.WriteString(m.nameInput.View() + "\n")
		if m.err != "" {
			b.WriteString(errorStyle.Render("  " + m.err) + "\n")
		}
		b.WriteString(dimStyle.Render("\nPress Enter to continue") + "\n")

	case stepStack:
		b.WriteString(m.stackList.View() + "\n")
		b.WriteString(dimStyle.Render("↑/↓ to navigate • Enter to select") + "\n")

	case stepRegistry:
		b.WriteString(m.regList.View() + "\n")
		b.WriteString(dimStyle.Render("↑/↓ to navigate • Enter to select") + "\n")

	case stepDB:
		b.WriteString(m.dbList.View() + "\n")
		b.WriteString(dimStyle.Render("↑/↓ to navigate • Enter to select") + "\n")
	}

	return b.String()
}

// RunPrompt runs the interactive TUI and returns the filled Options.
// If name is non-empty, the name step is pre-filled.
func RunPrompt(name string) (Options, error) {
	m := newPromptModel(name)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return Options{}, fmt.Errorf("prompt error: %w", err)
	}
	final := result.(promptModel)
	if final.opts.Name == "" {
		return Options{}, fmt.Errorf("init cancelled")
	}
	return final.opts, nil
}
