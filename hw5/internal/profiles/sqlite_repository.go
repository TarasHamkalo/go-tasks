package profiles

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	SCHEMA_QUERY = `
	CREATE TABLE IF NOT EXISTS
			profiles(
					user_id INTEGER PRIMARY KEY AUTOINCREMENT,
					username TEXT NOT NULL UNIQUE,
					password BLOB NOT NULL
			);

	-- shift id range (for all to have 9-digit IDs)
	INSERT OR IGNORE INTO sqlite_sequence (name, seq) VALUES ('profiles', 99999999);
	`

	INSERT_QUERY = `
		INSERT INTO profiles (username, password) 
		VALUES (:username, :password)
	`

	GET_BY_USER_ID_QUERY = `
    SELECT CAST(user_id AS TEXT) AS user_id, username, password 
    FROM profiles 
    WHERE user_id = ?
	`
)

type SqliteRepository struct {
	Db *sqlx.DB
}

func NewSqliteRepository(dbPath string) (*SqliteRepository, error) {
	dsn := "file:" + dbPath + "?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(1 * time.Hour)

	return &SqliteRepository{Db: db}, nil
}

func (r SqliteRepository) InitializeSchema(ctx context.Context) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Second*5))
	defer cancel()

	_, err := r.Db.ExecContext(queryCtx, SCHEMA_QUERY)
	return err
}

func (r SqliteRepository) InsertProfile(ctx context.Context, p *Profile) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	res, err := r.Db.NamedExecContext(queryCtx, INSERT_QUERY, p)
	if err != nil {
		if isUniqueConstraint(err) {
			// This will now trigger if the username is already taken
			return ErrorUniqueConstraintViolated
		}
		return err
	}

	lastInsertId, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to retrieve generated user ID: %w", err)
	}

	// format int to string
	p.UserId = strconv.FormatInt(lastInsertId, 10)

	return nil
}

func (r SqliteRepository) GetProfileByUserId(
	ctx context.Context, userId string,
) (Profile, error) {
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(time.Second*2))
	defer cancel()

	p := Profile{}
	err := r.Db.GetContext(queryCtx, &p, GET_BY_USER_ID_QUERY, userId)
	return p, err
}

func (r SqliteRepository) Close() error {
	return r.Db.Close()
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
