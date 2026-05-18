package messaging

import (
	"context"
	"time"
)

// Repository defines persistent storage operations for chats,
// chat members, messages, and delivery/read acknowledgements.
type Repository interface {
	// InitializeSchema creates database tables if they do not already exist.
	InitializeSchema(ctx context.Context) error

	// Close safely terminates database connections and frees allocated engine resources.
	Close() error

	// Chats
	GetChatById(ctx context.Context, chatId string) (Chat, error)
	GetDirectChatByUsers(
		ctx context.Context, userIdA string, userIdB string,
	) (Chat, error)

	InsertChat(ctx context.Context, chat Chat, memberIds []string) error
	GetUserChats(ctx context.Context, userId string) ([]Chat, error)

	// Chat members
	AddChatMember(ctx context.Context, chatId, userId string) error
	RemoveChatMember(ctx context.Context, chatId, userId string) error
	GetChatMembers(ctx context.Context, chatId string) ([]string, error)

	// Messages
	InsertMessage(ctx context.Context, message *Message) error

	// InsertMessageWithAcks unifies message and acks insert under one transaction.
	InsertMessageWithAcks(
		ctx context.Context, message *Message, acks []MessageAck,
	) error

	// Delivery/read tracking
	InsertMessageAcks(ctx context.Context, acks []MessageAck) error

	// GetUndeliveredMessages returns all yet undelivered 
	// messages for target user
	GetUndeliveredMessages(
		ctx context.Context, userId string,
	) ([]Message, error)
	
	// AcknowledgeAndCleanupMessage set message delivery time, that is
	// client successfully stored message locally and removes message from db
	// if all target recipients acknowledged delivery.
	// Should be executed under transaction
	AcknowledgeAndCleanupMessage(
		ctx context.Context,
		messageId string,
		userId string,
		deliveredAt time.Time,
	) error

	// SetMessagesRead set read status for batch of messages
	SetMessagesRead(
		ctx context.Context,
		messageIds []string,
		userId string,
		readAt time.Time,
	) error

	GetMessageAcks(
		ctx context.Context,
		messageId string,
	) ([]MessageAck, error)
}
