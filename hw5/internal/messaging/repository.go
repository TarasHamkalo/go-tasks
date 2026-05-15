package messaging

import (
	"context"
	"time"
)

type Repository interface {
	InitializeSchema(ctx context.Context) error
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
	InsertMessageWithAcks(
		ctx context.Context, message *Message, acks []MessageAck,
	) error

	// Delivery/read tracking
	InsertMessageAcks(ctx context.Context, acks []MessageAck) error
	GetUndeliveredMessages(
		ctx context.Context, userId string,
	) ([]Message, error)

	AcknowledgeAndCleanupMessage(
		ctx context.Context,
		messageId string,
		userId string,
		deliveredAt time.Time,
	) error

	SetMessageRead(
		ctx context.Context,
		messageId string,
		userId string,
		readAt time.Time,
	) error

	GetMessageAcks(
		ctx context.Context,
		messageId string,
	) ([]MessageAck, error)
}
