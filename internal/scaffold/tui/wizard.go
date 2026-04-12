package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mouhamedsylla/pilot/internal/scaffold/analyze"
	"github.com/mouhamedsylla/pilot/internal/scaffold/catalog"
)

// ── Step IDs ──────────────────────────────────────────────────────────────────

const (
	stepName             = 0
	stepServices         = 1
	stepManagedServices  = 2 // shown only when ≥1 selected service canBeManaged
	stepEnvs             = 3
	stepTarget           = 4 // shown only when non-dev envs are selected
	stepVPSHost          = 5 // shown only when target type requires a host
	stepRegistry         = 6
	stepRegistryImage    = 7
	stepRegistryCreds    = 8 // shown only when registry credentials are missing
	stepConfirm          = 9
	stepCount            = 10
)

var stepLabels = [stepCount]string{
	"Project", "Services", "Hosting", "Environments",
	"Target", "Host", "Registry", "Image", "Credentials", "Confirm",
}

// ── Result types ──────────────────────────────────────────────────────────────

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
	ServiceHosting    map[string]string // serviceKey → "container" | "managed"
	ServiceProvider   map[string]string // serviceKey → provider key (e.g. "neon", "upstash")
	Environments      []string
	TargetType        string
	TargetHost        string
	TargetHostSkipped bool
	Registry          string
	RegistryImage     string
	RegistryCreds     map[string]string // varName → value (to write to .env.local)
	Cancelled         bool
}

// DetectedInfo carries auto-detected project information passed to the wizard.
type DetectedInfo struct {
	Name            string
	Stack           string
	LanguageVersion string
	IsExisting      bool
	Hints           *analyze.Hints // service + provider hints from env/dep analysis
}

// ── Model ─────────────────────────────────────────────────────────────────────

// Model is the top-level Bubbletea model for the init wizard.
type Model struct {
	step    int
	detected DetectedInfo

	// Text inputs
	nameInput   textinput.Model
	stackInput  textinput.Model
	imageInput  textinput.Model
	hostInput   textinput.Model
	credsInput  textinput.Model

	// Multi-selects
	services        MultiSelect
	managedServices MultiSelect // populated after stepServices
	envs            MultiSelect
	targets         MultiSelect
	registries      MultiSelect

	// Registry credentials collection
	credsKeys   []string          // ordered vars to collect
	credsValues map[string]string // collected values

	// Misc state
	hostAttempted bool
	result        Result
	quitting      bool
	err           string
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

	ci := textinput.New()
	ci.CharLimit = 256

	return Model{
		detected:    d,
		nameInput:   ni,
		stackInput:  si,
		imageInput:  ii,
		hostInput:   hi,
		credsInput:  ci,
		services:    buildServicesSelect(),
		envs:        buildEnvsSelect(),
		targets:     buildTargetsSelect(),
		registries:  buildRegistriesSelect(),
		credsValues: make(map[string]string),
		result: Result{
			ServiceHosting:  make(map[string]string),
			ServiceProvider: make(map[string]string),
			RegistryCreds:   make(map[string]string),
		},
	}
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

// ── Update ────────────────────────────────────────────────────────────────────

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
				m.step = m.prevStep(m.step)
			}
			return m, nil

		case "enter":
			return m.advance()

		case "up", "k":
			if sel := m.activeSelect(); sel != nil {
				sel.Up()
			}
			return m, nil

		case "down", "j":
			if sel := m.activeSelect(); sel != nil {
				sel.Down()
			}
			return m, nil

		case " ", "tab":
			switch m.step {
			case stepServices, stepEnvs, stepManagedServices:
				if sel := m.activeSelect(); sel != nil {
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
	case stepRegistryCreds:
		m.credsInput, cmd = m.credsInput.Update(msg)
	case stepConfirm:
		m.stackInput, cmd = m.stackInput.Update(msg)
	}
	return m, cmd
}

// ── Advance ───────────────────────────────────────────────────────────────────

