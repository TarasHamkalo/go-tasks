package profiles

// Profile represents a user's account and personal details within the system.
type Profile struct {
	// UserId is a unique 9-digit identifier string
	UserId   string `db:"user_id"`
	Username string `db:"username"`
	Password []byte `db:"password"`
	Bio      string `db:"bio"`
	Status   string `db:"status"`
}
