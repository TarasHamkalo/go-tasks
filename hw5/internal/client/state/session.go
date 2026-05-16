package state

type Session struct {
	UserId string

	// Data stores
	Profiles    map[string]*Profile // UserId -> Profile
	Chats       map[string]Chat     // ChatId -> Polymorphic Chat Interface
	ChatMembers map[string][]string // ChatId -> []UserID

	// Unread tracking (UI state)
	UnreadCounts map[string]int // ChatId -> Count of new messages
}

func NewSessionState() *Session {
	return &Session{
		Profiles:     make(map[string]*Profile),
		Chats:        make(map[string]Chat),
		ChatMembers:  make(map[string][]string),
		UnreadCounts: make(map[string]int),
	}
}
