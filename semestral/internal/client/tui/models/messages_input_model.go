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

func NewMessageInputModel() *MessagesInputModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message... (500 chars max)"
	ti.CharLimit = 500
	ti.Prompt = ""

	return &MessagesInputModel{
		Input:   ti,
		Engaged: false,
	}
}

func (m *MessagesInputModel) View() tea.View {
	return tea.NewView(m.ContentView(80, 5, false))
}

func (m *MessagesInputModel) Init() tea.Cmd {
	return nil
}

func (m *MessagesInputModel) SetEngaged(engaged bool) tea.Cmd {
	m.Engaged = engaged

	if engaged {
		m.Input.Focus()
		return textinput.Blink
	}

	m.Input.Blur()
	return nil
}

func (m *MessagesInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "enter" {
			val := strings.TrimSpace(m.Input.Value())
			if val != "" {
				m.Input.SetValue("")
				return m, func() tea.Msg {
					return MessageInputSubmittedMsg{
						Text: val,
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

func (m *MessagesInputModel) ContentView(
	width int, height int, focused bool,
) string {
	borderColor := "#3C3C3C"
	if m.Engaged && focused {
		borderColor = "#FF007F"
	} else if focused {
		borderColor = "#00FF00"
	}

	// Outer box.
	outerStyle := lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	innerWidth := max(1, width-6)
	m.Input.SetWidth(innerWidth)

	return outerStyle.Render(
		lipgloss.Place(
			innerWidth,
			max(1, height-2),
			lipgloss.Left,
			lipgloss.Center,
			m.Input.View(),
		),
	)
}

func (m *MessagesInputModel) ShortHelp() []tui.Binding {
	return []tui.Binding{{Key: "Enter", Description: "Send message"}}
}
