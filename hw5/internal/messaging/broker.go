package messaging

import (
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type UserSession struct {
	id          string
	messageChan chan *Message
}

func newUserSession() *UserSession {
	return &UserSession{
		id:          uuid.New().String(),
		messageChan: make(chan *Message, 50),
	}
}

func (s *UserSession) GetMessageChan() <-chan *Message {
	return s.messageChan
}

type Broker struct {
	sessions   map[string][]*UserSession
	sessionsMu sync.RWMutex

	shutdownChan chan struct{}

	logger *zap.Logger
}

func NewBroker(logger *zap.Logger) *Broker {
	return &Broker{
		sessions:   make(map[string][]*UserSession, 10),
		sessionsMu: sync.RWMutex{},

		shutdownChan: make(chan struct{}),

		logger: logger,
	}
}

func (b *Broker) Subscribe(userId string) *UserSession {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	userSession := newUserSession()
	userSessions, ok := b.sessions[userId]
	if !ok {
		userSessions = []*UserSession{}
	}

	userSessions = append(userSessions, userSession)
	b.sessions[userId] = userSessions
	return userSession
}

func (b *Broker) Unsubscribe(userId string, targetSession *UserSession) {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	userSessions, ok := b.sessions[userId]
	if !ok {
		// okay, session is not present, accept it
		return
	}

	filtered := make([]*UserSession, len(userSessions))
	for _, session := range userSessions {
		if session.id != targetSession.id {
			filtered = append(filtered, session)
		}
	}
}

func (b *Broker) Publish(message *Message, acks []MessageAck) {
	var targets []*UserSession
	b.sessionsMu.RLock()
	for _, ack := range acks {
		userSessions, ok := b.sessions[ack.UserId]
		if !ok {
			continue
		}

		targets = append(targets, userSessions...)
	}

	b.sessionsMu.RUnlock()

	for _, session := range targets {
		select {
		case session.messageChan <- message:
		default:
			// Session buffer full, message remains in DB and will
			// be delivered later after reconnect. 

			// In my impl that simplest way to make client keep up with whats happenning
			// and build history/UI of messages properly
			b.logger.Warn("killing slow client session", zap.String("sessionId", session.id))
			close(session.messageChan)
		}
	}
}
