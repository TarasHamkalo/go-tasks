package models

import (
	"gomessenger/internal/client/tui"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type MessageInputSubmittedMsg struct {
	Text string
}

type MessagesInputModel struct {
	Input textinput.Model

	Engaged bool
}

func NewMessageInputModel() *MessagesInputModel{
	ti := textinput.New()
	ti.Placeholder = "Type a message... (500 chars max)"
	ti.CharLimit = 500

	return &MessagesInputModel{
		Input: ti,
		Engaged: false,
	}
}

func (m *MessagesInputModel) View() tea.View {
	return tea.NewView(m.ContentView(500, 500))
}

func (m *MessagesInputModel) Init() tea.Cmd {
	return nil
}

func (m *MessagesInputModel) SetEngaged(engaged bool) {
	m.Engaged = engaged
	if engaged {
		m.Input.Focus()
	} else {
		m.Input.Blur()
	}
}

func (m *MessagesInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "enter" {
			val := strings.TrimSpace(m.Input.Value())
			if len(val) > 0 {
				m.Input.SetValue("")
				return m, func() tea.Msg {
					return MessageInputSubmittedMsg{
						Text: val,
					}
				}
			}
		}

		var cmd tea.Cmd
		m.Input, cmd = m.Input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *MessagesInputModel) ContentView(width int, height int) string {
	borderColor := "#3C3C3C"
	if m.Engaged {
		borderColor = "#FF007F"
	}

	outerStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor))

	inputStyle := lipgloss.NewStyle().
		Width(width-6).
		Padding(1, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("#2B2B2B"))

	return outerStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			inputStyle.Render(m.Input.View()),
		),
	)
}

func (m *MessagesInputModel)	ShortHelp() []tui.Binding {
	return []tui.Binding{{Key: "Enter", Description: "Send message"}}
}
