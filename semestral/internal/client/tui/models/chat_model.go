package models

import (
	"gomessenger/internal/client/state"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// --- Messages for triggering dialogs ---
type OpenProfileSettingsMsg struct{}
type OpenCreateChatMsg struct{ IsGroup bool }
type OpenMembersMsg struct{}

type OpenInviteMsg struct{}
type CloseDialogMsg struct{} // Used to return to the chat view

type FocusArea int

const (
	FocusProfile FocusArea = iota
	FocusChatsList
	FocusActiveChat
)

type ChatModel struct {
	appContext *state.AppContext

	focusArea FocusArea
	isEngaged bool

	activeDialog SubModel 

	profileSection    ProfileSection
	chatsListSection  *ChatsListSection
	activeChatSection *ActiveChatSection
}

func NewChatModel(appContext *state.AppContext) *ChatModel {
	return &ChatModel{
		appContext:        appContext,
		focusArea:         FocusChatsList,
		isEngaged:         false,
		profileSection:    ProfileSection{Username: "JohnDoe", UserID: "100000002"},
		chatsListSection:  NewChatsListSection(),
		activeChatSection: NewActiveChatSection(),
	}
}

func (m *ChatModel) Id() SubModelId { return ScreenActiveChat }
func (m *ChatModel) Init() tea.Cmd  { return nil }

func (m *ChatModel) ShortHelp() []Binding {
	if m.activeDialog != nil {
		return m.activeDialog.ShortHelp()
	}
	if m.isEngaged {
		return []Binding{{Key: "Esc", Description: "Unfocus Section"}}
	}
	return []Binding{
		{Key: "Tab", Description: "Cycle Sections"},
		{Key: "Enter", Description: "Interact"},
		{Key: "d/g", Description: "New Direct/Group (in Chats)"},
		{Key: "m/i", Description: "Members/Invite (in Chat)"},
	}
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if m.activeDialog != nil {
		switch msg.(type) {
		case CloseDialogMsg:
			m.activeDialog = nil
			m.isEngaged = false 
			return m, nil
		}

		var nextDialog tea.Model
		nextDialog, cmd = m.activeDialog.Update(msg)
		m.activeDialog = nextDialog.(SubModel)
		return m, cmd
	}

	// catch messages sent by the sections
	switch msg.(type) {
	case OpenProfileSettingsMsg:
		m.activeDialog = NewProfileSettingsDialog(m.appContext, m.profileSection.Username)
		return m, nil
	case OpenCreateChatMsg:
		// m.activeDialog = NewCreateChatDialog(...) // Implement later
		return m, nil
	case OpenMembersMsg, OpenInviteMsg:
		// m.activeDialog = NewMembersDialog(...) // Implement later
		return m, nil
	}

	// navigation between sections 
	if !m.isEngaged {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "shift+tab":
				m.focusArea = (m.focusArea + 2) % 3
				return m, nil
			case "tab":
				m.focusArea = (m.focusArea + 1) % 3
				return m, nil
			case "enter":
				m.isEngaged = true
				if m.focusArea == FocusProfile {
					// open dialog 
					return m, func() tea.Msg { return OpenProfileSettingsMsg{} }
				}
				return m, nil
			}
		}
	} else {
		// BREAK ENGAGEMENT
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "esc" {
				m.isEngaged = false
				return m, nil
			}
		}
	}

	// forward to sections
	switch m.focusArea {
	case FocusProfile:
		m.profileSection, cmd = m.profileSection.Update(msg)
	case FocusChatsList:
		m.chatsListSection, cmd = m.chatsListSection.Update(msg)
	case FocusActiveChat:
		m.activeChatSection, cmd = m.activeChatSection.Update(msg, m.isEngaged)
	}

	return m, cmd
}

func (m *ChatModel) View() tea.View { return m.ContentView(80, 24) }

func (m *ChatModel) ContentView(width, height int) tea.View {
	// ABANDON OVERLAY: If a dialog is active, ONLY render the dialog!
	if m.activeDialog != nil {
		return m.activeDialog.ContentView(width, height)
	}

	leftWidth := int(float64(width) * 0.30)
	rightWidth := width - leftWidth
	profileHeight := 5
	chatsHeight := height - profileHeight

	profileFocused := m.focusArea == FocusProfile
	chatsFocused := m.focusArea == FocusChatsList
	activeChatFocused := m.focusArea == FocusActiveChat

	profileView := m.profileSection.View(leftWidth, profileHeight, profileFocused, m.isEngaged)
	chatsListView := m.chatsListSection.View(leftWidth, chatsHeight, chatsFocused, m.isEngaged)
	activeChatView := m.activeChatSection.View(rightWidth, height, activeChatFocused, m.isEngaged)

	leftPanel := lipgloss.JoinVertical(lipgloss.Left, profileView, chatsListView)
	mainLayout := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, activeChatView)

	return tea.NewView(mainLayout)
}
