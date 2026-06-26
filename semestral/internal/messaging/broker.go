package messaging

import (
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"
)


// UserSession represents active/subscribed user 
type UserSession struct {
	// id is UserSession id (uuid used)
	id          string

	// messageChan channel to pass messages to given user.
	//
	// Reader end of the channel is used in routine handling user subscription stream 
	// Writer end is used in Publish method and respectively SendMessage rpc
	messageChan chan *Message
	
	// done is closed to identify that given session is terminated and no new messages
	// will be sent to messageChan. 
	done        chan struct{}

	// closeOnce is guard to close done channel
	closeOnce   sync.Once
}

// messageTarget internal struct to identify message target
type messageTarget struct {
	userId  string
	session *UserSession
}

// newUserSession creates new sesssion with buffered channel, allow 30 messages 
func newUserSession() *UserSession {
	return &UserSession{
		id:          uuid.New().String(),
		messageChan: make(chan *Message, 30),
		done:        make(chan struct{}),
		closeOnce:   sync.Once{},
	}
}

func (s *UserSession) GetMessageChan() <-chan *Message {
	return s.messageChan
}

// Done returns a channel will be closed when the session is force killed
func (s *UserSession) Done() <-chan struct{} {
	return s.done
}

// Close safely closes session, signals without panicking 
// (e.g. if multiple routines try too close session because of slow client)
func (s *UserSession) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}


// Broker handles multiplexing and routing of messages to users/routines
// handling user subscription stream 
type Broker struct {
	// sessions map of session per user, user can have multiple sessions
	sessions   map[string][]*UserSession
	sessionsMu sync.RWMutex

	logger *zap.Logger
}


// NewBroker constructs new broker instance
func NewBroker(logger *zap.Logger) *Broker {
	return &Broker{
		sessions:   make(map[string][]*UserSession, 10),
		sessionsMu: sync.RWMutex{},
		logger:     logger,
	}
}

// Subscribe create new session for given user
func (b *Broker) Subscribe(userId string) *UserSession {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	userSession := newUserSession()

	b.sessions[userId] = append(b.sessions[userId], userSession)

	b.logger.Info("client subscribed to broker session",
		zap.String("userId", userId),
		zap.String("sessionId", userSession.id),
		zap.Int("activeUserSessions", len(b.sessions[userId])),
	)

	return userSession
}

// Unsubscribe removes session for user
func (b *Broker) Unsubscribe(userId string, targetSession *UserSession) {
	b.sessionsMu.Lock()
	defer b.sessionsMu.Unlock()

	userSessions, ok := b.sessions[userId]
	if !ok {
		// no sessions present.
		return
	}

	filtered := make([]*UserSession, 0, len(userSessions))
	for _, session := range userSessions {
		if session.id != targetSession.id {
			filtered = append(filtered, session)
		}
	}

	if len(filtered) == 0 {
		delete(b.sessions, userId)
		b.logger.Info(
			"all sessions removed for user, clearing map entry",
			zap.String("userId", userId),
		)
	} else {
		b.sessions[userId] = filtered
		b.logger.Debug(
			"removed single session for user",
			zap.String("userId", userId),
			zap.String("sessionId", targetSession.id),
		)
	}
}

// Publish constructs message targets (it is all active user sessions)
// from acks object under read lock and sends those corresponding channels
//
// NOTE: channels write is unblocking, slow clients session is force closed,
// no messages are lost at this point as they are store in db
func (b *Broker) Publish(message *Message, acks []MessageAck) {
	// make copy of all target sessions under read lock
	b.sessionsMu.RLock()

	var targets []messageTarget
	for _, ack := range acks {
		if userSessions, online := b.sessions[ack.UserId]; online {
			for _, session := range userSessions {
				targets = append(
					targets,
					messageTarget{userId: ack.UserId, session: session},
				)
			}
		}
	}
	b.sessionsMu.RUnlock()

	// broadcast messages using non-blocking selection
	for _, t := range targets {
		select {
		case <-t.session.Done():
			// session already closed
			continue

		case t.session.messageChan <- message:
			// message written
		default:
			// Session buffer full, message remains in DB and will
			// be delivered later after reconnect.

			// In my impl that is the simplest way to make client 
			// keep up with whats happening
			// and build history/UI of messages properly
			b.logger.Warn("killing slow client session",
				zap.String("userId", t.userId),
				zap.String("sessionId", t.session.id),
				zap.String("messageId", message.Id),
			)

			// chan reader should react on chan closing and call Unsubscribe
			t.session.Close()
		}
	}
}
