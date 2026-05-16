package storage

import (
	"context"
)

type Repository interface {
	InitializeSchema(ctx context.Context) error
	
	// InsertMessage saves a new message
	InsertMessage(ctx context.Context, m Message) error
	
	// GetMessagesByChatId returns messages for a specific chat, 
	// ordered by newest first (DESC) for pagination.
	GetMessagesByChatId(
		ctx context.Context, chatId string, limit, offset int,
	) ([]Message, error)
	
	Close() error
}
