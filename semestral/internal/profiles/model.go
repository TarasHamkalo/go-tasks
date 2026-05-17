package profiles

type Profile struct {
	UserId   string `db:"user_id"`
	Username string `db:"username"`
	Password []byte `db:"password"`
	Bio      string `db:"bio"`
	Status   string `db:"status"`
}
