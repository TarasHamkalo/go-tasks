package models

import (
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gomessenger/internal/client/tui"
)

type CreateChatSubModel struct {
	returnTo   SubModel
	isGroup    bool
	userIds    []string
	inputVal   string
	cursor     int
	focusInput bool // true = typing ID, false = navigating entries list
}

func NewCreateChatSubModel(isGroup bool, returnTo SubModel) *CreateChatSubModel {
	return &CreateChatSubModel{
		isGroup:    isGroup,
		returnTo:   returnTo,
		userIds:    []string{},
		focusInput: true,
	}
}

func (m *CreateChatSubModel) Id() SubModelId {
	return ScreenCreateChat
}

func (m *CreateChatSubModel) Init() tea.Cmd {
	return nil
}

func (m *CreateChatSubModel) ShortHelp() []tui.Binding {
	if m.focusInput {
		return []tui.Binding{
			{Key: "0-9", Description: "Type User ID"},
			{Key: "Tab", Description: "Go to list navigation"},
			{Key: "Enter", Description: "Add user ID / Submit if empty"},
			{Key: "Esc", Description: "Cancel"},
		}
	}
	return []tui.Binding{
		{Key: "k/j, up/down", Description: "Navigate entries"},
		{Key: "x, backspace", Description: "Remove item"},
		{Key: "Tab", Description: "Go back to typing"},
		{Key: "Ctrl+s", Description: "Confirm and Create"},
		{Key: "Esc", Description: "Cancel"},
	}
}

func (m *CreateChatSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		keyStr := msg.String()
		switch keyStr {
		case "esc":
			return m.returnTo, nil

		case "tab":
			m.focusInput = !m.focusInput
			return m, nil
		case "ctrl+s":
			if len(m.userIds) > 0 {
				return m.returnTo, m.submitCreateChat()
			}
		}

		if m.focusInput {
			switch keyStr {
			case "backspace":
				if len(m.inputVal) > 0 {
					m.inputVal = m.inputVal[:len(m.inputVal)-1]
				}
			case "enter":
				if m.inputVal == "" && len(m.userIds) > 0 {
					return m.returnTo, m.submitCreateChat()
				}

				// enforce 9 digit
				if len(m.inputVal) == 9 {
					if !m.isGroup && len(m.userIds) >= 1 {
						m.inputVal = ""
						errModel := NewErrorSubModel(
							fmt.Errorf("direct chats can only have exactly 1 recipient"),
							m,
						)
						return errModel, errModel.Init()
					}

					m.userIds = append(m.userIds, m.inputVal)
					m.inputVal = ""
					m.cursor = len(m.userIds) - 1
				}
			default:
				if len(m.inputVal) < 9 {
					if _, err := strconv.Atoi(keyStr); err == nil {
						m.inputVal += keyStr
					}
				}
			}
		} else {
			// Navigating current tracking entries
			switch keyStr {
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
				}
			case "j", "down":
				if m.cursor < len(m.userIds)-1 {
					m.cursor++
				}
			case "x", "backspace":
				if len(m.userIds) > 0 {
					m.userIds = append(m.userIds[:m.cursor], m.userIds[m.cursor+1:]...)
					m.cursor = max(0, m.cursor-1)
					if len(m.userIds) == 0 {
						m.focusInput = true
					}
				}
			}
		}
	}
	return m, nil
}

func (m *CreateChatSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *CreateChatSubModel) ContentView(width, height int) tea.View {
	title := "New direct chat"
	if m.isGroup {
		title = "New group chat"
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00")).
		MarginBottom(1)
	listStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Padding(1, 0).
		Width(34)

	var entries []string
	if len(m.userIds) == 0 {
		entries = append(entries,
			lipgloss.NewStyle().
				Italic(true).
				Foreground(lipgloss.Color("#666666")).
				Render("No recipients added yet"))
	} else {
		for i, id := range m.userIds {
			prefix := "  "
			itemStyle := lipgloss.NewStyle()
			if !m.focusInput && i == m.cursor {
				prefix = "> "
				itemStyle = itemStyle.Foreground(lipgloss.Color("#00FFFF")).Bold(true)
			}
			entries = append(entries, itemStyle.Render(fmt.Sprintf("%sUser ID: %s", prefix, id)))
		}
	}
	listRendered := listStyle.Render(lipgloss.JoinVertical(lipgloss.Left, entries...))

	inputLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).MarginTop(1)
	inputBoxColor := "#3C3C3C"
	if m.focusInput {
		inputBoxColor = "#FF007F"
	}

	inputBoxStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(inputBoxColor)).Width(34).Padding(0, 1)

	displayInput := m.inputVal
	if len(displayInput) == 0 {
		displayInput = "Enter 9-digit ID..."
	}
	inputRendered := inputBoxStyle.Render(displayInput)

	// Combine inside standard dialog body box
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(title),
		listRendered,
		inputLabelStyle.Render("Add Recipient (Press Tab to toggle):"),
		inputRendered,
	)

	box := tui.DialogBoxStyle.BorderForeground(lipgloss.Color("#00FF00")).Render(content)
	centered := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)

	return tea.NewView(centered)
}

func (m *CreateChatSubModel) submitCreateChat() tea.Cmd {
	return func() tea.Msg {
		// TODO: create chat and send message
		time.Sleep(100 * time.Millisecond)
		return ChatSelectedMsg{ChatId: "created-chat-id"}
	}
}
