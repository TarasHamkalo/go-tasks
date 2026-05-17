package messaging

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	// not sure whether to add foreign keys at all, we can not bound
	// sender id still, so...
	// Just leaving constraints that are possible to verify (inside single db)
	// PRAGMA foreign_keys = ON;
	SCHEMA_QUERY = `

	CREATE TABLE IF NOT EXISTS
		chats(
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				is_group INTEGER NOT NULL
		);

	CREATE TABLE IF NOT EXISTS 
		chat_members(
			chat_id TEXT NOT NULL,
			user_id TEXT NOT NULL, -- FK to profile service users
			PRIMARY KEY (chat_id, user_id),
			FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);

	CREATE TABLE IF NOT EXISTS 
		messages(
				id TEXT PRIMARY KEY,

				chat_id TEXT NOT NULL,
				sender_id TEXT NOT NULL, -- FK to profile service users

				content BLOB NOT NULL,

				sent_at DATETIME NOT NULL,

				FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE
		);

	CREATE TABLE IF NOT EXISTS 
		message_acks(
				message_id TEXT NOT NULL,
				user_id TEXT NOT NULL, 
				chat_id TEXT NOT NULL,

				delivered_at DATETIME,
				read_at DATETIME,

				PRIMARY KEY(message_id, user_id)
		);
	`

	// --- Chats ---
	INSERT_CHAT_QUERY = `
		INSERT INTO chats (id, name, is_group) 
		VALUES (:id, :name, :is_group)
	`

	GET_CHATS_BY_USER_QUERY = `
		SELECT c.id, c.name, c.is_group 
		FROM chats c
		INNER JOIN chat_members cm ON c.id = cm.chat_id
		WHERE cm.user_id = ?
	`

	// --- Chat Members ---
	ADD_CHAT_MEMBER_QUERY = `
		INSERT INTO chat_members (chat_id, user_id) 
		VALUES (:chat_id, :user_id)
		ON CONFLICT(chat_id, user_id) DO NOTHING
	`

	REMOVE_CHAT_MEMBER_QUERY = `
		DELETE FROM chat_members 
		WHERE chat_id = :chat_id AND user_id = :user_id 
	`

	GET_CHAT_MEMBERS_QUERY = `
		SELECT user_id 
		FROM chat_members 
		WHERE chat_id = ?
	`

	// --- Messages ---
	INSERT_MESSAGE_QUERY = `
		INSERT INTO messages (id, chat_id, sender_id, content, sent_at) 
		VALUES (:id, :chat_id, :sender_id, :content, :sent_at)
	`

	// --- Tracking and Delivery ---
	// Note: Used for pulling messages for offline users.
	// Messages are sort by sent_at so the client app appends them in
	// historical order.
	INSERT_MESSAGE_ACK_QUERY = `
		INSERT INTO message_acks (message_id, user_id, chat_id, delivered_at, read_at)
		VALUES (:message_id, :user_id, :chat_id, :delivered_at, :read_at)
	`
	// Query to check if any other recipients are still waiting for this message
	COUNT_PENDING_ACKS_QUERY = `
		SELECT COUNT(*) 
		FROM message_acks 
		WHERE message_id = ? AND delivered_at IS NULL
	`

	GET_UNDELIVERED_MESSAGES_QUERY = `
		SELECT m.id, m.chat_id, m.sender_id, m.content, m.sent_at 
		FROM messages m
		INNER JOIN message_acks ma ON m.id = ma.message_id
		WHERE ma.user_id = ? AND ma.delivered_at IS NULL
		ORDER BY m.sent_at ASC
		LIMIT 100
	`

	SET_MESSAGE_DELIVERED_QUERY = `
		UPDATE message_acks 
		SET delivered_at = ? 
		WHERE message_id = ? AND user_id = ? AND delivered_at IS NULL
	`

	DELETE_MESSAGE_QUERY = `
		DELETE FROM messages 
		WHERE id = ?
	`

	SET_MESSAGE_READ_QUERY = `
		UPDATE message_acks 
		SET read_at = ? 
		WHERE message_id = ? AND user_id = ? AND read_at IS NULL
	`

	GET_MESSAGE_ACKS_QUERY = `
		SELECT message_id, user_id, chat_id, delivered_at, read_at 
		FROM message_acks 
		WHERE message_id = ?
	`

	GET_CHAT_BY_ID_QUERY = `
		SELECT id, name, is_group
		FROM chats
		WHERE id = ?

	`
	GET_DIRECT_CHAT_BY_USERS_QUERY = `
		SELECT c.id, c.name, c.is_group 
		FROM chats c
		INNER JOIN chat_members cm1 ON c.id = cm1.chat_id
		INNER JOIN chat_members cm2 ON c.id = cm2.chat_id
		WHERE c.is_group = 0 
		  AND cm1.user_id = ? 
		  AND cm2.user_id = ?
	`
)

