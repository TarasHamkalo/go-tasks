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

type ChatInfoLoadedMsg struct {
	MemberIds []string
}

type ChatInfoLoadFailedMsg struct {
	Err error
}

type ChatInfoSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	chatId     string

	memberIds []string
	cursor    int

	loading bool
}

func NewChatInfoSubModel(
	appContext *state.AppContext,
	chatId string,
	returnTo SubModel,
) *ChatInfoSubModel {
	return &ChatInfoSubModel{
		appContext: appContext,
		returnTo:   returnTo,
		chatId:     chatId,
		memberIds:  []string{},
		loading:    true,
	}
}

func (m *ChatInfoSubModel) Id() SubModelId {
	return ScreenChatInfo
}

func (m *ChatInfoSubModel) Init() tea.Cmd {
	return m.loadMembers()
}

func (m *ChatInfoSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *ChatInfoSubModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "up/k", Description: "Up"},
		{Key: "down/j", Description: "Down"},
		{Key: "Enter", Description: "Open Profile"},
		{Key: "Esc", Description: "Back"},
	}
}

func (m *ChatInfoSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ChatInfoLoadedMsg:
		m.memberIds = msg.MemberIds
		m.loading = false

		if m.cursor >= len(m.memberIds) {
			m.cursor = max(0, len(m.memberIds)-1)
		}

		return m, nil

	case ChatInfoLoadFailedMsg:
		return NewErrorSubModel(msg.Err, m.returnTo), nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m.returnTo, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.memberIds)-1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			if len(m.memberIds) == 0 {
				return m, nil
			}

			model := NewProfileSubModel(
				m.appContext,
				m.memberIds[m.cursor],
				false,
				m,
			)

			return model, model.Init()
		}
	}

	return m, nil
}

func (m *ChatInfoSubModel) ContentView(width, height int) tea.View {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5"))

	placeholderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)

	listWidth := min(55, width-10)
	if listWidth < 30 {
		listWidth = 30
	}

	listStyle := lipgloss.NewStyle().
		Padding(1, 0).
		Width(listWidth)

	chatName := m.chatId
	if chat, ok := m.appContext.Session.GetChat(m.chatId); ok {
		chatName = chat.Name()
	}

	var rows []string

	switch {
	case m.loading:
		rows = []string{
			placeholderStyle.Render("Loading chat members..."),
		}

	case len(m.memberIds) == 0:
		rows = []string{
			placeholderStyle.Render("No members found"),
		}

	default:
		for i, memberId := range m.memberIds {
			prefix := "  "
			style := lipgloss.NewStyle()

			if i == m.cursor {
				prefix = "> "
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("#00FFFF"))
			}

			displayName := memberId
			if profile, ok := m.appContext.Session.GetProfile(memberId); ok {
				displayName = fmt.Sprintf(
					"%s (%s)",
					profile.Username,
					memberId,
				)
			}

			rows = append(
				rows,
				style.Render(prefix + displayName),
			)
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Chat: " + chatName),
		"",
		titleStyle.Render("Members:"),
		listStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left, rows...),
		),
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

func (m *ChatInfoSubModel) loadMembers() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			3*time.Second,
		)
		defer cancel()

		res, err := m.appContext.MessagingClient.GetChatMembers(
			ctx,
			&pb.GetChatMembersRequest{
				ChatId: m.chatId,
			},
		)
		if err != nil {
			return ChatInfoLoadFailedMsg{Err: err}
		}

		// update local session cache
		m.appContext.Session.SetChatMembers(
			m.chatId,
			res.MemberIds,
		)

		return ChatInfoLoadedMsg{
			MemberIds: res.MemberIds,
		}
	}
}
