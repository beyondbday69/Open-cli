package page

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/llm/agent"
	"github.com/opencode-ai/opencode/internal/session"
	"github.com/opencode-ai/opencode/internal/tui/components/chat"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

var ChatPage PageID = "chat"

// ChatPageInterface describes the full interface of the chat page.
type ChatPageInterface interface {
	tea.Model
	layout.Sizeable
	layout.Bindings
}

type chatPage struct {
	width, height   int
	app             *app.App
	selectedSession session.Session
	// pendingSend holds a SendMsg that arrived before a session was available.
	// It is replayed once the new session broadcasts its SessionSelectedMsg.
	pendingSend *chat.SendMsg
	layout      layout.SplitPaneLayout
}

func (p *chatPage) Init() tea.Cmd {
	return p.layout.Init()
}

func (p *chatPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		cmd := p.layout.SetSize(msg.Width, msg.Height)
		cmds = append(cmds, cmd)
		return p, tea.Batch(cmds...)

	case chat.SendMsg:
		if p.selectedSession.ID == "" {
			// No active session yet — create one first, then replay the send.
			p.pendingSend = &msg
			cmds = append(cmds, p.createSessionCmd())
		} else {
			cmds = append(cmds, p.runAgentCmd(p.selectedSession.ID, msg))
		}

	case chat.SessionClearedMsg:
		p.selectedSession = session.Session{}
		p.pendingSend = nil

	case chat.SessionSelectedMsg:
		p.selectedSession = msg
		// If a send was waiting for a session, replay it now.
		if p.pendingSend != nil {
			pending := *p.pendingSend
			p.pendingSend = nil
			cmds = append(cmds, p.runAgentCmd(p.selectedSession.ID, pending))
		}
	}

	l, cmd := p.layout.Update(msg)
	p.layout = l.(layout.SplitPaneLayout)
	cmds = append(cmds, cmd)

	return p, tea.Batch(cmds...)
}

// createSessionCmd creates a fresh session and broadcasts it as a
// SessionSelectedMsg so all sub-components (messages, sidebar, editor) update.
// The agent will rename the session automatically after the first message
// via the generateTitle goroutine in agent.go.
func (p *chatPage) createSessionCmd() tea.Cmd {
	return func() tea.Msg {
		sess, err := p.app.Sessions.Create(context.Background(), "New Session")
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("failed to create session: %v", err),
			}
		}
		return chat.SessionSelectedMsg(sess)
	}
}

// runAgentCmd sends the user message to the coder agent and drains the event
// channel asynchronously (actual streaming events are delivered through the
// pubsub broker and arrive as pubsub.Event[agent.AgentEvent] messages).
func (p *chatPage) runAgentCmd(sessionID string, msg chat.SendMsg) tea.Cmd {
	return func() tea.Msg {
		events, err := p.app.CoderAgent.Run(
			context.Background(),
			sessionID,
			msg.Text,
			msg.Attachments...,
		)
		if err != nil {
			if err == agent.ErrSessionBusy {
				return util.InfoMsg{Type: util.InfoTypeWarn, Msg: "Agent is busy, please wait…"}
			}
			return util.InfoMsg{Type: util.InfoTypeError, Msg: err.Error()}
		}
		// Drain the result channel so the agent goroutine can exit cleanly.
		go func() {
			for range events {
			}
		}()
		return nil
	}
}

func (p *chatPage) View() string {
	return p.layout.View()
}

func (p *chatPage) SetSize(width, height int) tea.Cmd {
	p.width = width
	p.height = height
	return p.layout.SetSize(width, height)
}

func (p *chatPage) GetSize() (int, int) {
	return p.width, p.height
}

func (p *chatPage) BindingKeys() []key.Binding {
	return p.layout.BindingKeys()
}

// NewChatPage constructs the main chat page, wiring together the messages
// list, the sidebar, and the inline editor inside a split-pane layout.
//
// Layout (default):
//
//	┌─────────────────────┬────────────┐
//	│  messages (70 %)    │  sidebar   │  ← 85 % of total height
//	│                     │  (30 %)    │
//	├─────────────────────┴────────────┤
//	│  editor  (full width)            │  ← 15 % of total height
//	└──────────────────────────────────┘
func NewChatPage(a *app.App) tea.Model {
	messages := chat.NewMessagesCmp(a)
	editor := chat.NewEditorCmp(a)
	sidebar := chat.NewSidebarCmp(session.Session{}, a.History)

	splitLayout := layout.NewSplitPane(
		layout.WithLeftPanel(layout.NewContainer(messages)),
		layout.WithRightPanel(layout.NewContainer(sidebar)),
		layout.WithBottomPanel(layout.NewContainer(editor)),
		layout.WithRatio(0.70),
		layout.WithVerticalRatio(0.85),
	)

	return &chatPage{
		app:    a,
		layout: splitLayout,
	}
}
