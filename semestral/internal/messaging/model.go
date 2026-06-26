package messaging

import (
	"database/sql"
	"time"
)

// Chat represents message destination, each message is associated with 
// target chat and sender
type Chat struct {
	Id      string `db:"id"`
	Name    string `db:"name"`
	// IsGroup if false, indicates direct chat
	IsGroup bool   `db:"is_group"`
}

// Message represents user sent 
//
// NOTE: message is stored until all target users, it is members 
// of chat specfied by ChatId don't acknowledge message delivery, see
// MessageAck
type Message struct {
	Id       string    `db:"id"`
	ChatId   string    `db:"chat_id"`
	SenderId string    `db:"sender_id"`
	Content  []byte    `db:"content"`
	SentAt   time.Time `db:"sent_at"`
}

// MessageAck track message delivery state as well user "view/read" time 
// of given message. 
//
// NOTE: clients should always ack delivery for server to remove message from
// local store. Implementation of "invisible" mode just does not send "Read" message,
// which is not used by messaging service in any form.
// 
// NOTE: this object is persisted all the time, not removal is handled at the moment
type MessageAck struct {
	ChatId      string       `db:"chat_id"`
	MessageId   string       `db:"message_id"`
	UserId      string       `db:"user_id"`
	DeliveredAt sql.NullTime `db:"delivered_at"`
	ReadAt      sql.NullTime `db:"read_at"`
}
