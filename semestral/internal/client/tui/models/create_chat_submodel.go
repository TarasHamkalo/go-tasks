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

type ChatCreationSuccessMsg struct{}

type CreateChatSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	isGroup    bool
	userIds    []string
	inputVal   string
	cursor     int
	focusInput bool // true = typing ID, false = navigating entries list
}

func NewCreateChatSubModel(
	appContext *state.AppContext,
	isGroup bool,
	returnTo SubModel,
) *CreateChatSubModel {
	return &CreateChatSubModel{
		appContext: appContext,
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
		{Key: "x, backspace", Description: "Remove member"},
		{Key: "Tab", Description: "Go back to typing"},
		{Key: "Ctrl+s", Description: "Confirm and Create"},
		{Key: "Esc", Description: "Cancel"},
	}
}

func (m *CreateChatSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ChatCreationSuccessMsg:
		return m.returnTo, nil
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
							fmt.Errorf("direct chats can only have exactly 1 member"),
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
	title := "New Direct Chat"
	if m.isGroup {
		title = "New Group Chat"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0")).
		MarginTop(1)

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#3C3C3C")).
		Padding(1, 0).
		Width(34)

	inputBorderColor := "#3C3C3C"
	if m.focusInput {
		inputBorderColor = "#FF007F"
	}

	inputBoxStyle := lipgloss.NewStyle().
		Width(34).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(inputBorderColor))

	placeholderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	// render recipients
	var entries []string
	if len(m.userIds) == 0 {
		entries = append(
			entries,
			placeholderStyle.Render("No members added yet"),
		)
	} else {
		for i, id := range m.userIds {
			prefix := "  "
			itemStyle := lipgloss.NewStyle()

			if !m.focusInput && i == m.cursor {
				prefix = "> "
				itemStyle = itemStyle.
					Bold(true).
					Foreground(lipgloss.Color("#00FFFF"))
			}

			entries = append(
				entries,
				itemStyle.Render(fmt.Sprintf("%sUser ID: %s", prefix, id)),
			)
		}
	}

	listRendered := listStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left, entries...),
	)

	// render input
	displayInput := m.inputVal
	if displayInput == "" {
		displayInput = placeholderStyle.Render("Enter 9-digit user ID...")
	}

	if m.isGroup {
		displayInput = m.inputVal
		if displayInput == "" {
			displayInput = placeholderStyle.Render("Enter member user ID...")
		}
	}

	inputRendered := inputBoxStyle.Render(displayInput)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render(title),
		listRendered,
		labelStyle.Render("Add Members (Tab to switch focus)"),
		inputRendered,
	)

	// Neutral outer dialog border.
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

func (m *CreateChatSubModel) submitCreateChat() tea.Cmd {
	return func() tea.Msg {
		var chatId string
		if m.isGroup {
			return m.createGroup()
		}

		if len(m.userIds) != 1 {
			return ChatModelHandleErrorMsg{
				Err: fmt.Errorf("direct chat requires exactly one recipient"),
			}
		}

		targetUserId := m.userIds[0]

		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()
		res, err := m.appContext.MessagingClient.CreateDirectChat(
			ctx,
			&pb.CreateDirectChatRequest{
				TargetUserId: targetUserId,
			},
		)

		if err != nil {
			return ChatModelHandleErrorMsg{Err: err}
		}

		chatId = res.ChatId
		otherProfile, ok := m.appContext.Session.GetProfile(targetUserId)

		if !ok {
			otherProfile, err = tui.ResolveProfileToSession(
				m.appContext, targetUserId,
			)
			if err != nil {
				return ChatModelHandleErrorMsg{Err: err}
			}
		}

		m.appContext.Session.InsertChat(&state.DirectChat{
			ChatId:       chatId,
			OtherProfile: otherProfile,
		})

		m.appContext.Session.SetChatMembers(
			chatId,
			[]string{
				m.appContext.Session.GetUserId(),
				targetUserId,
			},
		)

		return ChatCreationSuccessMsg{}
	}
}

func (m *CreateChatSubModel) createGroup() tea.Msg {
	groupName := fmt.Sprintf("Group %s", time.Now().Format("15:04:05"))
	ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
	defer cancel()

	res, err := m.appContext.MessagingClient.CreateGroupChat(
		ctx,
		&pb.CreateGroupChatRequest{
			Name:      groupName,
			MemberIds: m.userIds,
		},
	)
	if err != nil {
		return ChatModelHandleErrorMsg{Err: err}
	}

	chatId := res.ChatId
	m.appContext.Session.InsertChat(&state.GroupChat{
		ChatId:    chatId,
		GroupName: groupName,
	})

	m.appContext.Session.SetChatMembers(chatId, m.userIds)
	return ChatCreationSuccessMsg{}
}
