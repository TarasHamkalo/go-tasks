package tui

type ProgramStateId int

const (
	StateOnboarding ProgramStateId = iota // Handles Login/Register
	StateChatList                  = iota
)

type ProgramState struct {
	Id     ProgramStateId
	UserId string
}
