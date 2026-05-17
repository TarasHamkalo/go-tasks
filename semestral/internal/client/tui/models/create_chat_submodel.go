package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type ChatCreationSuccessMsg struct{}

const (
)

type CreateChatSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	isGroup    bool

	userIds   []string
	inputVal  string
	groupName string // used only for groups, max 70 chars

	// Focus order:
	// Direct chat: recipient input <-> members list
	// Group chat:  group name -> member input -> members list
	focusIndex int
	cursor     int

	maxUserIdLen    int
	maxGroupNameLen int
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
		focusIndex: 0,
		maxUserIdLen: 9,
		maxGroupNameLen: 100,
	}
}

func (m *CreateChatSubModel) Id() SubModelId {
	return ScreenCreateChat
}

func (m *CreateChatSubModel) Init() tea.Cmd {
	return nil
}

func (m *CreateChatSubModel) isGroupNameFocused() bool {
	return m.isGroup && m.focusIndex == 0
}

func (m *CreateChatSubModel) isMemberInputFocused() bool {
	if m.isGroup {
		return m.focusIndex == 1
	}
	return m.focusIndex == 0
}

func (m *CreateChatSubModel) isMembersListFocused() bool {
	if m.isGroup {
		return m.focusIndex == 2
	}
	return m.focusIndex == 1
}

func (m *CreateChatSubModel) nextFocus() {
	if m.isGroup {
		m.focusIndex = (m.focusIndex + 1) % 3
	} else {
		m.focusIndex = (m.focusIndex + 1) % 2
	}
}

func (m *CreateChatSubModel) ShortHelp() []tui.Binding {
	if m.isGroupNameFocused() {
		return []tui.Binding{
			{Key: "Type", Description: "Enter group name"},
			{Key: "Tab", Description: "Next field"},
			{Key: "Esc", Description: "Cancel"},
		}
	}

	if m.isMemberInputFocused() {
		return []tui.Binding{
			{Key: "0-9", Description: "Type User Id"},
			{Key: "Enter", Description: "Add member"},
			{Key: "Tab", Description: "Next field"},
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
			m.nextFocus()
			return m, nil

		case "ctrl+s":
			if len(m.userIds) > 0 {
				return m.returnTo, m.submitCreateChat()
			}
		}

		if m.isGroupNameFocused() {
			switch keyStr {
			case "backspace":
				if len(m.groupName) > 0 {
					runes := []rune(m.groupName)
					m.groupName = string(runes[:len(runes)-1])
				}
			default:
				if len([]rune(m.groupName)) < m.maxGroupNameLen {
					m.groupName += msg.Text
				}
			}
			return m, nil
		}

		if m.isMemberInputFocused() {
			switch keyStr {
			case "backspace":
				if len(m.inputVal) > 0 {
					m.inputVal = m.inputVal[:len(m.inputVal)-1]
				}

			case "enter":
				if m.inputVal == "" {
					if len(m.userIds) > 0 {
						if !m.isGroup || strings.TrimSpace(m.groupName) != "" {
							return m.returnTo, m.submitCreateChat()
						}
					}
					return m, nil
				}

				if len(m.inputVal) == m.maxUserIdLen {
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
				if len(m.inputVal) < m.maxUserIdLen {
					if _, err := strconv.Atoi(keyStr); err == nil {
						m.inputVal += keyStr 
					}
				}
			}

			return m, nil
		}


		if m.isMembersListFocused() {
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
						m.focusIndex = 0 
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

	var sections []string
	sections = append(sections, titleStyle.Render(title))

	// Group name input
	if m.isGroup {
		groupName := m.groupName
		if groupName == "" {
			groupName = placeholderStyle.Render("Enter group name...")
		}

		sections = append(
			sections,
			labelStyle.Render("Group Name"),
			makeInputStyle(m.isGroupNameFocused()).Render(groupName),
		)
	}

	// Members list
	var entries []string
	if len(m.userIds) == 0 {
		entries = append(entries, placeholderStyle.Render("No members added yet"))
	} else {
		for i, id := range m.userIds {
			prefix := "  "
			style := lipgloss.NewStyle()

			if m.isMembersListFocused() && i == m.cursor {
				prefix = "> "
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("#00FFFF"))
			}

			entries = append(
				entries,
				style.Render(fmt.Sprintf("%sUser ID: %s", prefix, id)),
			)
		}
	}

	sections = append(
		sections,
		listStyle.Render(lipgloss.JoinVertical(lipgloss.Left, entries...)),
	)

	// Member input
	memberPlaceholder := "Enter 9-digit user ID..."
	if m.isGroup {
		memberPlaceholder = "Enter member user ID..."
	}

	memberInput := m.inputVal
	if memberInput == "" {
		memberInput = placeholderStyle.Render(memberPlaceholder)
	}

	sections = append(
		sections,
		labelStyle.Render("Add Members (Tab to switch focus)"),
		makeInputStyle(m.isMemberInputFocused()).Render(memberInput),
	)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

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
	groupName := strings.TrimSpace(m.groupName)
	if groupName == "" {
		groupName = fmt.Sprintf("Group %s", time.Now().Format("15:04:05"))
	}

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
