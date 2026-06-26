package models

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type ChatLeftSuccessMsg struct{}

type LeaveChatSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	chatId     string

	// false = No (default), true = Yes
	confirm bool
}

func NewLeaveChatSubModel(
	appContext *state.AppContext,
	chatId string,
	returnTo SubModel,
) *LeaveChatSubModel {
	return &LeaveChatSubModel{
		appContext: appContext,
		returnTo:   returnTo,
		chatId:     chatId,
		confirm:    false,
	}
}

func (m *LeaveChatSubModel) Id() SubModelId {
	return ScreenLeaveChat
}

func (m *LeaveChatSubModel) Init() tea.Cmd {
	return nil
}

func (m *LeaveChatSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *LeaveChatSubModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "Space", Description: "Toggle"},
		{Key: "Ctrl+s", Description: "Confirm"},
		{Key: "Esc", Description: "Cancel"},
	}
}

func (m *LeaveChatSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		switch msg.(type) {
		case ChatLeftSuccessMsg:
			return m.returnTo, nil
		}
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		return m.returnTo, nil

	case "space":
		m.confirm = !m.confirm
		return m, nil

	case "ctrl+s":
		if !m.confirm {
			errModel := NewErrorSubModel(
				fmt.Errorf("please confirm that you want to leave the chat"),
				m,
			)
			return errModel, errModel.Init()
		}

		return m.returnTo, m.submitLeave()
	}

	return m, nil
}

func (m *LeaveChatSubModel) ContentView(width, height int) tea.View {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0"))

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF5F5F")).
		Bold(true)

	checkboxStyle := lipgloss.NewStyle().
		Padding(0, 1)

	checkbox := "[ ] Yes, leave this chat"
	if m.confirm {
		checkbox = "[x] Yes, leave this chat"
	}

	checkbox = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Bold(true).
		Render("> " + checkbox)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Leave Chat"),
		"",
		labelStyle.Render("You are about to leave this chat."),
		warningStyle.Render("This action cannot be undone."),
		"",
		checkboxStyle.Render(checkbox),
		"",
		labelStyle.Render("Press Ctrl+S to confirm."),
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

func (m *LeaveChatSubModel) submitLeave() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			2*time.Second,
		)
		defer cancel()

		_, err := m.appContext.MessagingClient.LeaveChat(
			ctx,
			&pb.LeaveChatRequest{
				ChatId: m.chatId,
			},
		)
		if err != nil {
			return ChatModelHandleErrorMsg{Err: err}
		}

		m.appContext.Session.RemoveChat(m.chatId)	
		return ChatLeftSuccessMsg{}
	}
}
