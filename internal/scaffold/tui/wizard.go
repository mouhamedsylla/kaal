package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Step IDs.
const (
	stepName          = 0
	stepServices      = 1
	stepEnvs          = 2
	stepTarget        = 3
	stepVPSHost       = 4 // shown only when target type requires a host
	stepRegistry      = 5
	stepRegistryImage = 6
	stepConfirm       = 7
	stepCount         = 8
)

var stepLabels = [stepCount]string{
	"Project", "Services", "Environments", "Target", "Host", "Registry", "Image", "Confirm",
}

// ServiceItem is a service selected during the wizard.
type ServiceItem struct {
	Key  string
	Type string
}

// Result holds everything collected by the wizard.
type Result struct {
	Name              string
	Stack             string
	Services          []ServiceItem
	Environments      []string
	TargetType        string
	TargetHost        string // IP or hostname — empty if user skipped
	TargetHostSkipped bool   // true when user skipped the host step
	Registry          string
	RegistryImage     string // full image name e.g. ghcr.io/user/app
	Cancelled         bool
}

// DetectedInfo carries auto-detected project information passed to the wizard.
type DetectedInfo struct {
	Name            string
	Stack           string
	LanguageVersion string
	IsExisting      bool
}

// Model is the top-level Bubbletea model for the init wizard.
type Model struct {
	step            int
	detected        DetectedInfo
	nameInput       textinput.Model
	stackInput      textinput.Model
	imageInput      textinput.Model
	hostInput       textinput.Model
	hostAttempted   bool // true after first empty submit on stepVPSHost
	services        MultiSelect
	envs            MultiSelect
	targets         MultiSelect
	registries      MultiSelect
	result          Result
	quitting        bool
	err             string
}

func NewWizard(d DetectedInfo) Model {
	ni := textinput.New()
	ni.Placeholder = "my-app"
	ni.CharLimit = 64
	ni.SetValue(d.Name)
	ni.Focus()

	si := textinput.New()
	si.Placeholder = "go, node, python, rust, java..."
	si.CharLimit = 32
	if d.Stack != "" {
		si.SetValue(d.Stack)
	}

	ii := textinput.New()
	ii.Placeholder = "ghcr.io/your-user/your-app"
	ii.CharLimit = 128

	hi := textinput.New()
	hi.Placeholder = "1.2.3.4  or  vps.example.com"
	hi.CharLimit = 256

	return Model{
		detected:   d,
		nameInput:  ni,
		stackInput: si,
		imageInput: ii,
		hostInput:  hi,
		services:   buildServicesSelect(),
		envs:       buildEnvsSelect(),
		targets:    buildTargetsSelect(),
		registries: buildRegistriesSelect(),
	}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.err = ""
		switch msg.String() {
		case "ctrl+c":
			m.result.Cancelled = true
			m.quitting = true
			return m, tea.Quit
		case "esc":
			if m.step > 0 {
				m.step--
			}
			return m, nil
		case "enter":
			return m.advance()
		case "up", "k":
			sel := m.activeSelect()
			if sel != nil {
				sel.Up()
			}
			return m, nil
		case "down", "j":
			sel := m.activeSelect()
			if sel != nil {
				sel.Down()
			}
			return m, nil
		case " ", "tab":
			if m.step == stepServices || m.step == stepEnvs {
				sel := m.activeSelect()
				if sel != nil {
					sel.Toggle()
				}
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepVPSHost:
		m.hostInput, cmd = m.hostInput.Update(msg)
	case stepRegistryImage:
		m.imageInput, cmd = m.imageInput.Update(msg)
	case stepConfirm:
		m.stackInput, cmd = m.stackInput.Update(msg)
	}
	return m, cmd
}

func (m Model) advance() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.err = "project name cannot be empty"
			return m, nil
		}
		if strings.ContainsAny(name, " /\\:") {
			m.err = "no spaces or slashes allowed"
			return m, nil
		}
		m.result.Name = name
		m.step = stepServices

	case stepServices:
		keys := m.services.SelectedKeys()
		if len(keys) == 0 {
			m.err = "select at least your app service (space to toggle)"
			return m, nil
		}
		m.result.Services = nil
		for _, item := range m.services.Items {
			if item.Selected || item.Disabled {
				m.result.Services = append(m.result.Services, ServiceItem{
					Key:  item.Key,
					Type: item.Key,
				})
			}
		}
		m.step = stepEnvs

	case stepEnvs:
		keys := m.envs.SelectedKeys()
		if len(keys) == 0 {
			m.err = "select at least one environment"
			return m, nil
		}
		m.result.Environments = keys
		onlyDev := len(keys) == 1 && keys[0] == "dev"
		if onlyDev {
			m.step = stepRegistry
		} else {
			m.step = stepTarget
		}

	case stepTarget:
		if len(m.targets.Items) > 0 {
			m.result.TargetType = m.targets.Items[m.targets.Cursor].Key
		}
		if requiresHost(m.result.TargetType) {
			m.hostInput.Focus()
			m.step = stepVPSHost
		} else {
			m.step = stepRegistry
		}

	case stepVPSHost:
		host := strings.TrimSpace(m.hostInput.Value())
		if host == "" {
			if !m.hostAttempted {
				m.hostAttempted = true
				m.err = "host is required for deployment — press Enter again to skip (you can edit pilot.yaml later)"
				return m, nil
			}
			// Second attempt with empty → allow skip, warn in summary
			m.result.TargetHostSkipped = true
		}
		m.result.TargetHost = host
		m.step = stepRegistry

	case stepRegistry:
		if len(m.registries.Items) > 0 {
			m.result.Registry = m.registries.Items[m.registries.Cursor].Key
		}
		// Pre-fill image input with a sensible placeholder based on registry + project name.
		m.imageInput.SetValue(defaultImageSuggestion(m.result.Registry, m.result.Name))
		m.imageInput.Focus()
		m.step = stepRegistryImage

	case stepRegistryImage:
		img := strings.TrimSpace(m.imageInput.Value())
		if img == "" {
			m.err = "image name cannot be empty"
			return m, nil
		}
		if strings.Contains(img, "your-user") || strings.Contains(img, "YOUR_") {
			m.err = "replace the placeholder with your real username"
			return m, nil
		}
		m.result.RegistryImage = img
		m.step = stepConfirm
		m.stackInput.Focus()

	case stepConfirm:
		stack := strings.TrimSpace(m.stackInput.Value())
		if stack == "" {
			stack = m.detected.Stack
		}
		m.result.Stack = stack
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// requiresHost returns true for target types that need an IP/hostname at deploy time.
func requiresHost(targetType string) bool {
	switch targetType {
	case "vps", "hetzner", "aws", "gcp", "azure", "do":
		return true
	}
	return false
}

