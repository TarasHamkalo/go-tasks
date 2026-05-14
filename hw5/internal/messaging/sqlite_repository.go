package messaging

import (
	// "context"
	// "strings"
	// "time"
	//
	// "github.com/jmoiron/sqlx"
	// _ "modernc.org/sqlite"
)

const (
	// not sure whether to add foreign keys at all, we can not bound
	// sender id still, so...
	// Just leaving constraints that are possible to verify (inside single db)
	SCHEMA_QUERY = `
	CREATE TABLE IF NOT EXISTS
			chats(
					id TEXT PRIMARY KEY,
					name TEXT NOT NULL,
					is_group INTEGER NOT NULL
			);

	CREATE TABLE IF NOT EXISTS chat_members (
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
				user_id TEXT NOT NULL, -- FK to profile service users

				delivered_at DATETIME 
				read_at DATETIME 

				PRIMARY KEY(message_id, user_id),
				FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE
		);
	`

	INSERT_QUERY = `
		INSERT INTO profiles (user_id, username, password) 
		VALUES (:user_id, :username, :password)
	`

	GET_MESSAGE_BY_ID_QUERY = `
		SELECT user_id, username, password FROM profiles 
		WHERE user_id = ?
	`
)

// type SqliteRepository struct {
// 	Db *sqlx.DB
// }
//
// func NewSqliteRepository(dbPath string) (*SqliteRepository, error) {
// 	db, err := sqlx.Open("sqlite", dbPath)
// 	if err != nil {
// 		return &SqliteRepository{}, err
// 	}
// 	return &SqliteRepository{Db: db}, nil
// }
//
// func (r SqliteRepository) InitializeSchema(ctx context.Context) error {
// 	queryCtx, cancel := context.WithTimeout(
// 		ctx, time.Duration(time.Second*5),
// 	)
//
// 	defer cancel()
// 	_, err := r.Db.ExecContext(queryCtx, SCHEMA_QUERY)
//
// 	return err
// }
//
// func (r SqliteRepository) InsertProfile(ctx context.Context, p Profile) error {
// 	queryCtx, cancel := context.WithTimeout(
// 		ctx, time.Duration(time.Second*2),
// 	)
// 	defer cancel()
// 	_, err := r.Db.NamedExecContext(queryCtx, INSERT_QUERY, p)
// 	if err != nil && isUniqueConstraint(err) {
// 		return ErrorUniqueConstraintViolated
// 	}
//
// 	return err
// }
//
// func (r SqliteRepository) GetProfileByUserId(
// 	ctx context.Context, userId string,
// ) (Profile, error) {
// 	queryCtx, cancel := context.WithTimeout(
// 		ctx, time.Duration(time.Second*2),
// 	)
// 	defer cancel()
// 	p := Profile{}
// 	err := r.Db.GetContext(queryCtx, &p, GET_BY_USER_ID_QUERY, userId)
// 	return p, err
// }
//
// func (r SqliteRepository) Close() error {
// 	return r.Db.Close()
// }
//
// // isUniqueConstraint verifies whether given SQL error is unique constraint
// // violation. Pretty hard to check with given API of modernc.org/sqlite
// func isUniqueConstraint(err error) bool {
// 	return err != nil &&
// 		strings.Contains(err.Error(), "UNIQUE constraint failed")
// }
