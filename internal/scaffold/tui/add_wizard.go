package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
)

// ── Add wizard steps ──────────────────────────────────────────────────────────

const (
	addStepType     = 0 // service type selection
	addStepHosting  = 1 // container vs managed (only if canBeManaged)
	addStepProvider = 2 // which managed provider (only if managed)
	addStepName     = 3 // service name in pilot.yaml
	addStepConfirm  = 4
	addStepCount    = 5
)

var addStepLabels = [addStepCount]string{
	"Type", "Hosting", "Provider", "Name", "Confirm",
}

// ── Add result ────────────────────────────────────────────────────────────────

// AddWizardResult holds everything collected by the add wizard.
type AddWizardResult struct {
	Type      string
	Hosting   string // "container" | "managed" | "local-only"
	Provider  string // catalog provider key
	Name      string // key in pilot.yaml
	Cancelled bool
}

// ── Add model ─────────────────────────────────────────────────────────────────

type addModel struct {
	step         int
	presetType   string // non-empty when type was passed as CLI arg
	typeSelect   MultiSelect
	hostingSelect MultiSelect
	providerSelect MultiSelect
	nameInput    textinput.Model
	result       AddWizardResult
	err          string
	quitting     bool
}

func newAddWizard(presetType string) addModel {
	ni := textinput.New()
	ni.CharLimit = 64
	if presetType != "" {
		ni.SetValue(presetType)
	}

	// Hosting options.
	hosting := NewMultiSelect([]MultiSelectItem{
		{Key: "container", Label: "Self-hosted container", Description: "runs on your VPS inside Docker"},
		{Key: "managed", Label: "Managed external service", Description: "SaaS — connects via env vars, no container"},
		{Key: "local-only", Label: "Local only", Description: "runs in dev, absent in staging / prod"},
	})

	m := addModel{
		presetType:    presetType,
		typeSelect:    buildAddTypeSelect(),
		hostingSelect: hosting,
		nameInput:     ni,
	}

	// If type is preset, skip the type selection step.
	if presetType != "" {
		m.result.Type = presetType
		if catalog.CanBeManaged(presetType) {
			m.step = addStepHosting
		} else {
			// Not manageable: skip hosting + provider steps.
			ni.SetValue(presetType)
			ni.Focus()
			m.step = addStepName
		}
	} else {
		m.step = addStepType
	}

	return m
}

func (m addModel) Init() tea.Cmd { return textinput.Blink }

// ── Update ────────────────────────────────────────────────────────────────────

func (m addModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				m.step = m.addPrevStep()
			}
			return m, nil

		case "enter":
			return m.addAdvance()

		case "up", "k":
			if sel := m.addActiveSelect(); sel != nil {
				sel.Up()
			}
			return m, nil

		case "down", "j":
			if sel := m.addActiveSelect(); sel != nil {
				sel.Down()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.step == addStepName {
		m.nameInput, cmd = m.nameInput.Update(msg)
	}
	return m, cmd
}

// ── Advance ───────────────────────────────────────────────────────────────────

func (m addModel) addAdvance() (tea.Model, tea.Cmd) {
	switch m.step {

	case addStepType:
		sel := m.typeSelect.Items[m.typeSelect.Cursor]
		m.result.Type = sel.Key
		m.nameInput.Placeholder = sel.Key
		if m.nameInput.Value() == "" {
			m.nameInput.SetValue(sel.Key)
		}

		if catalog.CanBeManaged(sel.Key) {
			m.step = addStepHosting
		} else {
			m.result.Hosting = "container"
			m.nameInput.Focus()
			m.step = addStepName
		}

	case addStepHosting:
		hosting := m.hostingSelect.Items[m.hostingSelect.Cursor].Key
		m.result.Hosting = hosting

		if hosting == "managed" {
			m.providerSelect = buildProviderSelect(m.result.Type)
			m.step = addStepProvider
		} else {
			m.nameInput.Focus()
			m.step = addStepName
		}

	case addStepProvider:
		provider := m.providerSelect.Items[m.providerSelect.Cursor].Key
		m.result.Provider = provider
		m.nameInput.Focus()
		m.step = addStepName

	case addStepName:
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			m.err = "service name cannot be empty"
			return m, nil
		}
		if strings.ContainsAny(name, " /\\:.") {
			m.err = "no spaces or special characters allowed"
			return m, nil
		}
		m.result.Name = name
		m.step = addStepConfirm

	case addStepConfirm:
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m addModel) addPrevStep() int {
	switch m.step {
	case addStepHosting:
		if m.presetType != "" {
			return addStepHosting // can't go back past preset
		}
		return addStepType
	case addStepProvider:
		return addStepHosting
	case addStepName:
		if m.result.Hosting == "managed" {
			return addStepProvider
		}
		if catalog.CanBeManaged(m.result.Type) {
			return addStepHosting
		}
		if m.presetType != "" {
			return addStepName // can't go back past preset
		}
		return addStepType
	case addStepConfirm:
		return addStepName
	}
	return m.step - 1
}

