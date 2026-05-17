package models

import (
	"context"
	"errors"
	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
	"gomessenger/internal/client/tui"
	"gomessenger/internal/client/tui/components"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"google.golang.org/grpc"
)

// messages
type ChatModelHandleErrorMsg struct {
	Err error
}

type IncomingMessageMsg struct{ Message storage.Message }

type SubscriptionStartedMsg struct{}
type SubscriptionErrorMsg struct{ Err error }

type ReconnectMsg struct{}

type RetryAckMsg struct{ MsgId string }
type AckSuccessMsg struct{ MsgId string }

type FocusArea int

const (
	FocusProfile FocusArea = iota
	FocusChatsList
	FocusActiveChat
	FocusMessageInput
)

type ChatModel struct {
	appContext *state.AppContext

	messageStream grpc.ServerStreamingClient[pb.ServerEvent]

	focusedArea FocusArea
	isEngaged   bool

	messageInputSection *MessagesInputModel

	messagesListModel *MessagesListModel
	chatsListModel     *ChatsListModel
}

func NewChatModel(appContext *state.AppContext) *ChatModel {
	return &ChatModel{
		appContext:  appContext,
		focusedArea: FocusProfile,
		isEngaged:   false,

		messageInputSection: NewMessageInputModel(),
		messagesListModel:   NewMessagesListModel(appContext),
		chatsListModel:       NewChatsListModel(appContext),
	}
}

func (m *ChatModel) Id() SubModelId {
	return ScreenActiveChat
}

func (m *ChatModel) Init() tea.Cmd {
	return m.subscribe(0)
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case ChatModelHandleErrorMsg:
		errModel := NewErrorSubModel(msg.Err, m)
		return errModel, errModel.Init()

	// MessagesListModel handles these messages.
	case TriggerDeliveryMsg,
		ChatSelectedMsg,
		MessagesLoadedMsg,
		DeliverySuccessMsg,
		DeliveryRetryMsg,
		MessageInputSubmittedMsg:

		model, cmd := m.messagesListModel.Update(msg)
		m.messagesListModel = model.(*MessagesListModel)
		return m, cmd

	case SubscriptionStartedMsg:
		// start waiting for the first message
		return m, m.recvMessage()

	case IncomingMessageMsg:
		// TODO: this handles message list model m.appContext.Session.IncrementUnread(msg.ChatId)
		return m.handleIncommingMessage(msg)

	case SubscriptionErrorMsg:
		// Retry after delay.
		return m, m.subscribe(5 * time.Second)

	case RetryAckMsg:
		return m, m.ackMessage(5*time.Second, msg.MsgId)

	case AckSuccessMsg:
		return m, nil

	case ReconnectMsg:
		// stream closed cleanly by server, reconnect after delay.
		// TODO: exponential delay up to max
		return m, m.subscribe(5 * time.Second)
	}

	if !m.isEngaged {
		return m.handleOwnKeys(msg)
	}

	// break current section
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if ok && keyMsg.String() == "esc" {
		m.isEngaged = false
		// blur section if needed.
		switch m.focusedArea {
		case FocusChatsList:
			m.chatsListModel.SetEngaged(false)
		case FocusActiveChat:
			m.messagesListModel.SetEngaged(false)
		case FocusMessageInput:
			m.messageInputSection.SetEngaged(false)
		}

		return m, nil
	}

	// forward to sections
	var model tea.Model
	var cmd tea.Cmd

	switch m.focusedArea {
	case FocusProfile:
		m.isEngaged = false
		return NewErrorSubModel(errors.New("not impl"), m), nil

	case FocusMessageInput:
		model, cmd = m.messageInputSection.Update(msg)
		m.messageInputSection = model.(*MessagesInputModel)

	case FocusChatsList:
		model, cmd = m.chatsListModel.Update(msg)
		m.chatsListModel = model.(*ChatsListModel)

	case FocusActiveChat:
		model, cmd = m.messagesListModel.Update(msg)
		m.messagesListModel = model.(*MessagesListModel)
	}

	return m, cmd
}

