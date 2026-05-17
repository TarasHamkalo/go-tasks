package models

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
	"gomessenger/internal/client/tui"
	"gomessenger/internal/client/tui/components"
)

// Messages
type ChatModelHandleErrorMsg struct{ Err error }

// Handle server stream connection
type SubscriptionStartedMsg struct{}
type SubscriptionErrorMsg struct{ Err error }

// TODO: Reconnect retry count is not used at the moment
type ReconnectMsg struct{ RetryCount int }

type IncomingMessageMsg struct{ Message storage.Message }

// Handle message acks
type RetryAckMsg struct {
	MsgId      string
	RetryCount int
}

type AckSuccessMsg struct{ MsgId string }

type FocusArea int

const (
	FocusProfile FocusArea = iota
	FocusChatsList
	FocusMessageList
	FocusMessageInput
)

type ChatModel struct {
	appContext *state.AppContext

	messageStream grpc.ServerStreamingClient[pb.ServerEvent]

	focusedArea FocusArea
	isEngaged   bool

	activeModel SectionModel

	messageInputSection *MessagesInputModel
	messagesListModel   *MessagesListModel
	chatsListModel      *ChatsListModel

	logger *zap.Logger
}

func NewChatModel(appContext *state.AppContext) *ChatModel {
	return &ChatModel{
		appContext: appContext,

		focusedArea: FocusProfile,
		isEngaged:   false,

		messageInputSection: NewMessageInputModel(),
		messagesListModel:   NewMessagesListModel(appContext),
		chatsListModel:      NewChatsListModel(appContext),

		logger: appContext.RootLogger.With(zap.String("mvc", "chat")),
	}
}

func (m *ChatModel) Id() SubModelId {
	return ScreenActiveChat
}

func (m *ChatModel) Init() tea.Cmd {
	return m.subscribe(0)
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("handling message", zap.String("type", fmt.Sprintf("%T", msg)))

	// handle global errors
	if errMsg, ok := msg.(ChatModelHandleErrorMsg); ok {
		m.logger.Error("chat model encountered fatal error", zap.Error(errMsg.Err))
		errModel := NewErrorSubModel(errMsg.Err, m)
		return errModel, errModel.Init()
	}

	// handle network / stream events
	if model, cmd, handled := m.handleNetworkEvents(msg); handled {
		return model, cmd
	}

	// handle keyboard routing
	if !m.isEngaged {
		return m.handleOwnKeys(msg)
	}

	return m.handleComponentRouting(msg)
}

func (m *ChatModel) handleNetworkEvents(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case SubscriptionStartedMsg:
		m.logger.Info("subscription stream started")
		return m, m.recvMessage(), true

	case IncomingMessageMsg:
		model, cmd := m.handleIncomingMessage(msg)
		return model, cmd, true

	case SubscriptionErrorMsg:
		m.logger.Error("subscription error", zap.Error(msg.Err))
		return m, func() tea.Msg { return ChatModelHandleErrorMsg(msg) }, true

	case RetryAckMsg:
		return m, m.ackMessage(msg.MsgId, msg.RetryCount), true

	case AckSuccessMsg:
		m.logger.Debug("message acked successfully", zap.String("msgId", msg.MsgId))
		return m, nil, true

	case ReconnectMsg:
		if msg.RetryCount >= 5 {
			return m, func() tea.Msg {
				return ChatModelHandleErrorMsg{
					Err: errors.New("max reconnect attempts reached, restart program or continue offline"),
				}
			}, true
		}

		delay := tui.CalculateBackoff(msg.RetryCount)
		m.logger.Warn(
			"reconnecting stream",
			zap.Duration("delay", delay),
			zap.Int("attempt", msg.RetryCount),
		)
		return m, m.subscribe(delay), true

	// Forward these specific messages down to the messages list
	case TriggerDeliveryMsg,
		ChatSelectedMsg,
		MessagesLoadedMsg,
		DeliverySuccessMsg,
		DeliveryRetryMsg,
		MessageInputSubmittedMsg:
		model, cmd := m.messagesListModel.Update(msg)
		m.messagesListModel = model.(*MessagesListModel)
		return m, cmd, true
	}

	return m, nil, false
}

func (m *ChatModel) handleComponentRouting(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if ok && keyMsg.String() == "esc" {
		m.isEngaged = false
		if m.activeModel != nil {
			m.activeModel.SetEngaged(false)
		}
		return m, nil
	}

	var model tea.Model
	var cmd tea.Cmd

	switch m.focusedArea {
	case FocusProfile:
		if ok && keyMsg.String() == "enter" {
			m.isEngaged = false
			return NewErrorSubModel(errors.New("not impl"), m), nil
		}

	case FocusMessageInput:
		model, cmd = m.messageInputSection.Update(msg)
		m.messageInputSection = model.(*MessagesInputModel)

	case FocusChatsList:
		model, cmd = m.chatsListModel.Update(msg)
		m.chatsListModel = model.(*ChatsListModel)

	case FocusMessageList:
		model, cmd = m.messagesListModel.Update(msg)
		m.messagesListModel = model.(*MessagesListModel)
	}

	return m, cmd
}

func (m *ChatModel) handleIncomingMessage(
	msg IncomingMessageMsg,
) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	chatListModel, chatListCmd := m.chatsListModel.Update(msg)
	m.chatsListModel = chatListModel.(*ChatsListModel)
	if chatListCmd != nil {
		cmds = append(cmds, chatListCmd)
	}

	// TODO: increment unread
	msgListModel, msgListCmd := m.messagesListModel.Update(msg)
	m.messagesListModel = msgListModel.(*MessagesListModel)
	if msgListCmd != nil {
		cmds = append(cmds, msgListCmd)
	}

	cmds = append(cmds, m.ackMessage(msg.Message.Id, 0))
	cmds = append(cmds, m.recvMessage())

	return m, tea.Batch(cmds...)
}

