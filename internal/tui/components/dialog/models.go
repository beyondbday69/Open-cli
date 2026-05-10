package dialog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

const (
	numVisibleModels = 12
	dialogWidth      = 64 // wide enough for long model names on one line
)

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model models.Model
}

// CloseModelDialogMsg is sent when a model is selected
type CloseModelDialogMsg struct{}

// ModelDialog interface for the model selection dialog
type ModelDialog interface {
	tea.Model
	layout.Bindings
}

type modelDialogCmp struct {
	// all models for current provider
	allModels []models.Model
	// filtered models (after search)
	filteredModels []models.Model

	provider           models.ModelProvider
	availableProviders []models.ModelProvider

	selectedIdx   int
	width         int
	height        int
	scrollOffset  int
	hScrollOffset int
	hScrollPossible bool

	search textinput.Model
}

type modelKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Escape key.Binding
	J      key.Binding
	K      key.Binding
	H      key.Binding
	L      key.Binding
}

var modelKeys = modelKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "previous model"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "next model"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "prev provider"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "next provider"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select model"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	J: key.NewBinding(
		key.WithKeys("j"),
		key.WithHelp("j", "next model"),
	),
	K: key.NewBinding(
		key.WithKeys("k"),
		key.WithHelp("k", "previous model"),
	),
	H: key.NewBinding(
		key.WithKeys("h"),
		key.WithHelp("h", "prev provider"),
	),
	L: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "next provider"),
	),
}

func (m *modelDialogCmp) Init() tea.Cmd {
	m.setupModels()
	return textinput.Blink
}

func (m *modelDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, modelKeys.Escape):
			return m, util.CmdHandler(CloseModelDialogMsg{})

		case key.Matches(msg, modelKeys.Enter):
			if len(m.filteredModels) > 0 {
				selected := m.filteredModels[m.selectedIdx]
				util.ReportInfo(fmt.Sprintf("selected model: %s", selected.Name))
				return m, util.CmdHandler(ModelSelectedMsg{Model: selected})
			}

		case key.Matches(msg, modelKeys.Up) || key.Matches(msg, modelKeys.K):
			m.moveSelectionUp()

		case key.Matches(msg, modelKeys.Down) || key.Matches(msg, modelKeys.J):
			m.moveSelectionDown()

		case key.Matches(msg, modelKeys.Left) || key.Matches(msg, modelKeys.H):
			if m.hScrollPossible && m.search.Value() == "" {
				m.switchProvider(-1)
			}

		case key.Matches(msg, modelKeys.Right) || key.Matches(msg, modelKeys.L):
			if m.hScrollPossible && m.search.Value() == "" {
				m.switchProvider(1)
			}

		default:
			// All other keys go to the search input
			var cmd tea.Cmd
			prevVal := m.search.Value()
			m.search, cmd = m.search.Update(msg)
			cmds = append(cmds, cmd)
			if m.search.Value() != prevVal {
				m.applyFilter()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	default:
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// ── movement ──────────────────────────────────────────────────────────────────

func (m *modelDialogCmp) moveSelectionUp() {
	if len(m.filteredModels) == 0 {
		return
	}
	if m.selectedIdx > 0 {
		m.selectedIdx--
	} else {
		m.selectedIdx = len(m.filteredModels) - 1
		m.scrollOffset = max(0, len(m.filteredModels)-numVisibleModels)
	}
	if m.selectedIdx < m.scrollOffset {
		m.scrollOffset = m.selectedIdx
	}
}

func (m *modelDialogCmp) moveSelectionDown() {
	if len(m.filteredModels) == 0 {
		return
	}
	if m.selectedIdx < len(m.filteredModels)-1 {
		m.selectedIdx++
	} else {
		m.selectedIdx = 0
		m.scrollOffset = 0
	}
	if m.selectedIdx >= m.scrollOffset+numVisibleModels {
		m.scrollOffset = m.selectedIdx - (numVisibleModels - 1)
	}
}

func (m *modelDialogCmp) switchProvider(offset int) {
	newOffset := m.hScrollOffset + offset
	if newOffset < 0 {
		newOffset = len(m.availableProviders) - 1
	}
	if newOffset >= len(m.availableProviders) {
		newOffset = 0
	}
	m.hScrollOffset = newOffset
	m.provider = m.availableProviders[m.hScrollOffset]
	m.search.SetValue("")
	m.setupModelsForProvider(m.provider)
}

// ── filtering ─────────────────────────────────────────────────────────────────

func (m *modelDialogCmp) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.search.Value()))
	if q == "" {
		m.filteredModels = m.allModels
	} else {
		m.filteredModels = nil
		for _, mdl := range m.allModels {
			if strings.Contains(strings.ToLower(mdl.Name), q) ||
				strings.Contains(strings.ToLower(string(mdl.ID)), q) {
				m.filteredModels = append(m.filteredModels, mdl)
			}
		}
	}
	m.selectedIdx = 0
	m.scrollOffset = 0
}

