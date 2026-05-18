package models

import (
	"context"
	"errors"
	"fmt"
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

type MessageSelectedMsg struct {
	MsgId string
}

type MessagesLoadedMsg struct {
	ChatId   string
	Messages []storage.Message
	IsAppend bool
	HasMore  bool
}

type TriggerDeliveryMsg struct {
	Message    storage.Message
	RetryCount int
}

type DeliverySuccessMsg struct {
	LocalId  string
	ServerId string
}

type DeliveryRetryMsg struct {
	Message    storage.Message
	Err        error
	RetryCount int
}

type MessagesListModel struct {
	appContext *state.AppContext

	chatId  string
	engaged bool

	sending map[string]bool

	messages []storage.Message

	limit      int
	cursor     int
	startIndex int

	isLoading bool
	hasMore   bool

	logger *zap.Logger
}

func NewMessagesListModel(appContext *state.AppContext) *MessagesListModel {
	return &MessagesListModel{
		appContext: appContext,
		logger:     appContext.RootLogger.With(zap.String("mvc", "msg-list")),
		limit:      25, // how many messages to load
		messages:   make([]storage.Message, 0, 10),
		sending:    make(map[string]bool),
		hasMore:    true,
	}
}

func (m *MessagesListModel) Init() tea.Cmd { return nil }

func (m *MessagesListModel) SetEngaged(engaged bool) tea.Cmd {
	m.engaged = engaged
	return nil
}

func (m *MessagesListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ChatSelectedMsg:
		// reset state
		m.chatId = msg.ChatId
		m.messages = []storage.Message{}
		m.cursor = 0
		m.startIndex = 0
		m.hasMore = true
		m.isLoading = true
		return m, m.loadMessagesFromDb(msg.ChatId, 0)
	case MessagesLoadedMsg:
		if m.chatId == msg.ChatId {
			m.isLoading = false

			// reverse array
			var normalized []storage.Message
			for i := len(msg.Messages) - 1; i >= 0; i-- {
				normalized = append(normalized, msg.Messages[i])
			}

			if !msg.IsAppend {
				m.messages = normalized
				m.cursor = max(0, len(m.messages)-1)
			} else {
				m.messages = append(normalized, m.messages...)
				m.cursor += len(normalized)
			}

			m.hasMore = msg.HasMore

			// send all unsent
			for _, locMsg := range m.messages {
				if locMsg.IsPending {
					if m.sending[locMsg.Id] {
						continue
					}
					m.sending[locMsg.Id] = true
					cmds = append(cmds, m.sendToServer(locMsg, 0))
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
			// tail -f
			if m.cursor == len(m.messages)-2 {
				m.cursor++
			}
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
			return m, func() tea.Msg { return ChatModelHandleErrorMsg{Err: err} }
		}

		m.messages = append(m.messages, newMsg)
		// tail -f
		if m.cursor == len(m.messages)-2 {
			m.cursor++
		}
		m.sending[newMsg.Id] = true
		return m, m.sendToServer(newMsg, 0)

	case TriggerDeliveryMsg:
		return m, m.sendToServer(msg.Message, msg.RetryCount)

	case DeliverySuccessMsg:
		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()
		// ignore if message was not acked on local repo
		_ = m.appContext.LocalRepo.MarkMessageDelivered(
			ctx, msg.LocalId, msg.ServerId,
		)

		delete(m.sending, msg.LocalId)

		for i, locMsg := range m.messages {
			if locMsg.Id == msg.LocalId {
				m.messages[i].IsPending = false
				m.messages[i].Id = msg.ServerId
				break
			}
		}
		return m, nil

	case DeliveryRetryMsg:
		m.logger.Warn(
			"message delivery failed",
			zap.Int("attempt", msg.RetryCount),
			zap.Error(msg.Err),
		)

		if msg.RetryCount >= 5 {
			delete(m.sending, msg.Message.Id)
			return m, func() tea.Msg {
				return ChatModelHandleErrorMsg{
					Err: fmt.Errorf("failed to send message after 5 attempts: %w", msg.Err),
				}
			}
		}

		retryCmd := func() tea.Msg {
			time.Sleep(tui.CalculateBackoff(msg.RetryCount))
			return TriggerDeliveryMsg{
				Message: msg.Message, RetryCount: msg.RetryCount + 1,
			}
		}
		return m, retryCmd
	case tea.KeyPressMsg:
		if m.chatId == "" {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.messages)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor == 0 && !m.isLoading && m.hasMore {
					m.isLoading = true
					return m, m.loadMessagesFromDb(m.chatId, len(m.messages))
				}
			}
		case "enter":
			if len(m.messages) > 0 {
				selected := m.messages[m.cursor]
				if selected.IsPending {
					return m, func() tea.Msg {
						return ChatModelHandleErrorMsg{
							Err: errors.New("message is pending, no info available"),
						}
					}
				}

				return m, func() tea.Msg {
					return MessageSelectedMsg{MsgId: selected.Id}
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *MessagesListModel) View() tea.View {
	return tea.NewView(m.ContentView(500, 500, false))
}


func (m *MessagesListModel) ContentView(
	width int, height int, focused bool,
) string {
	borderColor := "#3C3C3C"
	if m.engaged && focused {
		borderColor = "#FF007F"
	} else if focused {
		borderColor = "#00FF00"
	}

	style := lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	if m.chatId == "" {
		return style.Render("Select a chat to start messaging")
	}

	if len(m.messages) == 0 {
		if m.isLoading {
			return style.Render("Loading messages...")
		}
		return style.Render("No messages yet")
	}

	innerWidth := max(1, width-6)
	innerHeight := max(1, height-2)

	var rendered []string
	for i, msg := range m.messages {
		isFocused := m.engaged && focused && i == m.cursor
		rendered = append(rendered, m.renderMessage(msg, innerWidth, isFocused))
	}

	if m.cursor < m.startIndex {
		m.startIndex = m.cursor
	}

	for {
		totalHeight := 0
		for i := m.startIndex; i <= m.cursor; i++ {
			totalHeight += lipgloss.Height(rendered[i])
		}
		if totalHeight > innerHeight && m.startIndex < m.cursor {
			m.startIndex++
		} else {
			break
		}
	}

	totalHeight := 0
	var visible []string
	for i := m.startIndex; i < len(rendered); i++ {
		h := lipgloss.Height(rendered[i])
		if totalHeight+h > innerHeight {
			break
		}
		visible = append(visible, rendered[i])
		totalHeight += h
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, visible...))
}

func (m *MessagesListModel) renderMessage(
	msg storage.Message, width int, isFocused bool,
) string {
	isUser := msg.SenderId == m.appContext.Session.GetUserId()
	msgBorderColor := lipgloss.Color("#555555")
	if isFocused {
		msgBorderColor = lipgloss.Color("#00FFFF")
	} else if isUser {
		msgBorderColor = lipgloss.Color("#FFFF00")
	}

	statusIndicator := "[OK]"
	if msg.IsPending {
		statusIndicator = "[pending...]"
	}

	timestamp := msg.SentAt.Local().Format("01/02 15:04")
	header := fmt.Sprintf("%s | %s %s", msg.SenderId, timestamp, statusIndicator)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#888888")).
		MarginBottom(1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(msgBorderColor).
		Width(width-2).
		Padding(0, 1)

	contentWrap := lipgloss.NewStyle().
		Width(width - 4).
		Render(string(msg.Content))

	fullBody := lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(header),
		contentWrap,
	)

	return boxStyle.Render(fullBody)
}

func (m *MessagesListModel) loadMessagesFromDb(chatId string, offset int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()

		msgs, err := m.appContext.LocalRepo.GetMessagesByChatId(
			ctx, chatId, m.limit, offset,
		)
		if err != nil {
			m.logger.Error("failed loading messages from DB", zap.Error(err))
			return ChatModelHandleErrorMsg{Err: err}
		}

		return MessagesLoadedMsg{
			ChatId:   chatId,
			Messages: msgs,
			IsAppend: offset > 0,
			HasMore:  len(msgs) == m.limit,
		}
	}
}

func (m *MessagesListModel) sendToServer(
	msg storage.Message, retryCount int,
) tea.Cmd {
	return func() tea.Msg {
		m.logger.Debug(
			"sending message to server",
			zap.String("id", msg.Id),
			zap.Int("attempt", retryCount),
		)

		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 5*time.Second)
		defer cancel()

		res, err := m.appContext.MessagingClient.SendMessage(
			ctx,
			&pb.SendMessageRequest{
				ChatId:  msg.ChatId,
				Content: msg.Content,
			},
		)

		if err != nil && tui.IsRetriable(err) {
			return DeliveryRetryMsg{
				Message: msg, Err: err, RetryCount: retryCount,
			}
		}

		return DeliverySuccessMsg{
			LocalId:  msg.Id,
			ServerId: res.MsgId,
		}
	}
}

func (m *MessagesListModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "Enter", Description: "Show info"},
		{Key: "k, ^", Description: "Cursor up"},
		{Key: "j, v", Description: "Cursor down"},
	}
}
