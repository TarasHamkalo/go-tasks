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
	// SCHEMA_QUERY create schema with user_id autoincrement sequence shifted to have 9-digits
	SCHEMA_QUERY = `
	CREATE TABLE IF NOT EXISTS
			profiles(
					user_id INTEGER PRIMARY KEY AUTOINCREMENT,
					username TEXT NOT NULL UNIQUE,
					password BLOB NOT NULL,
					bio TEXT NOT NULL DEFAULT '',
					status TEXT NOT NULL DEFAULT 'offline'
			);

	-- shift id range (for all to have 9-digit IDs)
	INSERT OR IGNORE INTO sqlite_sequence (name, seq) VALUES ('profiles', 99999999);
	`

	INSERT_QUERY = `
		INSERT INTO profiles (username, password) 
		VALUES (:username, :password)
	`

	// GET_BY_USER_ID_QUERY gets user profile and casts id to string
	GET_BY_USER_ID_QUERY = `
		SELECT CAST(user_id AS TEXT) AS user_id, username, password, bio, status 
		FROM profiles 
		WHERE user_id = ?
	`

	UPDATE_STATUS_QUERY = `
		UPDATE profiles SET status = ? WHERE user_id = ?
	`

	UPDATE_PROFILE_QUERY = `
		UPDATE profiles SET username = ?, bio = ? WHERE user_id = ?
	`
	// GET_EXISTING_USERS filters out ids of users that exist in db
	GET_EXISTING_USERS = `
		SELECT CAST(user_id AS TEXT) 
		FROM profiles 
		WHERE user_id IN (?)
	`
)

// SqliteRepository implements the profiles.Repository interface using a SQLite database
type SqliteRepository struct {
	Db *sqlx.DB
}

// NewSqliteRepository opens a SQLite database and returns
// a repository backed by it.
func NewSqliteRepository(dbPath string) (*SqliteRepository, error) {
	// SQLite DSN configuration:
	// - foreign_keys=1     enables foreign key constraint enforcement
	// - journal_mode=WAL   allows concurrent reads during writes
	// - busy_timeout=5000  waits up to 5 seconds if the database is locked though 
	//			- timeouts over ctx are mostly shorter though 
	// - synchronous=NORMAL balances durability and performance
	dsn := "file:" + dbPath +
		"?_pragma=foreign_keys=1" +
		"&_pragma=journal_mode=WAL" +
		"&_pragma=busy_timeout=5000" +
		"&_pragma=synchronous=NORMAL"
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Configure connection pool limits
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
			// this will now trigger if the username is already taken
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

func (r SqliteRepository) UpdateStatus(
	ctx context.Context, userId string, status string,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := r.Db.ExecContext(queryCtx, UPDATE_STATUS_QUERY, status, userId)
	return err
}

func (r SqliteRepository) UpdateProfile(
	ctx context.Context, userId string, username string, bio string,
) error {
	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	_, err := r.Db.ExecContext(
		queryCtx, UPDATE_PROFILE_QUERY, username, bio, userId,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return ErrorUniqueConstraintViolated
		}
		return err
	}
	return nil
}

func (r SqliteRepository) CheckUsersExist(
	ctx context.Context, userIds []string,
) ([]string, error) {
	if len(userIds) == 0 {
		return []string{}, nil
	}

	queryCtx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()

	// sqlx.In expands the slice into the correct number of bindvars (?, ?, ?)
	query, args, err := sqlx.In(GET_EXISTING_USERS, userIds)
	if err != nil {
		return nil, err
	}

	query = r.Db.Rebind(query)

	var existingIds []string
	err = r.Db.SelectContext(queryCtx, &existingIds, query, args...)
	if err != nil {
		return nil, err
	}

	return existingIds, nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
