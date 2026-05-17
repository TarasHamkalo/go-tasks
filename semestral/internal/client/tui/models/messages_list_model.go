package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
	"gomessenger/internal/client/tui"
)

type ChatSelectedMsg struct {
	ChatId string
}

type MessagesLoadedMsg struct {
	ChatId   string
	Messages []storage.Message
}

type TriggerDeliveryMsg struct {
	Message storage.Message
}

type DeliverySuccessMsg struct {
	LocalId  string
	ServerId string
}

type DeliveryRetryMsg struct {
	Message storage.Message
	Err     error
}

type MessagesListModel struct {
	appContext *state.AppContext
	logger     *zap.Logger

	chatId   string
	messages []storage.Message
	engaged  bool

	// track which messages are at flight already
	sending map[string]bool
}

// TODO: scrolling
func NewMessagesListModel(appContext *state.AppContext) *MessagesListModel {
	return &MessagesListModel{
		appContext: appContext,
		logger:     appContext.RootLogger.With(zap.String("mvc", "msg-list")),
		messages:   make([]storage.Message, 0, 10),
		sending: make(map[string]bool),
	}
}

func (m *MessagesListModel) Init() tea.Cmd {
	return nil
}

func (m *MessagesListModel) SetEngaged(engaged bool) {
	m.engaged = engaged
}

func (m *MessagesListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case ChatSelectedMsg:
		m.chatId = msg.ChatId
		m.messages = []storage.Message{}
		return m, m.loadMessagesFromDb(msg.ChatId)

	case MessagesLoadedMsg:
		if m.chatId == msg.ChatId {
			for i := len(msg.Messages) - 1; i >= 0; i-- {
				m.messages = append(m.messages, msg.Messages[i])
			}

			for _, locMsg := range m.messages {
				if locMsg.IsPending {
					if m.sending[locMsg.Id] {
						continue
					}

					m.sending[locMsg.Id] = true
					cmds = append(cmds, m.sendToServer(locMsg))
				}
			}
		}

		return m, tea.Batch(cmds...)

	case IncomingMessageMsg:
		if msg.Message.SenderId == m.appContext.Session.GetUserId() {
			return m, nil
		}

		if msg.Message.ChatId == m.chatId {
			m.messages = append(m.messages, msg.Message)
		}

		return m, nil

	case MessageInputSubmittedMsg:
		if m.chatId == "" {
			return m, nil
		}

		newMsg := storage.Message{
			Id:        uuid.New().String(),
			ChatId:    m.chatId,
			SenderId:  m.appContext.Session.GetUserId(),
			Content:   []byte(msg.Text),
			SentAt:    time.Now(),
			IsPending: true,
		}

		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		err := m.appContext.LocalRepo.InsertMessage(ctx, &newMsg)
		cancel()

		if err != nil {
			m.logger.Error("failed to save local message", zap.Error(err))
			return m, nil
		}

		m.messages = append(m.messages, newMsg)
		return m, m.sendToServer(newMsg)

	case TriggerDeliveryMsg:
		return m, m.sendToServer(msg.Message)

	case DeliverySuccessMsg:

		ctx, cancel := context.WithTimeout( m.appContext.Ctx, 2*time.Second)
		_ = m.appContext.LocalRepo.MarkMessageDelivered(
			ctx, msg.LocalId, msg.ServerId,
		)
		cancel()

		delete(m.sending, msg.LocalId)

		// Mutate tracking attributes accurately inside memory array maps
		for i, locMsg := range m.messages {
			if locMsg.Id == msg.LocalId {
				m.messages[i].IsPending = false
				m.messages[i].Id = msg.ServerId // key replacement match
				break
			}
		}
		return m, nil

	case DeliveryRetryMsg:
		m.logger.Warn("message delivery failed, retrying...", zap.Error(msg.Err))
		retryCmd := func() tea.Msg {
			time.Sleep(5 * time.Second)
			return TriggerDeliveryMsg{Message: msg.Message}
		}
		return m, retryCmd
	}

	return m, tea.Batch(cmds...)
}

func (m *MessagesListModel) View() tea.View {
	return tea.NewView(m.ContentView(500, 500))
}

func (m *MessagesListModel) ContentView(width, height int) string {
	borderColor := "#3C3C3C"
	if m.engaged {
		borderColor = "#FF007F"
	}

	style := lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	if m.chatId == "" {
		return style.Render(
			lipgloss.Place(
				width-2,
				height-2,
				lipgloss.Center,
				lipgloss.Center,
				"Select a chat to start messaging",
			),
		)
	}

	var renderedMsgs []string
	userId := m.appContext.Session.GetUserId()

	for _, msg := range m.messages {
		sender := "Them"
		if msg.SenderId == userId {
			sender = "You"
		}

		statusMarker := ""
		if msg.IsPending {
			statusMarker = " [🕒 pending...]"
		}

		line := fmt.Sprintf("\033[1m%s:\033[0m %s%s", sender, string(msg.Content), statusMarker)
		renderedMsgs = append(renderedMsgs, line)
	}

	return style.Render(strings.Join(renderedMsgs, "\n"))
}

func (m *MessagesListModel) loadMessagesFromDb(chatId string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()

		msgs, err := m.appContext.LocalRepo.GetMessagesByChatId(ctx, chatId, 50, 0)
		if err != nil {
			m.logger.Error("failed loading messages", zap.Error(err))
			return nil
		}

		return MessagesLoadedMsg{
			ChatId:   chatId,
			Messages: msgs,
		}
	}
}

func (m *MessagesListModel) sendToServer(msg storage.Message) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 5*time.Second)
		defer cancel()

		res, err := m.appContext.MessagingClient.SendMessage(
			ctx,
			&pb.SendMessageRequest{
				ChatId:  msg.ChatId,
				Content: msg.Content,
			},
		)

		// TODO: depends what happend
		if err != nil {
			return DeliveryRetryMsg{Message: msg, Err: err}
		}

		return DeliverySuccessMsg{
			LocalId:  msg.Id,
			ServerId: res.MsgId,
		}
	}
}

func (m *MessagesListModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "k, ^", Description: "Cursor up"},
		{Key: "j, v", Description: "Cursor down"},
	}
}
