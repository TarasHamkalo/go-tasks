// Package storage implements local client-side persistence.
package storage

import (
	"context"
)

// Repository defines local storage operations for messages 
type Repository interface {
	InitializeSchema(ctx context.Context) error

	// InsertMessage saves a new message
	InsertMessage(ctx context.Context, m *Message) error

	// GetMessagesByChatId returns messages for a specific chat,
	// ordered by newest first for pagination.
	GetMessagesByChatId(
		ctx context.Context, chatId string, limit, offset int,
	) ([]Message, error)

	// MarkMessageDelivered updates a locally generated message
	// with the server-assigned identifier, it is server stored/received message NOT
	// delivery to recipients
	MarkMessageDelivered(
		ctx context.Context, localId string, serverId string,
	) error

	// MarkMessagesRead marks messages as read locally.
	MarkMessagesRead(ctx context.Context, ids []string) error

	Close() error
}
