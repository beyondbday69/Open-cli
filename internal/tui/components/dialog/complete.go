package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/logging"
	utilComponents "github.com/opencode-ai/opencode/internal/tui/components/util"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type CompletionItem struct {
	title string
	Title string
	Value string
}

type CompletionItemI interface {
	utilComponents.SimpleListItem
	GetValue() string
	DisplayValue() string
}

func (ci *CompletionItem) Render(selected bool, width int) string {
	t := theme.CurrentTheme()
	baseStyle := styles.BaseStyle()

	rowStyle := baseStyle.
		Width(width).
		Padding(0, 1)

	if selected {
		rowStyle = rowStyle.
			Background(t.Background()).
			Foreground(t.Primary()).
			Bold(true)
	} else {
		rowStyle = rowStyle.Foreground(t.Text())
	}

	return rowStyle.Render(" " + ci.GetValue())
}

func (ci *CompletionItem) DisplayValue() string { return ci.Title }
func (ci *CompletionItem) GetValue() string      { return ci.Value }

func NewCompletionItem(completionItem CompletionItem) CompletionItemI {
	return &completionItem
}

type CompletionProvider interface {
	GetId() string
	GetEntry() CompletionItemI
	GetChildEntries(query string) ([]CompletionItemI, error)
}

type CompletionSelectedMsg struct {
	SearchString    string
	CompletionValue string
}

type CompletionDialogCompleteItemMsg struct {
	Value string
}

type CompletionDialogCloseMsg struct{}

type ShowCompletionDialogMsg struct {
	Provider CompletionProvider
}

type CompletionDialog interface {
	tea.Model
	layout.Bindings
	SetWidth(width int)
}

// title shown at the top of the popup – derived from the provider's entry
func providerTitle(p CompletionProvider) string {
	entry := p.GetEntry()
	if entry == nil {
		return ""
	}
	name := entry.DisplayValue()
	if name == "" {
		name = entry.GetValue()
	}
	return name
}

type completionDialogCmp struct {
	query                string
	completionProvider   CompletionProvider
	title                string
	width                int
	height               int
	pseudoSearchTextArea textarea.Model
	listView             utilComponents.SimpleList[CompletionItemI]
}

type completionDialogKeyMap struct {
	Complete key.Binding
	Cancel   key.Binding
}

var completionDialogKeys = completionDialogKeyMap{
	Complete: key.NewBinding(
		key.WithKeys("tab", "enter"),
	),
	Cancel: key.NewBinding(
		key.WithKeys(" ", "esc", "backspace"),
	),
}

func (c *completionDialogCmp) Init() tea.Cmd { return nil }

func (c *completionDialogCmp) complete(item CompletionItemI) tea.Cmd {
	value := c.pseudoSearchTextArea.Value()
	if value == "" {
		return nil
	}
	return tea.Batch(
		util.CmdHandler(CompletionSelectedMsg{
			SearchString:    value,
			CompletionValue: item.GetValue(),
		}),
		c.close(),
	)
}

func (c *completionDialogCmp) close() tea.Cmd {
	c.listView.SetItems([]CompletionItemI{})
	c.pseudoSearchTextArea.Reset()
	c.pseudoSearchTextArea.Blur()
	return util.CmdHandler(CompletionDialogCloseMsg{})
}