// defaultImageSuggestion builds a pre-filled image name based on registry + project.
func defaultImageSuggestion(registry, projectName string) string {
	switch registry {
	case "ghcr":
		return "ghcr.io/your-user/" + projectName
	case "dockerhub":
		return "your-user/" + projectName
	default:
		return "registry.example.com/" + projectName
	}
}

func (m *Model) activeSelect() *MultiSelect {
	switch m.step {
	case stepServices:
		return &m.services
	case stepEnvs:
		return &m.envs
	case stepTarget:
		return &m.targets
	case stepRegistry:
		return &m.registries
	}
	return nil
}

// ──────────────────────────── View ────────────────────────────

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	return m.header() + "\n" + m.body() + "\n" + m.footer()
}

func (m Model) header() string {
	left := "  " + StyleTitle.Render("pilot init")
	right := m.progressBar()
	gap := width - visibleLen(left) - visibleLen(right)
	if gap < 1 {
		gap = 1
	}
	return StyleHeader.Render(left + strings.Repeat(" ", gap) + right)
}

func (m Model) progressBar() string {
	var parts []string
	for i := 0; i < stepCount; i++ {
		switch {
		case i < m.step:
			parts = append(parts, StyleSuccess.Render("✓"))
		case i == m.step:
			parts = append(parts, StyleStepActive.Render(fmt.Sprintf("%d %s", i+1, stepLabels[i])))
		default:
			parts = append(parts, StyleStep.Render(fmt.Sprintf("%d", i+1)))
		}
	}
	return strings.Join(parts, StyleDim.Render(" · "))
}

func (m Model) body() string {
	var content string
	switch m.step {
	case stepName:
		content = m.viewName()
	case stepServices:
		content = m.viewServices()
	case stepEnvs:
		content = m.viewEnvs()
	case stepTarget:
		content = m.viewSingleSelect(&m.targets, "Where do you deploy?")
	case stepVPSHost:
		content = m.viewVPSHost()
	case stepRegistry:
		content = m.viewSingleSelect(&m.registries, "Container registry")
	case stepRegistryImage:
		content = m.viewRegistryImage()
	case stepConfirm:
		content = m.viewConfirm()
	}

	if m.err != "" {
		content += "\n" + StyleError.Render("  ✗ "+m.err)
	}
	return content
}

func (m Model) viewName() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Project name") + "\n")
	b.WriteString("  " + m.nameInput.View() + "\n")
	if m.detected.IsExisting {
		info := fmt.Sprintf("  ✓ %s project detected", m.detected.Stack)
		if m.detected.LanguageVersion != "" {
			info += " " + m.detected.LanguageVersion
		}
		b.WriteString("\n" + StyleDetected.Render(info) + "\n")
	}
	return b.String()
}

