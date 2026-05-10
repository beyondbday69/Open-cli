package completions

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
)

// slashCommandItem implements dialog.CompletionItemI for slash commands,
// rendering both the command name and a short description.
type slashCommandItem struct {
	name        string
	description string
}

func (s *slashCommandItem) Render(selected bool, width int) string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	nameStyle := base.Bold(true)
	descStyle := base.Foreground(t.TextMuted())

	if selected {
		nameStyle = nameStyle.Foreground(t.Primary())
		descStyle = descStyle.Foreground(t.Primary())
	} else {
		nameStyle = nameStyle.Foreground(t.Text())
	}

	name := nameStyle.Render(s.name)
	desc := descStyle.Render(s.description)

	gap := base.Foreground(t.TextMuted()).Render("  ")
	row := lipgloss.JoinHorizontal(lipgloss.Left, name, gap, desc)

	rowStyle := base.Width(width).Padding(0, 1)
	if selected {
		rowStyle = rowStyle.Background(t.Background())
	}
	return rowStyle.Render(row)
}

func (s *slashCommandItem) DisplayValue() string { return s.name }
func (s *slashCommandItem) GetValue() string      { return s.name }

// slashCommandsContextGroup is the CompletionProvider for / commands.
type slashCommandsContextGroup struct{}

var allSlashCommands = []*slashCommandItem{
	{name: "/model", description: "Switch AI model"},
	{name: "/session", description: "Switch or create session"},
	{name: "/theme", description: "Change color theme"},
	{name: "/file", description: "Attach a file"},
	{name: "/new", description: "Start a new session"},
	{name: "/commands", description: "Run custom commands"},
	{name: "/logs", description: "View application logs"},
	{name: "/help", description: "Show help & key bindings"},
}

func (cg *slashCommandsContextGroup) GetId() string { return "slash" }

func (cg *slashCommandsContextGroup) GetEntry() dialog.CompletionItemI {
	return dialog.NewCompletionItem(dialog.CompletionItem{
		Title: "Commands",
		Value: "commands",
	})
}

func (cg *slashCommandsContextGroup) GetChildEntries(query string) ([]dialog.CompletionItemI, error) {
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(query), "/"))

	items := make([]dialog.CompletionItemI, 0, len(allSlashCommands))
	for _, cmd := range allSlashCommands {
		nameQ := strings.ToLower(strings.TrimPrefix(cmd.name, "/"))
		if q == "" || strings.Contains(nameQ, q) || strings.Contains(strings.ToLower(cmd.description), q) {
			items = append(items, cmd)
		}
	}
	return items, nil
}

func NewSlashCommandContextGroup() dialog.CompletionProvider {
	return &slashCommandsContextGroup{}
}