func (c *completionDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if c.pseudoSearchTextArea.Focused() {
			if !key.Matches(msg, completionDialogKeys.Complete) {
				var cmd tea.Cmd
				c.pseudoSearchTextArea, cmd = c.pseudoSearchTextArea.Update(msg)
				cmds = append(cmds, cmd)

				query := c.pseudoSearchTextArea.Value()
				if query != "" {
					query = query[1:]
				}

				if query != c.query {
					logging.Info("Query", query)
					items, err := c.completionProvider.GetChildEntries(query)
					if err != nil {
						logging.Error("Failed to get child entries", err)
					}
					c.listView.SetItems(items)
					c.query = query
				}

				u, cmd := c.listView.Update(msg)
				c.listView = u.(utilComponents.SimpleList[CompletionItemI])
				cmds = append(cmds, cmd)
			}

			switch {
			case key.Matches(msg, completionDialogKeys.Complete):
				item, i := c.listView.GetSelectedItem()
				if i == -1 {
					return c, nil
				}
				return c, c.complete(item)

			case key.Matches(msg, completionDialogKeys.Cancel):
				if msg.String() != "backspace" || len(c.pseudoSearchTextArea.Value()) <= 0 {
					return c, c.close()
				}
			}

			return c, tea.Batch(cmds...)
		} else {
			items, err := c.completionProvider.GetChildEntries("")
			if err != nil {
				logging.Error("Failed to get child entries", err)
			}
			c.listView.SetItems(items)
			c.pseudoSearchTextArea.SetValue(msg.String())
			return c, c.pseudoSearchTextArea.Focus()
		}

	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
	}

	return c, tea.Batch(cmds...)
}


func (c *completionDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	// ── inner width: match dialogWidth from models.go (64 chars) ─────────────
	innerW := 56 // matches model selector feel
	for _, item := range c.listView.GetItems() {
		if n := len(item.DisplayValue()) + 16; n > innerW {
			innerW = n
		}
	}
	if innerW > c.width-4 && c.width > 0 {
		innerW = c.width - 4
	}

	// ── title (identical to model selector: bold primary, 2-space indent) ────
	titleStr := c.title
	if titleStr == "" {
		titleStr = providerTitle(c.completionProvider)
	}
	title := base.
		Foreground(t.Primary()).
		Bold(true).
		Width(innerW).
		Render(fmt.Sprintf("  %s", titleStr))

	// ── search box (rounded border like model selector) ───────────────────────
	queryVal := c.pseudoSearchTextArea.Value()
	searchContent := queryVal
	if searchContent == "" {
		searchContent = "type to filter…"
	}
	searchStyle := base.Foreground(t.Text())
	if queryVal == "" {
		searchStyle = base.Foreground(t.TextMuted())
	}
	searchBox := base.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.TextMuted()).
		Width(innerW).
		Padding(0, 1).
		Render(searchStyle.Render(searchContent))

	// ── list rows ─────────────────────────────────────────────────────────────
	c.listView.SetMaxWidth(innerW)
	listContent := c.listView.View()

	// ── result count (right-aligned, like model selector hint) ────────────────
	count := len(c.listView.GetItems())
	var countHint string
	if queryVal != "" && count >= 0 {
		countHint = base.
			Foreground(t.TextMuted()).
			Width(innerW).
			Render(fmt.Sprintf("  %d / %d results", count, count))
	}

	// ── assemble ─────────────────────────────────────────────────────────────
	parts := []string{title, searchBox, listContent}
	if countHint != "" {
		parts = append(parts, countHint)
	}
	content := strings.TrimRight(
		lipgloss.JoinVertical(lipgloss.Left, parts...),
		"\n",
	)

	// Padding(1,2) + RoundedBorder matches the model selector exactly
	return base.
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(lipgloss.Width(content) + 4).
		Render(content)
}

func (c *completionDialogCmp) SetWidth(width int) { c.width = width }

func (c *completionDialogCmp) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(completionDialogKeys)
}

func NewCompletionDialogCmp(completionProvider CompletionProvider) CompletionDialog {
	ti := textarea.New()

	items, err := completionProvider.GetChildEntries("")
	if err != nil {
		logging.Error("Failed to get child entries", err)
	}

	li := utilComponents.NewSimpleList(
		items,
		7,
		"No matches found",
		false,
	)

	return &completionDialogCmp{
		query:                "",
		title:                providerTitle(completionProvider),
		completionProvider:   completionProvider,
		pseudoSearchTextArea: ti,
		listView:             li,
	}
}