func (m *addModel) addActiveSelect() *MultiSelect {
	switch m.step {
	case addStepType:
		return &m.typeSelect
	case addStepHosting:
		return &m.hostingSelect
	case addStepProvider:
		return &m.providerSelect
	}
	return nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m addModel) View() string {
	if m.quitting {
		return ""
	}

	header := m.addHeader()
	var body string
	switch m.step {
	case addStepType:
		body = m.addViewSingleSelect(&m.typeSelect, "What service do you want to add?")
	case addStepHosting:
		svcDef, _ := catalog.Get(m.result.Type)
		body = m.addViewSingleSelect(&m.hostingSelect,
			fmt.Sprintf("How is %s hosted?", svcDef.Label))
	case addStepProvider:
		svcDef, _ := catalog.Get(m.result.Type)
		body = m.addViewSingleSelect(&m.providerSelect,
			fmt.Sprintf("Which %s provider?", svcDef.Label))
	case addStepName:
		body = m.addViewName()
	case addStepConfirm:
		body = m.addViewConfirm()
	}

	if m.err != "" {
		body += "\n" + StyleError.Render("  ✗ "+m.err)
	}

	footer := StyleFooter.Render("  ↑/↓ navigate   enter select   esc back   ctrl+c cancel")
	if m.step == addStepName || m.step == addStepConfirm {
		footer = StyleFooter.Render("  enter confirm   esc back   ctrl+c cancel")
	}

	return header + "\n" + body + "\n" + footer
}

func (m addModel) addHeader() string {
	left := "  " + StyleTitle.Render("pilot add")
	parts := make([]string, addStepCount)
	for i := 0; i < addStepCount; i++ {
		switch {
		case i < m.step:
			parts[i] = StyleSuccess.Render("✓")
		case i == m.step:
			parts[i] = StyleStepActive.Render(fmt.Sprintf("%d %s", i+1, addStepLabels[i]))
		default:
			parts[i] = StyleStep.Render(fmt.Sprintf("%d", i+1))
		}
	}
	right := strings.Join(parts, StyleDim.Render(" · "))
	gap := width - visibleLen(left) - visibleLen(right)
	if gap < 1 {
		gap = 1
	}
	return StyleHeader.Render(left + strings.Repeat(" ", gap) + right)
}

func (m addModel) addViewSingleSelect(sel *MultiSelect, title string) string {
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

func (m addModel) addViewName() string {
	svcDef, _ := catalog.Get(m.result.Type)
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Service name in pilot.yaml") + "\n")
	b.WriteString("  " + StyleDim.Render(fmt.Sprintf(
		"This is the key used in pilot.yaml services: section  (e.g. services.%s)",
		m.nameInput.Value(),
	)) + "\n\n")
	b.WriteString("  " + m.nameInput.View() + "\n\n")

	// Show env vars that will be added if managed.
	if m.result.Hosting == "managed" && m.result.Provider != "" {
		envVars := catalog.EnvVarsFor(m.result.Type, m.result.Provider)
		if len(envVars) > 0 {
			b.WriteString("  " + StyleDetected.Render(
				fmt.Sprintf("✓ %s will add to .env.example: %s",
					svcDef.Label, strings.Join(envVars, ", ")),
			) + "\n")
		}
	}
	return b.String()
}

func (m addModel) addViewConfirm() string {
	svcDef, _ := catalog.Get(m.result.Type)
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Summary") + "\n\n")

	row := func(k, v string) string {
		return fmt.Sprintf("  %s  %s\n",
			StyleDim.Render(fmt.Sprintf("%-12s", k)),
			StyleSelected.Render(v),
		)
	}

	b.WriteString(row("service", m.result.Name))
	b.WriteString(row("type", svcDef.Label))

	hostingLabel := m.result.Hosting
	if m.result.Hosting == "managed" && m.result.Provider != "" {
		if pDef, ok := catalog.GetProvider(m.result.Type, m.result.Provider); ok {
			hostingLabel = "managed → " + pDef.Label
		}
	}
	b.WriteString(row("hosting", hostingLabel))

	// Show what will be written.
	b.WriteString("\n  " + StyleDim.Render("Will write:") + "\n")
	b.WriteString("  " + StyleDim.Render("  • pilot.yaml — services."+m.result.Name) + "\n")
	if m.result.Hosting == "managed" {
		envVars := catalog.EnvVarsFor(m.result.Type, m.result.Provider)
		if len(envVars) > 0 {
			b.WriteString("  " + StyleDim.Render(
				fmt.Sprintf("  • .env.example — %s", strings.Join(envVars, ", ")),
			) + "\n")
		}
	}

	b.WriteString("\n  " + StyleDim.Render("Press Enter to confirm") + "\n")
	return b.String()
}

// ── Catalog helpers ───────────────────────────────────────────────────────────

// buildAddTypeSelect builds a single-select list of all catalog services
// except "app" (which can't be added — it's always there from init).
func buildAddTypeSelect() MultiSelect {
	var items []MultiSelectItem
	for _, svc := range catalog.Services {
		if svc.Key == "app" {
			continue
		}
		items = append(items, MultiSelectItem{
			Key:         svc.Key,
			Label:       svc.Label,
			Description: svc.Description,
		})
	}
	return NewMultiSelect(items)
}

// ── Run ───────────────────────────────────────────────────────────────────────

// RunAddWizard starts the add wizard and returns the collected result.
// presetType is the service type passed as a CLI argument (empty = show type selector).
func RunAddWizard(presetType string) (AddWizardResult, error) {
	m := newAddWizard(presetType)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return AddWizardResult{}, fmt.Errorf("add wizard: %w", err)
	}
	return final.(addModel).result, nil
}
