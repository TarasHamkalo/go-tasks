package state

import (
	"maps"
	"sync"
	"sync/atomic"
)

// Session stores user session data, all access to object has to synchronized
type Session struct {
	// sessionVersion object version, to track changes in UI and no rerender
	// on each frame... used only for chat list :)
	sessionVersion atomic.Int64

	userId string

	// Data stores
	// profiles is map UserId -> Profile
	profiles    map[string]*Profile 
	// chats is map ChatId -> Chat Interface
	chats       map[string]Chat     
	// chatMembers  -> []UserID
	chatMembers map[string][]string 

	// unread tracking (UI state), ChatId -> Count of new messages
	unreadCounts map[string]int 

	// isInvisible tracks whether user requested invisible session.
	// It is status info should not be propagated. Messages should be still
	// acked 
	//
	// NOTE: this one could use atomic bool
	isInvisible bool

	mu sync.RWMutex
}

// NewSession construct new session
func NewSession() *Session {
	return &Session{
		profiles:     make(map[string]*Profile),
		chats:        make(map[string]Chat),
		chatMembers:  make(map[string][]string),
		unreadCounts: make(map[string]int),
	}
}

func (s *Session) touch() {
	s.sessionVersion.Add(1)
}


func (s *Session) Version() int64 {
	return s.sessionVersion.Load()
}

func (s *Session) IsInvisible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isInvisible
}

func (s *Session) SetInvisible(invisible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isInvisible == invisible {
		return
	}

	s.isInvisible = invisible
	s.touch()
}

func (s *Session) SetUserId(userId string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.userId = userId
	s.touch()
}

func (s *Session) GetUserId() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.userId
}

func (s *Session) IncrementUnread(chatId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unreadCounts[chatId]++
	s.touch()
}

func (s *Session) GetUnread(chatId string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unreadCounts[chatId]
}

func (s *Session) ResetUnread(chatId string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.unreadCounts[chatId] = 0
	s.touch()
}

func (s *Session) SetProfile(profile *Profile) {
	if profile == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.profiles[profile.Id] = profile
	s.touch()
}

func (s *Session) GetCurrentUserProfile() (*Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.profiles[s.userId]
	return profile, ok
}

func (s *Session) GetProfile(userId string) (*Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, ok := s.profiles[userId]
	return profile, ok
}

func (s *Session) RemoveChat(chatId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.chats, chatId)
	delete(s.chatMembers, chatId)
	delete(s.unreadCounts, chatId)
	s.touch()
}

func (s *Session) InsertChat(chat Chat) {
	if chat == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.chats[chat.Id()] = chat
	s.touch()
}

func (s *Session) GetChat(chatId string) (Chat, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chat, ok := s.chats[chatId]
	return chat, ok
}

func (s *Session) GetChatsSnapshot() map[string]Chat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]Chat, len(s.chats))
	maps.Copy(out, s.chats)
	return out
}

func (s *Session) SetChatMembers(chatId string, members []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// copy slice to prevent outside mutation
	copied := append([]string(nil), members...)
	s.chatMembers[chatId] = copied
	s.touch()
}

func (s *Session) GetChatMembers(chatId string) ([]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	members, ok := s.chatMembers[chatId]
	if !ok {
		return nil, false
	}

	return append([]string(nil), members...), true
}
