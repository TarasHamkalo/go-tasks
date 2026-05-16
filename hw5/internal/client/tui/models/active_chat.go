package models 

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ActiveChatSection struct {
	Input    textinput.Model
	Messages []string
}

func NewActiveChatSection() *ActiveChatSection {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 500

	return &ActiveChatSection{
		Input: ti,
		Messages: []string{
			"System: Welcome to Go Messenger!",
			"Alice: Hey! Are the dialogs working?",
		},
	}
}

func (ac *ActiveChatSection) Init() tea.Cmd {
	return nil
}

// NEW: Added `engaged` to the signature so the component knows its state
func (ac *ActiveChatSection) Update(msg tea.Msg, engaged bool) (*ActiveChatSection, tea.Cmd) {
	// 1. Sync Focus State Safely in Update!
	if engaged && !ac.Input.Focused() {
		ac.Input.Focus()
	} else if !engaged && ac.Input.Focused() {
		ac.Input.Blur()
	}

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if engaged {
			// We are typing a message
			if msg.String() == "enter" {
				val := strings.TrimSpace(ac.Input.Value())
				if val != "" {
					ac.Messages = append(ac.Messages, "You: "+val)
					ac.Input.SetValue("")
					// TODO: Trigger gRPC SendMessage routine here
				}
				return ac, nil
			}
		} else {
			// We are NOT engaged, intercept shortcuts securely
			switch msg.String() {
			case "m":
				return ac, func() tea.Msg { return OpenMembersMsg{} }
			case "i":
				return ac, func() tea.Msg { return OpenInviteMsg{} }
			}
		}
	}

	// 2. Let the textinput process the message (it safely ignores keys if blurred)
	ac.Input, cmd = ac.Input.Update(msg)
	return ac, cmd
}

func (ac *ActiveChatSection) View(width, height int, focused, engaged bool) string {
	borderColor := "#3C3C3C"
	if engaged && focused {
		borderColor = "#FF007F"
	} else if focused {
		borderColor = "#00FF00"
	}

	// REMOVED: All ac.Input.Focus() and ac.Input.Blur() calls. 
	// View should only ever read state, never modify it!

	outerStyle := lipgloss.NewStyle().Width(width - 2).Height(height - 2).
		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(borderColor))

	messagesBoxHeight := height - 5
	msgStyle := lipgloss.NewStyle().Height(messagesBoxHeight).Padding(1, 2)
	inputStyle := lipgloss.NewStyle().Width(width - 6).Padding(1, 1).
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("#2B2B2B"))

	displayMsgs := strings.Join(ac.Messages, "\n")
	renderedMsgs := msgStyle.Render(displayMsgs)
	renderedInput := inputStyle.Render(ac.Input.View())

	return outerStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, renderedMsgs, renderedInput),
	)
}
