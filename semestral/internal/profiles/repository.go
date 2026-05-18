package profiles

import (
	"context"
	"errors"
)

// ErrorUniqueConstraintViolated indicates a record write 
// conflict on a unique database field.
var ErrorUniqueConstraintViolated = errors.New(
	"profile repository: query failed due to unique constraint violation",
)

// Repository defines the persistence interface for managing user profile data.
type Repository interface {

	// InitializeSchema creates database tables if they do not already exist.
	InitializeSchema(ctx context.Context) error

	// InsertProfile Inserts a new profile and populates the generated UserId.
	InsertProfile(ctx context.Context, p *Profile) error

	// GetProfileByUserId fetches a single profile using its unique 9-digit identifier.
	GetProfileByUserId(ctx context.Context, userId string) (Profile, error)

	// UpdateStatus transitions a user's network presence (e.g., ONLINE, OFFLINE).
	UpdateStatus(ctx context.Context, userId string, status string) error

	// UpdateProfile updates the mutable personal fields of an existing user record.
	UpdateProfile(ctx context.Context, userId string, username string, bio string) error

	// CheckUsersExist filters a list of IDs and returns only those that exist in storage.
	CheckUsersExist(ctx context.Context, userIds []string) ([]string, error)

	// Close safely terminates database connections and frees allocated engine resources.
	Close() error
}