type SqliteRepository struct {
	Db *sqlx.DB
}

func NewSqliteRepository(dbPath string) (*SqliteRepository, error) {
	// dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
	// dsn := "file:" + dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	db, err := sqlx.Open("sqlite", dbPath)

	if err != nil {
		return nil, err
	}


	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(1 * time.Hour)

	return &SqliteRepository{Db: db}, nil
}

func (r SqliteRepository) InitializeSchema(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*5),
	)

	defer cancel()
	_, err := r.Db.ExecContext(queryCtx, SCHEMA_QUERY)

	return err
}

func (r SqliteRepository) Close() error {
	return r.Db.Close()
}

func (r SqliteRepository) GetChatById(
	ctx context.Context, chatId string,
) (Chat, error) {
	chat := Chat{}

	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := r.Db.GetContext(queryCtx, &chat, GET_CHAT_BY_ID_QUERY, chatId)
	return chat, err
}

func (r SqliteRepository) GetDirectChatByUsers(
	ctx context.Context, userIdA string, userIdB string,
) (Chat, error) {
	var chat Chat

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Second*2))
	defer cancel()

	err := r.Db.GetContext(
		queryCtx, &chat, GET_DIRECT_CHAT_BY_USERS_QUERY, userIdA, userIdB,
	)
	return chat, err
}

