package profiles

import (
	"context"
	"errors"
)

// ErrorUniqueConstraintViolated indicates that a write operation
// failed because a unique field already exists.
var ErrorUniqueConstraintViolated = errors.New(
	"profile repository: query failed due to unique constraint violation",
)

// Repository defines persistence operations for user profiles.
type Repository interface {
	// Creates database tables if they do not already exist.
	InitializeSchema(ctx context.Context) error

	// Inserts a new profile and populates the generated UserId.
	InsertProfile(ctx context.Context, p *Profile) error

	// Returns a profile by its unique user ID.
	GetProfileByUserId(ctx context.Context, userID string) (Profile, error)

	// Updates the user's online/offline status.
	UpdateStatus(ctx context.Context, userID string, status string) error

	// Updates editable profile fields.
	UpdateProfile(
		ctx context.Context,
		userId string,
		username string,
		bio string,
	) error

	// Releases repository resources.
	Close() error
}
