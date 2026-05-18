package storage

import "time"

type Message struct {
	Id        string    `db:"id"`
	ChatId    string    `db:"chat_id"`
	SenderId  string    `db:"sender_id"`
	Content   []byte    `db:"content"`
	SentAt    time.Time `db:"sent_at"`
	IsPending bool      `db:"is_pending"`
	IsRead    bool      `db:"is_read"`
}