func (m Model) viewServices() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("What does your app need?") + "\n")
	b.WriteString("  " + StyleDim.Render("space to toggle") + "\n\n")
	for _, line := range strings.Split(strings.TrimRight(m.services.View(), "\n"), "\n") {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m Model) viewEnvs() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Environments") + "\n")
	b.WriteString("  " + StyleDim.Render("space to toggle") + "\n\n")
	for _, line := range strings.Split(strings.TrimRight(m.envs.View(), "\n"), "\n") {
		b.WriteString("  " + line + "\n")
	}
	return b.String()
}

func (m Model) viewSingleSelect(sel *MultiSelect, title string) string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render(title) + "\n\n")
	for i, item := range sel.Items {
		cursor := "  "
		if i == sel.Cursor {
			cursor = StyleCursor.Render("▶ ")
		}
		label := StyleDim.Render(item.Label)
		if i == sel.Cursor {
			label = StyleSelected.Render(item.Label)
		}
		desc := ""
		if item.Description != "" {
			desc = "  " + StyleDim.Render(item.Description)
		}
		b.WriteString(fmt.Sprintf("  %s%s%s\n", cursor, label, desc))
	}
	return b.String()
}

func (m Model) viewVPSHost() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Target host") + "\n")
	b.WriteString("  " + StyleDim.Render("IP address or hostname of your VPS — needed to deploy") + "\n\n")
	b.WriteString("  " + m.hostInput.View() + "\n\n")
	b.WriteString("  " + StyleDim.Render("user: deploy   key: ~/.ssh/id_pilot   port: 22  (edit in pilot.yaml to change)") + "\n")
	return b.String()
}

func (m Model) viewRegistryImage() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Image name") + "\n")
	hint := "Full image path — replace with your real username"
	switch m.result.Registry {
	case "ghcr":
		hint = "e.g.  ghcr.io/mouhamedsylla/my-app"
	case "dockerhub":
		hint = "e.g.  mouhamedsylla/my-app"
	}
	b.WriteString("  " + StyleDim.Render(hint) + "\n\n")
	b.WriteString("  " + m.imageInput.View() + "\n")
	return b.String()
}

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Stack / language") + "\n")
	b.WriteString("  " + m.stackInput.View() + "\n\n")
	b.WriteString("  " + StyleSubtitle.Render("Summary") + "\n\n")

	row := func(k, v string) string {
		return fmt.Sprintf("  %s  %s\n",
			StyleDim.Render(fmt.Sprintf("%-16s", k)),
			StyleSelected.Render(v),
		)
	}
	b.WriteString(row("project", m.result.Name))
	b.WriteString(row("environments", strings.Join(m.result.Environments, ", ")))
	if m.result.TargetType != "" {
		targetSummary := m.result.TargetType
		if m.result.TargetHost != "" {
			targetSummary += "  " + m.result.TargetHost
		} else if m.result.TargetHostSkipped {
			targetSummary += "  " + StyleError.Render("⚠ host not set")
		}
		b.WriteString(row("target", targetSummary))
	}
	b.WriteString(row("registry", m.result.Registry))
	if m.result.RegistryImage != "" {
		b.WriteString(row("image", m.result.RegistryImage))
	}
	names := make([]string, 0, len(m.result.Services))
	for _, s := range m.result.Services {
		names = append(names, s.Key)
	}
	b.WriteString(row("services", strings.Join(names, ", ")))
	b.WriteString("\n  " + StyleDim.Render("Press Enter to generate pilot.yaml") + "\n")
	return b.String()
}

func (m Model) footer() string {
	var hint string
	switch m.step {
	case stepServices, stepEnvs:
		hint = "↑/↓ navigate   space toggle   enter confirm   esc back   ctrl+c cancel"
	case stepName, stepRegistryImage, stepConfirm:
		hint = "enter confirm   esc back   ctrl+c cancel"
	case stepVPSHost:
		hint = "enter confirm   esc back   ctrl+c cancel   (leave empty + Enter twice to skip)"
	default:
		hint = "↑/↓ navigate   enter select   esc back   ctrl+c cancel"
	}
	return StyleFooter.Render("  " + hint)
}

// visibleLen returns the printable length of a string (strips ANSI codes).
func visibleLen(s string) int {
	var n int
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// Run starts the wizard and returns the collected Result.
func Run(detected DetectedInfo) (Result, error) {
	m := NewWizard(detected)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return Result{}, fmt.Errorf("wizard: %w", err)
	}
	return final.(Model).result, nil
}