func (r SqliteRepository) InsertChat(
	ctx context.Context, chat Chat, memberIds []string,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*3),
	)
	defer cancel()

	tx, err := r.Db.BeginTxx(queryCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.NamedExecContext(queryCtx, INSERT_CHAT_QUERY, chat)
	if err != nil {
		return err
	}

	if len(memberIds) == 0 {
		return tx.Commit()
	}

	members := make([]map[string]interface{}, 0, len(memberIds))
	// not the nices way, copied from docs
	for _, memberId := range memberIds {
		members = append(
			members,
			map[string]interface{}{"chat_id": chat.Id, "user_id": memberId},
		)
	}

	_, err = tx.NamedExecContext(
		queryCtx,
		ADD_CHAT_MEMBER_QUERY,
		members,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r SqliteRepository) GetUserChats(
	ctx context.Context, userId string,
) ([]Chat, error) {
	chats := []Chat{}

	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	err := r.Db.SelectContext(queryCtx, &chats, GET_CHATS_BY_USER_QUERY, userId)
	return chats, err
}

func (r SqliteRepository) AddChatMember(
	ctx context.Context, chatId, userId string,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	_, err := r.Db.NamedExecContext(
		queryCtx,
		ADD_CHAT_MEMBER_QUERY,
		map[string]interface{}{
			"chat_id": chatId,
			"user_id": userId,
		},
	)
	return err
}

func (r SqliteRepository) RemoveChatMember(
	ctx context.Context, chatId, userId string,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	_, err := r.Db.NamedExecContext(
		queryCtx,
		REMOVE_CHAT_MEMBER_QUERY,
		map[string]interface{}{
			"chat_id": chatId,
			"user_id": userId,
		},
	)
	return err
}

func (r SqliteRepository) GetChatMembers(
	ctx context.Context, chatId string,
) ([]string, error) {
	ids := []string{}

	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	err := r.Db.SelectContext(
		queryCtx,
		&ids,
		GET_CHAT_MEMBERS_QUERY,
		chatId,
	)

	return ids, err
}

func (r SqliteRepository) InsertMessage(
	ctx context.Context, message *Message,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()
	_, err := r.Db.NamedExecContext(queryCtx, INSERT_MESSAGE_QUERY, message)
	return err
}

// InsertMessageWithAcks unifies message and acks insert under one transaction.
func (r SqliteRepository) InsertMessageWithAcks(
	ctx context.Context,
	message *Message,
	acks []MessageAck,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := r.Db.BeginTxx(queryCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.NamedExecContext(queryCtx, INSERT_MESSAGE_QUERY, message)
	if err != nil {
		return err
	}

	if len(acks) > 0 {
		_, err = tx.NamedExecContext(queryCtx, INSERT_MESSAGE_ACK_QUERY, acks)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r SqliteRepository) InsertMessageAcks(
	ctx context.Context, acks []MessageAck,
) error {
	if len(acks) == 0 {
		return nil
	}

	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*5), // larger timeout
	)
	defer cancel()

	tx, err := r.Db.BeginTxx(queryCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.NamedExecContext(
		queryCtx,
		INSERT_MESSAGE_ACK_QUERY,
		acks,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r SqliteRepository) GetUndeliveredMessages(
	ctx context.Context, userId string,
) ([]Message, error) {
	messages := []Message{}

	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	err := r.Db.SelectContext(
		queryCtx,
		&messages,
		GET_UNDELIVERED_MESSAGES_QUERY,
		userId,
	)

	return messages, err
}

func (r SqliteRepository) AcknowledgeAndCleanupMessage(
	ctx context.Context, messageId string, userId string, deliveredAt time.Time,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	tx, err := r.Db.BeginTxx(queryCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// mark this user's delivery ack status
	_, err = tx.ExecContext(
		queryCtx, SET_MESSAGE_DELIVERED_QUERY, deliveredAt, messageId, userId,
	)

	if err != nil {
		return err
	}

	// count how many users still haven't received this message
	var pendingCount int
	err = tx.GetContext(
		queryCtx, &pendingCount, COUNT_PENDING_ACKS_QUERY, messageId,
	)
	if err != nil {
		return err
	}

	// if no pending acks remain, and remove message
	if pendingCount == 0 {
		_, err = tx.ExecContext(queryCtx, DELETE_MESSAGE_QUERY, messageId)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// func (r SqliteRepository) SetMessageDelivered(
// 	ctx context.Context,
// 	messageId string,
// 	userId string,
// 	deliveredAt time.Time,
// ) error {
// 	queryCtx, cancel := context.WithTimeout(
// 		ctx, time.Duration(time.Second*2),
// 	)
// 	defer cancel()
//
// 	_, err := r.Db.ExecContext(
// 		queryCtx,
// 		SET_MESSAGE_DELIVERED_QUERY,
// 		deliveredAt,
// 		messageId,
// 		userId,
// 	)
//
// 	return err
// }

func (r SqliteRepository) SetMessageRead(
	ctx context.Context,
	messageId string,
	userId string,
	readAt time.Time,
) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	_, err := r.Db.ExecContext(
		queryCtx,
		SET_MESSAGE_READ_QUERY,
		readAt,
		messageId,
		userId,
	)

	return err
}

func (r SqliteRepository) GetMessageAcks(
	ctx context.Context,
	messageId string,
) ([]MessageAck, error) {
	acks := []MessageAck{}

	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()

	err := r.Db.SelectContext(
		queryCtx,
		&acks,
		GET_MESSAGE_ACKS_QUERY,
		messageId,
	)

	return acks, err
}