func (m *ChatModel) handleOwnKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "shift+tab":
			m.focusedArea = (m.focusedArea + 3) % 4
		case "tab":
			m.focusedArea = (m.focusedArea + 1) % 4
		case "enter":
			m.isEngaged = true

			switch m.focusedArea {
			case FocusChatsList:
				m.activeModel = m.chatsListModel
			case FocusMessageList:
				m.activeModel = m.messagesListModel
			case FocusMessageInput:
				m.activeModel = m.messageInputSection
			}

			if m.activeModel != nil {
				m.activeModel.SetEngaged(true)
			}
		}
	}
	return m, nil
}

func (m *ChatModel) View() tea.View { return m.ContentView(80, 24) }

func (m *ChatModel) ContentView(width, height int) tea.View {
	profileHeight := 5
	messageInputHeight := 5

	leftWidth := int(float64(width) * 0.30)
	rightWidth := width - leftWidth

	chatsHeight := height - profileHeight
	messageListHeight := height - messageInputHeight

	p, _ := m.appContext.Session.GetCurrentUserProfile()
	profileView := components.RenderProfileSection(p, leftWidth, profileHeight, m.focusedArea == FocusProfile, m.isEngaged)
	chatListView := m.chatsListModel.ContentView(leftWidth, chatsHeight, m.focusedArea == FocusChatsList)
	messageListView := m.messagesListModel.ContentView(rightWidth, messageListHeight, m.focusedArea == FocusMessageList)
	messageInputView := m.messageInputSection.ContentView(rightWidth, messageInputHeight, m.focusedArea == FocusMessageInput)

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, profileView, chatListView)
	rightPanel := lipgloss.JoinVertical(lipgloss.Left, messageListView, messageInputView)
	mainLayout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	return tea.NewView(mainLayout)
}

func (m *ChatModel) ShortHelp() []tui.Binding {
	if m.isEngaged {
		bindings := []tui.Binding{}
		if m.activeModel != nil {
			bindings = append(bindings, m.activeModel.ShortHelp()...)
		}

		if m.focusedArea == FocusProfile {
			bindings = append(
				bindings,
				tui.Binding{Key: "Enter", Description: "Open user profile"},
			)
		}

		return append(
			bindings, tui.Binding{Key: "Esc", Description: "Unfocus Section"},
		)
	}

	return []tui.Binding{
		{Key: "Shift+Tab", Description: "Previous section"},
		{Key: "Tab", Description: "Next section"},
		{Key: "Enter", Description: "Interact"},
		{Key: "d/g", Description: "New Direct/Group"},
		{Key: "m/i", Description: "Members/Invite"},
	}
}

func (m *ChatModel) subscribe(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			time.Sleep(delay) // tea.Cmd runs in goroutine, sleep is safe here
		}

		m.logger.Info("attempting to subscribe to stream...")
		stream, err := m.appContext.MessagingClient.Subscribe(
			m.appContext.Ctx, &pb.SubscribeRequest{},
		)
		if err != nil {
			return SubscriptionErrorMsg{Err: err}
		}

		m.messageStream = stream
		return SubscriptionStartedMsg{}
	}
}

func (m *ChatModel) recvMessage() tea.Cmd {
	return func() tea.Msg {
		if m.messageStream == nil {
			return SubscriptionErrorMsg{
				Err: errors.New("subscription stream is nil"),
			}
		}

		res, err := m.messageStream.Recv()
		if err != nil {
			if err == io.EOF {
				return ReconnectMsg{RetryCount: 0}
			}
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return SubscriptionErrorMsg{Err: err}
		}

		switch event := res.Event.(type) {
		case *pb.ServerEvent_IncomingMessage:
			msg := ToStorageMessage(event.IncomingMessage)
			ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
			defer cancel()

			if err := m.appContext.LocalRepo.InsertMessage(ctx, &msg); err != nil {
				return ChatModelHandleErrorMsg{
					Err: fmt.Errorf("failed to save incoming message: %w", err),
				}
			}
			return IncomingMessageMsg{Message: msg}
		}

		return ReconnectMsg{RetryCount: 0}
	}
}

func ToStorageMessage(msg *pb.IncomingMessage) storage.Message {
	return storage.Message{
		Id:       msg.Id,
		ChatId:   msg.ChatId,
		SenderId: msg.SenderId,
		Content:  msg.Content,
		SentAt:   msg.SentAt.AsTime(),
	}
}

func (m *ChatModel) ackMessage(msgId string, retryCount int) tea.Cmd {
	return func() tea.Msg {
		if retryCount > 0 {
			time.Sleep(tui.CalculateBackoff(retryCount))
		}

		m.logger.Debug(
			"sending ack",
			zap.String("msgId", msgId),
			zap.Int("attempt", retryCount),
		)

		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()

		_, err := m.appContext.MessagingClient.AckMessage(
			ctx, &pb.AckMessageRequest{MsgId: msgId},
		)

		if err != nil {
			if retryCount >= 5 {
				return ChatModelHandleErrorMsg{
					Err: fmt.Errorf("failed to ack message after 5 attempts: %w", err),
				}
			}
			return RetryAckMsg{MsgId: msgId, RetryCount: retryCount + 1}
		}

		return AckSuccessMsg{MsgId: msgId}
	}
}