func (m Model) advance() (tea.Model, tea.Cmd) {
	switch m.step {

	// ── 0 · Project name ─────────────────────────────────────────────────────
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

	// ── 1 · Services ─────────────────────────────────────────────────────────
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
		// Build managed-services select from the chosen services.
		managedKeys := managedCandidates(m.result.Services)
		if len(managedKeys) > 0 {
			m.managedServices = buildManagedSelect(managedKeys)
			// Pre-check services that hints suggest are managed.
			m.preFillManagedFromHints()
			m.step = stepManagedServices
		} else {
			m.step = stepEnvs
		}

	// ── 2 · Managed services ─────────────────────────────────────────────────
	case stepManagedServices:
		// Read which services are managed (checked) vs container (unchecked).
		for i, item := range m.managedServices.Items {
			_ = i
			if item.Selected {
				m.result.ServiceHosting[item.Key] = "managed"
				m.result.ServiceProvider[item.Key] = resolveProvider(item.Key, m.detected.Hints)
			} else {
				m.result.ServiceHosting[item.Key] = "container"
			}
		}
		m.step = stepEnvs

	// ── 3 · Environments ─────────────────────────────────────────────────────
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

	// ── 4 · Deploy target ────────────────────────────────────────────────────
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

	// ── 5 · VPS host ─────────────────────────────────────────────────────────
	case stepVPSHost:
		host := strings.TrimSpace(m.hostInput.Value())
		if host == "" {
			if !m.hostAttempted {
				m.hostAttempted = true
				m.err = "host is required for deployment — press Enter again to skip (you can edit pilot.yaml later)"
				return m, nil
			}
			m.result.TargetHostSkipped = true
		}
		m.result.TargetHost = host
		m.step = stepRegistry

	// ── 6 · Registry ─────────────────────────────────────────────────────────
	case stepRegistry:
		if len(m.registries.Items) > 0 {
			m.result.Registry = m.registries.Items[m.registries.Cursor].Key
		}
		m.imageInput.SetValue(defaultImageSuggestion(m.result.Registry, m.result.Name))
		m.imageInput.Focus()
		m.step = stepRegistryImage

	// ── 7 · Registry image ───────────────────────────────────────────────────
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

		// Show credentials step only for missing vars.
		m.credsKeys = missingCredsFor(m.result.Registry)
		if len(m.credsKeys) > 0 {
			m.prepareCredsInput(0)
			m.step = stepRegistryCreds
		} else {
			m.step = stepConfirm
			m.stackInput.Focus()
		}

	// ── 8 · Registry credentials ─────────────────────────────────────────────
	case stepRegistryCreds:
		// Store the current input value (empty = user skipped this var).
		current := strings.TrimSpace(m.credsInput.Value())
		if current != "" {
			m.credsValues[m.credsKeys[len(m.credsValues)]] = current
		}

		// Advance to next var or finish.
		nextIdx := len(m.credsValues)
		if nextIdx < len(m.credsKeys) {
			m.prepareCredsInput(nextIdx)
			return m, nil
		}

		// All vars collected — copy non-empty values to result.
		for k, v := range m.credsValues {
			if v != "" {
				m.result.RegistryCreds[k] = v
			}
		}
		m.step = stepConfirm
		m.stackInput.Focus()

	// ── 9 · Confirm ──────────────────────────────────────────────────────────
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

// ── Helpers ───────────────────────────────────────────────────────────────────

// prevStep returns the step before current, skipping conditional steps that
// weren't shown on the way forward.
func (m Model) prevStep(current int) int {
	switch current {
	case stepEnvs:
		if len(m.managedServices.Items) > 0 {
			return stepManagedServices
		}
		return stepServices
	case stepTarget:
		return stepEnvs
	case stepRegistry:
		if m.result.TargetHostSkipped || m.result.TargetHost != "" {
			return stepVPSHost
		}
		if m.result.TargetType != "" {
			return stepTarget
		}
		return stepEnvs
	case stepRegistryCreds:
		return stepRegistryImage
	case stepConfirm:
		if len(m.credsKeys) > 0 {
			return stepRegistryCreds
		}
		return stepRegistryImage
	}
	return current - 1
}

// managedCandidates returns service keys that can be hosted externally.
func managedCandidates(services []ServiceItem) []string {
	var keys []string
	for _, svc := range services {
		if catalog.CanBeManaged(svc.Key) {
			keys = append(keys, svc.Key)
		}
	}
	return keys
}

// preFillManagedFromHints pre-checks services that hints suggest are managed.
func (m *Model) preFillManagedFromHints() {
	if m.detected.Hints == nil {
		return
	}
	for i, item := range m.managedServices.Items {
		hint := m.detected.Hints.GetService(item.Key)
		if hint == nil {
			continue
		}
		// Pre-check when confidence is high enough and provider is known managed.
		if hint.Confidence >= 0.85 && hint.Provider != "" && hint.Provider != "container" {
			m.managedServices.Items[i].Selected = true
		}
	}
}

// resolveProvider returns the best provider key for a service.
// Prefers hints with high confidence, falls back to first managed provider in catalog.
func resolveProvider(serviceKey string, hints *analyze.Hints) string {
	if hints != nil {
		if hint := hints.GetService(serviceKey); hint != nil {
			if hint.Provider != "" && hint.Provider != "container" && hint.Confidence >= 0.5 {
				return hint.Provider
			}
		}
	}
	// Fall back to first managed provider in catalog.
	managed := catalog.ManagedProviders(serviceKey)
	if len(managed) > 0 {
		return managed[0].Key
	}
	return ""
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

// missingCredsFor returns the env var names required for a registry that are
// not currently set in the process environment.
func missingCredsFor(registry string) []string {
	required := registryRequiredVars(registry)
	var missing []string
	for _, v := range required {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	return missing
}

// registryRequiredVars returns the env vars needed for each registry provider.
func registryRequiredVars(registry string) []string {
	switch registry {
	case "ghcr":
		return []string{"GITHUB_TOKEN", "GITHUB_ACTOR"}
	case "dockerhub":
		return []string{"DOCKER_USERNAME", "DOCKER_PASSWORD"}
	case "custom":
		return []string{"REGISTRY_USERNAME", "REGISTRY_PASSWORD"}
	}
	return nil
}

// prepareCredsInput sets up the credentials text input for the var at idx.
func (m *Model) prepareCredsInput(idx int) {
	key := m.credsKeys[idx]
	m.credsInput.Reset()
	m.credsInput.Placeholder = key
	if strings.Contains(strings.ToLower(key), "password") ||
		strings.Contains(strings.ToLower(key), "token") ||
		strings.Contains(strings.ToLower(key), "secret") {
		m.credsInput.EchoMode = textinput.EchoPassword
		m.credsInput.EchoCharacter = '•'
	} else {
		m.credsInput.EchoMode = textinput.EchoNormal
	}
	m.credsInput.Focus()
}

func (m *Model) activeSelect() *MultiSelect {
	switch m.step {
	case stepServices:
		return &m.services
	case stepManagedServices:
		return &m.managedServices
	case stepEnvs:
		return &m.envs
	case stepTarget:
		return &m.targets
	case stepRegistry:
		return &m.registries
	}
	return nil
}

// ── View ──────────────────────────────────────────────────────────────────────

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
	case stepManagedServices:
		content = m.viewManagedServices()
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
	case stepRegistryCreds:
		content = m.viewRegistryCreds()
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

func (m Model) viewManagedServices() string {
	var b strings.Builder
	b.WriteString("\n  " + StyleTitle.Render("Which services are externally managed?") + "\n")
	b.WriteString("  " + StyleDim.Render("Check the ones hosted outside your infra (Supabase, Neon, Upstash...)") + "\n")

	// Show detection hints if available.
	if m.detected.Hints != nil && len(m.detected.Hints.Services) > 0 {
		b.WriteString("  " + StyleDetected.Render("✓ detected from your project") + "\n")
	}
	b.WriteString("\n")

	for i, item := range m.managedServices.Items {
		cursor := "  "
		if i == m.managedServices.Cursor {
			cursor = StyleCursor.Render("▶ ")
		}

		checkbox := StyleDim.Render("[ ]")
		if item.Selected {
			checkbox = StyleSuccess.Render("[✓]")
		}

		label := item.Label
		if i == m.managedServices.Cursor {
			label = StyleSelected.Render(label)
		} else {
			label = StyleDim.Render(label)
		}

		// Show evidence from hints if available.
		evidence := ""
		if m.detected.Hints != nil {
			if hint := m.detected.Hints.GetService(item.Key); hint != nil && hint.Evidence != "" {
				evidence = "  " + StyleDetected.Render("← "+hint.Evidence)
			}
		}

		// Show available providers as description.
		desc := ""
		if item.Description != "" {
			desc = "  " + StyleDim.Render(item.Description)
		}

		b.WriteString(fmt.Sprintf("  %s%s %s%s%s\n", cursor, checkbox, label, desc, evidence))
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

func (m Model) viewRegistryCreds() string {
	var b strings.Builder

	registryLabel := map[string]string{
		"ghcr":      "GitHub Container Registry",
		"dockerhub": "Docker Hub",
		"custom":    "Custom registry",
	}
	label := registryLabel[m.result.Registry]
	if label == "" {
		label = m.result.Registry
	}

	b.WriteString("\n  " + StyleTitle.Render(label+" credentials") + "\n")
	b.WriteString("  " + StyleDim.Render("These will be saved to .env.local (gitignored) — never committed") + "\n\n")

	// Progress indicator: var 1/2
	collected := len(m.credsValues)
	total := len(m.credsKeys)
	currentKey := ""
	if collected < total {
		currentKey = m.credsKeys[collected]
	}

	b.WriteString(fmt.Sprintf("  %s  %s\n\n",
		StyleSelected.Render(currentKey),
		StyleDim.Render(fmt.Sprintf("(%d / %d)", collected+1, total)),
	))

	// Hint for token generation URL.
	if currentKey == "GITHUB_TOKEN" {
		b.WriteString("  " + StyleDim.Render("Generate at: https://github.com/settings/tokens  (scopes: write:packages)") + "\n\n")
	}

	b.WriteString("  " + m.credsInput.View() + "\n\n")
	b.WriteString("  " + StyleDim.Render("Leave empty + Enter to skip this var") + "\n")
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

	// Services with hosting mode.
	names := make([]string, 0, len(m.result.Services))
	for _, svc := range m.result.Services {
		hosting, ok := m.result.ServiceHosting[svc.Key]
		if ok && hosting == "managed" {
			provider := m.result.ServiceProvider[svc.Key]
			if provider != "" {
				names = append(names, svc.Key+"("+provider+")")
			} else {
				names = append(names, svc.Key+"(managed)")
			}
		} else {
			names = append(names, svc.Key)
		}
	}
	b.WriteString(row("services", strings.Join(names, ", ")))

	if len(m.result.RegistryCreds) > 0 {
		b.WriteString(row("credentials", fmt.Sprintf("%d var(s) → .env.local", len(m.result.RegistryCreds))))
	}

	b.WriteString("\n  " + StyleDim.Render("Press Enter to generate pilot.yaml") + "\n")
	return b.String()
}

func (m Model) footer() string {
	var hint string
	switch m.step {
	case stepServices, stepEnvs, stepManagedServices:
		hint = "↑/↓ navigate   space toggle   enter confirm   esc back   ctrl+c cancel"
	case stepName, stepRegistryImage, stepConfirm:
		hint = "enter confirm   esc back   ctrl+c cancel"
	case stepVPSHost:
		hint = "enter confirm   esc back   ctrl+c cancel   (leave empty + Enter twice to skip)"
	case stepRegistryCreds:
		hint = "enter confirm   leave empty + enter to skip   esc back   ctrl+c cancel"
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
