package messaging

import (
	"database/sql"
	"time"
)

type Chat struct {
	Id      string `db:"id"`
	Name    string `db:"name"`
	IsGroup bool   `db:"is_group"`
}

type Message struct {
	Id       string    `db:"id"`
	ChatId   string    `db:"chat_id"`
	SenderId string    `db:"sender_id"`
	Content  []byte    `db:"content"`
	SentAt   time.Time `db:"sent_at"`
}

type MessageAck struct {
	ChatId      string       `db:"chat_id"`
	MessageId   string       `db:"message_id"`
	UserId      string       `db:"user_id"`
	DeliveredAt sql.NullTime `db:"delivered_at"`
	ReadAt      sql.NullTime `db:"read_at"`
}
