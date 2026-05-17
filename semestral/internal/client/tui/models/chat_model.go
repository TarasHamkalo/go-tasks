package models

import (
	"context"
	"errors"
	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"google.golang.org/grpc"
)

// messages
type IncomingMessageMsg struct {
	Message storage.Message
}

type SubscriptionStartedMsg struct{}

type SubscriptionErrorMsg struct {
	Err error
}

type ReconnectMsg struct{}

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

	focusArea FocusArea
	isEngaged bool
}

func NewChatModel(appContext *state.AppContext) *ChatModel {
	return &ChatModel{
		appContext: appContext,
		focusArea:  FocusChatsList,
		isEngaged:  false,
	}
}

func (m *ChatModel) Id() SubModelId {
	return ScreenActiveChat
}

func (m *ChatModel) Init() tea.Cmd {
	return m.subscribe(0)
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

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case SubscriptionStartedMsg:
		// start waiting for the first message
		return m, m.recvMessage()

	case IncomingMessageMsg:
		// Continue listening.
		_ = msg
			// here is failure already
			// m.appContext.Session.IncrementUnread(msg.ChatId)
		// TODO: delegate this message to chat list model (can pull new chat)
		// TODO: delegate this message to message list model 

		return m, m.recvMessage()

	case SubscriptionErrorMsg:
		// Retry after delay.
		return m, m.subscribe(5 * time.Second)

	case ReconnectMsg:
		// stream closed cleanly by server, reconnect after delay.
		// TODO: exponential delay up to max
		return m, m.subscribe(5 * time.Second)
	}

	return m, nil
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
