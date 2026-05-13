package profiles

import (
	"context"
	"errors"
)

var ErrorUniqueConstraintViolated = errors.New(
	"profile repository: query failed due to unique constrain violation",
)

type Repository interface {

	InitializeSchema(ctx context.Context) error

	InsertProfile(ctx context.Context, p Profile) error

	GetProfileByUserId(ctx context.Context, userID string) (Profile, error)

	Close() error
}