// Forward the incoming message to:
//  1. Chats list (may fetch metadata for a new chat).
//  2. Messages list (append to currently opened chat).
//  3. ACK sender.
//  4. Next stream receive.
func (m *ChatModel) handleIncommingMessage(
	msg IncomingMessageMsg,
) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Update chats list.
	chatListModel, chatListCmd := m.chatsListModel.Update(msg)
	m.chatsListModel = chatListModel.(*ChatsListModel)
	if chatListCmd != nil {
		cmds = append(cmds, chatListCmd)
	}

	// Update messages list.
	msgListModel, msgListCmd := m.messagesListModel.Update(msg)
	m.messagesListModel = msgListModel.(*MessagesListModel)
	if msgListCmd != nil {
		cmds = append(cmds, msgListCmd)
	}

	// ack delivery to server.
	cmds = append(cmds, m.ackMessage(0, msg.Message.Id))
	// continue reading from stream.
	cmds = append(cmds, m.recvMessage())

	return m, tea.Batch(cmds...)
}

func (m *ChatModel) handleOwnKeys(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "shift+tab":
			m.focusedArea = (m.focusedArea + 3) % 4
			return m, nil
		case "tab":
			m.focusedArea = (m.focusedArea + 1) % 4
			return m, nil
		case "enter":
			m.isEngaged = true
			switch m.focusedArea {
			case FocusMessageInput:
				m.messageInputSection.SetEngaged(true)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *ChatModel) View() tea.View { return m.ContentView(80, 24) }

func (m *ChatModel) ContentView(width, height int) tea.View {
	leftWidth := int(float64(width) * 0.30)
	rightWidth := width - leftWidth
	profileHeight := 5
	chatsHeight := height - profileHeight

	profileFocused := m.focusedArea == FocusProfile
	chatsFocused := m.focusedArea == FocusChatsList
	activeChatFocused := m.focusedArea == FocusActiveChat

	p, _ := m.appContext.Session.GetCurrentUserProfile()
	// rendering handles nil
	profileView := components.RenderProfileSection(
		p, leftWidth, profileHeight, profileFocused, m.isEngaged,
	)
	// chatsListView := m.chatsListSection.View(leftWidth, chatsHeight, chatsFocused, m.isEngaged)
	// activeChatView := m.activeChatSection.View(rightWidth, height, activeChatFocused, m.isEngaged)
	//
	// leftPanel := lipgloss.JoinVertical(lipgloss.Left, profileView, chatsListView)
	// mainLayout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, activeChatView)

	// return tea.NewView(mainLayout)
}

func (m *ChatModel) ShortHelp() []tui.Binding {
	if m.isEngaged {
		bindings := []tui.Binding{}
		if m.activeSection != nil {
			bindings = append(bindings, m.ShortHelp()...)
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

// subscribe optionally waits, then opens the gRPC stream.
func (m *ChatModel) subscribe(delay time.Duration) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-m.appContext.Ctx.Done():
				return nil
			}
		}

		// long running, use app context
		stream, err := m.appContext.MessagingClient.Subscribe(
			m.appContext.Ctx,
			&pb.SubscribeRequest{},
		)
		if err != nil {
			return SubscriptionErrorMsg{Err: err}
		}

		m.messageStream = stream
		return SubscriptionStartedMsg{}
	}
}

// recvMessage blocks until:
//   - one message arrives,
//   - stream closes,
//   - or an error occurs.
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
				// possibly client can not keep up with messages and server force
				// killed session
				return ReconnectMsg{}
			}

			// context canceled during shutdown
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
			err := m.appContext.LocalRepo.InsertMessage(ctx, &msg)

			if err != nil {
				return SubscriptionErrorMsg{Err: err}
			}

			return IncomingMessageMsg{
				Message: msg,
			}
		}

		return ReconnectMsg{}
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

func (m *ChatModel) ackMessage(delay time.Duration, msgId string) tea.Cmd {
	return func() tea.Msg {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-m.appContext.Ctx.Done():
				return nil
			}
		}

		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 2*time.Second)
		defer cancel()
		_, err := m.appContext.MessagingClient.AckMessage(
			ctx,
			&pb.AckMessageRequest{
				MsgId: msgId,
			},
		)

		if err != nil {
			return RetryAckMsg{MsgId: msgId}
		}

		return
	}
}
