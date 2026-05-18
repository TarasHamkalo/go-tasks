package state

// Config stores readonly client configuration, constructed at startup
type Config struct {
	ProfilesApiAddr  string
	MessagingApiAddr string

	TokenIssuer string

	LocalDataDir string
}