// ── rendering ─────────────────────────────────────────────────────────────────

// truncateName ensures a model name fits in w runes, truncating with "…" if needed.
func truncateName(name string, w int) string {
	runes := []rune(name)
	if len(runes) <= w {
		return name + strings.Repeat(" ", w-len(runes))
	}
	if w <= 1 {
		return "…"
	}
	return string(runes[:w-1]) + "…"
}

func (m *modelDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()
	innerW := dialogWidth // usable width inside the border+padding

	// ── title ────────────────────────────────────────────────────────────────
	providerName := strings.ToUpper(string(m.provider)[:1]) + string(m.provider[1:])
	title := base.
		Foreground(t.Primary()).
		Bold(true).
		Width(innerW).
		Render(fmt.Sprintf("  Select %s Model", providerName))

	// ── search box ───────────────────────────────────────────────────────────
	m.search.Width = innerW - 6 // account for prompt + border chars
	searchBox := base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.TextMuted()).
		Width(innerW).
		Padding(0, 1).
		Render(m.search.View())

	// ── model list ───────────────────────────────────────────────────────────
	nameW := innerW - 2 // left margin
	endIdx := min(m.scrollOffset+numVisibleModels, len(m.filteredModels))
	rows := make([]string, 0, endIdx-m.scrollOffset)

	if len(m.filteredModels) == 0 {
		rows = append(rows, base.
			Foreground(t.TextMuted()).
			Width(innerW).
			Padding(0, 1).
			Render("  no models match your search"))
	} else {
		for i := m.scrollOffset; i < endIdx; i++ {
			mdl := m.filteredModels[i]
			nameLine := truncateName(mdl.Name, nameW)
			rowStyle := base.Width(innerW).Padding(0, 1)
			if i == m.selectedIdx {
				rowStyle = rowStyle.
					Background(t.Primary()).
					Foreground(t.Background()).
					Bold(true)
			} else {
				rowStyle = rowStyle.Foreground(t.Text())
			}
			rows = append(rows, rowStyle.Render(" "+nameLine))
		}
	}

	list := base.Width(innerW).Render(
		lipgloss.JoinVertical(lipgloss.Left, rows...),
	)

	// ── scroll / provider indicators ─────────────────────────────────────────
	indicator := m.buildIndicator(innerW)

	// ── result count hint ────────────────────────────────────────────────────
	hint := ""
	if m.search.Value() != "" {
		hint = base.
			Foreground(t.TextMuted()).
			Width(innerW).
			Render(fmt.Sprintf("  %d / %d models", len(m.filteredModels), len(m.allModels)))
	}

	// ── assemble ─────────────────────────────────────────────────────────────
	parts := []string{title, searchBox, list}
	if hint != "" {
		parts = append(parts, hint)
	}
	if indicator != "" {
		parts = append(parts, indicator)
	}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return base.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (m *modelDialogCmp) buildIndicator(maxWidth int) string {
	var parts []string

	if len(m.filteredModels) > numVisibleModels {
		if m.scrollOffset > 0 {
			parts = append(parts, "↑")
		}
		if m.scrollOffset+numVisibleModels < len(m.filteredModels) {
			parts = append(parts, "↓")
		}
	}
	if m.hScrollPossible && m.search.Value() == "" {
		prefix := ""
		suffix := ""
		if m.hScrollOffset > 0 {
			prefix = "← "
		}
		if m.hScrollOffset < len(m.availableProviders)-1 {
			suffix = " →"
		}
		if prefix != "" || suffix != "" {
			parts = append([]string{prefix + "provider" + suffix}, parts...)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	t := theme.CurrentTheme()
	return styles.BaseStyle().
		Foreground(t.Primary()).
		Width(maxWidth).
		Align(lipgloss.Right).
		Bold(true).
		Render(strings.Join(parts, "  "))
}

// ── bindings ──────────────────────────────────────────────────────────────────

func (m *modelDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(modelKeys)
}

// ── setup ─────────────────────────────────────────────────────────────────────

func (m *modelDialogCmp) setupModels() {
	cfg := config.Get()
	modelInfo := GetSelectedModel(cfg)
	m.availableProviders = getEnabledProviders(cfg)
	m.hScrollPossible = len(m.availableProviders) > 1

	m.provider = modelInfo.Provider
	m.hScrollOffset = findProviderIndex(m.availableProviders, m.provider)

	// Init search input
	si := textinput.New()
	si.Placeholder = "search models…"
	si.Focus()
	si.CharLimit = 64
	m.search = si

	m.setupModelsForProvider(m.provider)
}

func GetSelectedModel(cfg *config.Config) models.Model {
	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelId := agentCfg.Model
	return models.SupportedModels[selectedModelId]
}

func getEnabledProviders(cfg *config.Config) []models.ModelProvider {
	var providers []models.ModelProvider
	for providerId, provider := range cfg.Providers {
		if !provider.Disabled {
			providers = append(providers, providerId)
		}
	}
	slices.SortFunc(providers, func(a, b models.ModelProvider) int {
		rA := models.ProviderPopularity[a]
		rB := models.ProviderPopularity[b]
		if rA == 0 {
			rA = 999
		}
		if rB == 0 {
			rB = 999
		}
		return rA - rB
	})
	return providers
}

func findProviderIndex(providers []models.ModelProvider, provider models.ModelProvider) int {
	for i, p := range providers {
		if p == provider {
			return i
		}
	}
	return -1
}

func (m *modelDialogCmp) setupModelsForProvider(provider models.ModelProvider) {
	cfg := config.Get()
	agentCfg := cfg.Agents[config.AgentCoder]
	selectedModelId := agentCfg.Model

	m.provider = provider
	m.allModels = getModelsForProvider(provider)
	m.filteredModels = m.allModels
	m.selectedIdx = 0
	m.scrollOffset = 0

	if provider == models.SupportedModels[selectedModelId].Provider {
		for i, mdl := range m.allModels {
			if mdl.ID == selectedModelId {
				m.selectedIdx = i
				if m.selectedIdx >= numVisibleModels {
					m.scrollOffset = m.selectedIdx - (numVisibleModels - 1)
				}
				break
			}
		}
	}
}

func getModelsForProvider(provider models.ModelProvider) []models.Model {
	var providerModels []models.Model
	for _, mdl := range models.SupportedModels {
		if mdl.Provider == provider {
			providerModels = append(providerModels, mdl)
		}
	}
	slices.SortFunc(providerModels, func(a, b models.Model) int {
		if a.Name > b.Name {
			return -1
		} else if a.Name < b.Name {
			return 1
		}
		return 0
	})
	return providerModels
}

func NewModelDialogCmp() ModelDialog {
	return &modelDialogCmp{}
}