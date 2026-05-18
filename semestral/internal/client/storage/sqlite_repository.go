package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	SCHEMA_QUERY = `
	CREATE TABLE IF NOT EXISTS messages(
		id TEXT PRIMARY KEY,
		chat_id TEXT NOT NULL,
		sender_id TEXT NOT NULL,
		content BLOB NOT NULL,
		sent_at DATETIME NOT NULL,
		is_pending INTEGER NOT NULL DEFAULT 0,
		is_read INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_messages_chat_sent ON messages(chat_id, sent_at DESC);
	`

	INSERT_QUERY = `
		INSERT OR IGNORE INTO messages (id, chat_id, sender_id, content, sent_at, is_pending, is_read) 
		VALUES (:id, :chat_id, :sender_id, :content, :sent_at, :is_pending, :is_read)
	`

	GET_MESSAGES_QUERY = `
		SELECT id, chat_id, sender_id, content, sent_at, is_pending, is_read 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY sent_at DESC 
		LIMIT ? OFFSET ?
	`

	MARK_DELIVERED_QUERY = `
		UPDATE messages 
		SET id = ?, is_pending = 0 
		WHERE id = ?
	`

	SET_IS_READ_QUERY = `
		UPDATE messages 
		SET is_read = 1 
		WHERE id IN (?)
	`
)

// SqliteRepository implements the storage.Repository interface using a 
// SQLite database
type SqliteRepository struct {
	Db *sqlx.DB
}

// NewSqliteRepository opens a SQLite database and returns
// a repository backed by it.
//
// NOTE: dataDir is created
func NewSqliteRepository(
	dataDir string, userId string,
) (*SqliteRepository, error) {
	err := os.MkdirAll(dataDir, 0750)
	if err != nil {
		return nil, fmt.Errorf("could not create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, fmt.Sprintf("user-%s.db", userId))
	dsn :=
		"file:" + dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"

	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	return &SqliteRepository{Db: db}, nil
}

func (r SqliteRepository) InitializeSchema(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	_, err := r.Db.ExecContext(queryCtx, SCHEMA_QUERY)
	return err
}

func (r SqliteRepository) InsertMessage(ctx context.Context, m *Message) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := r.Db.NamedExecContext(queryCtx, INSERT_QUERY, m)
	return err
}

func (r SqliteRepository) GetMessagesByChatId(
	ctx context.Context, chatId string, limit, offset int,
) ([]Message, error) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	var messages []Message
	err := r.Db.SelectContext(
		queryCtx, &messages, GET_MESSAGES_QUERY, chatId, limit, offset,
	)
	return messages, err
}

func (r SqliteRepository) MarkMessageDelivered(
	ctx context.Context, localId string, serverId string,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := r.Db.ExecContext(queryCtx, MARK_DELIVERED_QUERY, serverId, localId)
	return err
}

func (r *SqliteRepository) MarkMessagesRead(
	ctx context.Context, ids []string,
) error {
	if len(ids) == 0 {
		return nil
	}
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	query, args, err := sqlx.In(SET_IS_READ_QUERY, ids)
	if err != nil {
		return err
	}
	query = r.Db.Rebind(query)

	_, err = r.Db.ExecContext(queryCtx, query, args...)
	return err
}

func (r SqliteRepository) Close() error {
	return r.Db.Close()
}
