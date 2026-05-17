package models

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type UserInvitedSuccessMsg struct{}

type InviteUserSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	chatId     string

	inputVal string
	userId   string // confirmed user ID

	focusInput bool // true = editing input, false = selected user row
	maxUserIdLen int
}

func NewInviteUserSubModel(
	appContext *state.AppContext,
	chatId string,
	returnTo SubModel,
) *InviteUserSubModel {
	return &InviteUserSubModel{
		appContext:   appContext,
		returnTo:     returnTo,
		chatId:       chatId,
		focusInput:   true,
		maxUserIdLen: 9,
	}
}

func (m *InviteUserSubModel) Id() SubModelId {
	return ScreenInviteUser
}

func (m *InviteUserSubModel) Init() tea.Cmd {
	return nil
}

func (m *InviteUserSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *InviteUserSubModel) ShortHelp() []tui.Binding {
	if m.focusInput {
		return []tui.Binding{
			{Key: "0-9", Description: "Type User Id"},
			{Key: "Enter", Description: "Set user / Submit if empty"},
			{Key: "Tab", Description: "Switch focus"},
			{Key: "Esc", Description: "Cancel"},
		}
	}

	return []tui.Binding{
		{Key: "x, Backspace", Description: "Remove user"},
		{Key: "Ctrl+s", Description: "Invite user"},
		{Key: "Tab", Description: "Switch focus"},
		{Key: "Esc", Description: "Cancel"},
	}
}

func (m *InviteUserSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case UserInvitedSuccessMsg:
		return m.returnTo, nil

	case tea.KeyPressMsg:
		key := msg.String()

		switch key {
		case "esc":
			return m.returnTo, nil

		case "tab":
			if m.userId != "" {
				m.focusInput = !m.focusInput
			}
			return m, nil

		case "ctrl+s":
			if m.userId != "" {
				return m.returnTo, m.submitInvite()
			}
		}

		if m.focusInput {
			switch key {
			case "backspace":
				if len(m.inputVal) > 0 {
					m.inputVal = m.inputVal[:len(m.inputVal)-1]
				}

			case "enter":
				if m.inputVal == "" {
					if m.userId != "" {
						return m.returnTo, m.submitInvite()
					}
					return m, nil
				}

				if len(m.inputVal) == m.maxUserIdLen {
					m.userId = m.inputVal
					m.inputVal = ""
					m.focusInput = false
				}

			default:
				if len(m.inputVal) < m.maxUserIdLen {
					if _, err := strconv.Atoi(key); err == nil {
						m.inputVal += key
					}
				}
			}

			return m, nil
		}

		// Selected user row
		switch key {
		case "x", "backspace":
			m.userId = ""
			m.focusInput = true
		}
	}

	return m, nil
}

func (m *InviteUserSubModel) ContentView(width, height int) tea.View {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0")).
		MarginTop(1)

	placeholderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Padding(1, 0).
		Width(34)

	makeInputStyle := func(focused bool) lipgloss.Style {
		color := "#3C3C3C"
		if focused {
			color = "#FF007F"
		}

		return lipgloss.NewStyle().
			Width(34).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color))
	}

	// Selected user section
	var selectedUser string
	if m.userId == "" {
		selectedUser = placeholderStyle.Render("No user selected")
	} else {
		prefix := "  "
		style := lipgloss.NewStyle()

		if !m.focusInput {
			prefix = "> "
			style = style.
				Bold(true).
				Foreground(lipgloss.Color("#00FFFF"))
		}

		selectedUser = style.Render(
			fmt.Sprintf("%sUser ID: %s", prefix, m.userId),
		)
	}

	listRendered := listStyle.Render(selectedUser)

	// Input section
	displayInput := m.inputVal
	if displayInput == "" {
		displayInput = placeholderStyle.Render("Enter 9-digit user ID...")
	}

	inputRendered := makeInputStyle(m.focusInput).Render(displayInput)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Invite User"),
		listRendered,
		labelStyle.Render("User ID (Tab to switch focus)"),
		inputRendered,
	)

	dialog := tui.DialogBoxStyle.
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Render(content)

	centered := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)

	return tea.NewView(centered)
}

func (m *InviteUserSubModel) submitInvite() tea.Cmd {
	return func() tea.Msg {
		if m.userId == "" {
			return ChatModelHandleErrorMsg{
				Err: fmt.Errorf("user ID is required"),
			}
		}

		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			2*time.Second,
		)
		defer cancel()

		_, err := m.appContext.MessagingClient.AddChatMember(
			ctx,
			&pb.AddChatMemberRequest{
				ChatId: m.chatId,
				TargetUserId: m.userId,
			},
		)
		if err != nil {
			return ChatModelHandleErrorMsg{Err: err}
		}

		// Update local session cache.
		members, ok := m.appContext.Session.GetChatMembers(m.chatId)
		if ok {
			members = append(members, m.userId)
			m.appContext.Session.SetChatMembers(m.chatId, members)
		}

		return UserInvitedSuccessMsg{}
	}
}
