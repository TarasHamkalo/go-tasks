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

type MessageAcksLoadedMsg struct {
	Acks []*pb.MessageAckInfo
}

type MessageAcksLoadFailedMsg struct {
	Err error
}

type MessageAcksSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	msgId      string

	acks   []*pb.MessageAckInfo
	cursor int

	loading bool
}

func NewMessageAcksSubModel(
	appContext *state.AppContext,
	msgId string,
	returnTo SubModel,
) *MessageAcksSubModel {
	return &MessageAcksSubModel{
		appContext: appContext,
		returnTo:   returnTo,
		msgId:      msgId,
		acks:       []*pb.MessageAckInfo{},
		loading:    true,
	}
}

func (m *MessageAcksSubModel) Id() SubModelId {
	return ScreenMessageAcks
}

func (m *MessageAcksSubModel) Init() tea.Cmd {
	return m.loadAcks()
}

func (m *MessageAcksSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *MessageAcksSubModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "up/k", Description: "Up"},
		{Key: "down/j", Description: "Down"},
		{Key: "Enter", Description: "Open profile"},
		{Key: "Esc", Description: "Back"},
	}
}

func (m *MessageAcksSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MessageAcksLoadedMsg:
		m.acks = msg.Acks
		m.loading = false
		if m.cursor >= len(m.acks) {
			m.cursor = max(0, len(m.acks)-1)
		}
		return m, nil

	case MessageAcksLoadFailedMsg:
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
			if m.cursor < len(m.acks)-1 {
				m.cursor++
			}
			return m, nil

		case "enter":
			model := NewProfileSubModel(
				m.appContext, m.acks[m.cursor].UserId, false, m,
			)

			return model, model.Init() 
		}
	}

	return m, nil
}

func (m *MessageAcksSubModel) ContentView(width, height int) tea.View {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5")).
		MarginBottom(1)

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

	title := titleStyle.Render("Message Delivery Status")

	var rows []string

	switch {
	case m.loading:
		rows = []string{
			placeholderStyle.Render("Loading acknowledgements..."),
		}

	case len(m.acks) == 0:
		rows = []string{
			placeholderStyle.Render("No acknowledgements available"),
		}

	default:
		for i, ack := range m.acks {
			prefix := "  "
			style := lipgloss.NewStyle()

			if i == m.cursor {
				prefix = "> "
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("#00FFFF"))
			}

			delivered := ack.DeliveredAt != nil && ack.DeliveredAt.IsValid()
			read := ack.ReadAt != nil && ack.ReadAt.IsValid()

			// %t is the correct formatter for bool values.
			row := fmt.Sprintf(
				"%s%s  Delivered: %t  Read: %t",
				prefix,
				ack.UserId,
				delivered,
				read,
			)

			rows = append(rows, style.Render(row))
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
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

func (m *MessageAcksSubModel) loadAcks() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			3*time.Second,
		)
		defer cancel()

		res, err := m.appContext.MessagingClient.GetMessageAcks(
			ctx,
			&pb.GetMessageAcksRequest{
				MsgId: m.msgId,
			},
		)
		if err != nil {
			return MessageAcksLoadFailedMsg{Err: err}
		}

		return MessageAcksLoadedMsg{
			Acks: res.Acks,
		}
	}
}
