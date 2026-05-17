package profiles

import (
	"context"
	"errors"
)

var ErrorUniqueConstraintViolated = errors.New(
	"profile repository: query failed due to unique constraint violation",
)

type Repository interface {
	InitializeSchema(ctx context.Context) error

	// fill in the generated UserId
	InsertProfile(ctx context.Context, p *Profile) error

	GetProfileByUserId(ctx context.Context, userID string) (Profile, error)

	Close() error
}
