package storage

import "time"

// Message local representation of messaging service message.
// This is the only persisted item for easier consistency with server
// storage
type Message struct {
	Id        string    `db:"id"`
	ChatId    string    `db:"chat_id"`
	SenderId  string    `db:"sender_id"`
	Content   []byte    `db:"content"`
	// SentAt stores when client sent this message
	SentAt    time.Time `db:"sent_at"`
	// IsPending message is marked pending until it is not received by server
	// e.g. Network loss
	IsPending bool      `db:"is_pending"`
	// IsRead local helper to track which message were read by user
	IsRead    bool      `db:"is_read"`
}
