package tui

type ProgramStateId int

const (
	StateOnboarding ProgramStateId = iota
	StateChatList
)

// ProgramState acts as our application context/config
type ProgramState struct {
	Id              ProgramStateId
	UserId          string
	ProfilesApiUrl  string
	MessagingApiUrl string
}
