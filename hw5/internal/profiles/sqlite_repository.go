package profiles

import (
	"context"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	SCHEMA_QUERY = `
	CREATE TABLE IF NOT EXISTS
			profiles(
					user_id TEXT PRIMARY KEY,
					username TEXT NOT NULL,
					password TEXT NOT NULL
			)
	`

	INSERT_QUERY = `
		INSERT INTO profiles (user_id, username, password) 
		VALUES (:user_id, :username, :password)
	`

	GET_BY_USER_ID_QUERY = `
		SELECT user_id, username, password FROM profiles 
		WHERE user_id = ?
	`
)

type SqliteRepository struct {
	Db *sqlx.DB
}

func NewSqliteRepository(dbPath string) (*SqliteRepository, error) {
	db, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return &SqliteRepository{}, err
	}
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

func (r SqliteRepository) InsertProfile(ctx context.Context, p Profile) error {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()
	_, err := r.Db.NamedExecContext(queryCtx, INSERT_QUERY, p)
	if err != nil && isUniqueConstraint(err) {
		return ErrorUniqueConstraintViolated;
	}

	return err
}

func (r SqliteRepository) GetProfileByUserId(
	ctx context.Context, userId string,
) (Profile, error) {
	queryCtx, cancel := context.WithTimeout(
		ctx, time.Duration(time.Second*2),
	)
	defer cancel()
	p := Profile{}
	err := r.Db.GetContext(queryCtx, &p, GET_BY_USER_ID_QUERY, userId)
	return p, err
}

func (r SqliteRepository) Close() error {
	return r.Db.Close()
}

func isUniqueConstraint(err error) bool {
    return err != nil &&
        strings.Contains(err.Error(), "UNIQUE constraint failed")
}
