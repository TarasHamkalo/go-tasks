package models

import (
	"fmt"
	"strings"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type FocusArea int

const (
	FocusProfile FocusArea = iota
	FocusChatsList
	FocusActiveChat
)

type ChatModel struct {
	appContext *state.AppContext
	
	focusArea  FocusArea
	isEngaged  bool // If true, keystrokes pass directly to the inner sub-model
	
	// Track whether the profile overlay dialog is showing
	showProfileDialog bool

	// Sub-sections (Mocked structural layouts)
	profileSection   ProfileSection
	chatsListSection ChatsListSection
	activeChatSection ActiveChatSection
}

func NewChatModel(appContext *state.AppContext) *ChatModel {
	return &ChatModel{
		appContext:        appContext,
		focusArea:         FocusChatsList, // Default focus on the chat list
		isEngaged:         false,
		profileSection:   ProfileSection{Username: "JohnDoe", UserID: "100000002"},
		chatsListSection: ChatsListSection{SelectedIndex: 0},
		activeChatSection: ActiveChatSection{},
	}
}

func (m *ChatModel) Id() SubModelId {
	return ScreenActiveChat
}

func (m *ChatModel) Init() tea.Cmd {
	return nil
}

func (m *ChatModel) ShortHelp() []Binding {
	if m.showProfileDialog {
		return []Binding{{Key: "Esc", Description: "Close Profile"}}
	}
	if m.isEngaged {
		return []Binding{{Key: "Esc", Description: "Unfocus Section"}}
	}
	return []Binding{
		{Key: "S-Tab", Description: "Cycle Sections Backward"},
		{Key: "Tab", Description: "Cycle Sections Forward"},
		{Key: "Enter", Description: "Interact"},
	}
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// handle profile dialog intercepts first
	if m.showProfileDialog {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "esc" {
				m.showProfileDialog = false
				m.isEngaged = false 
				return m, nil
			}
		}
		return m, nil // Block input from panels behind overlay
	}

	// 2. Handle Navigation Ring intercepts if NOT engaged
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
				// Special behavior for profile: pressing enter immediately triggers dialog state
				if m.focusArea == FocusProfile {
					m.showProfileDialog = true
				}
				return m, nil
			}
		}
		return m, nil
	} else {
		// break engaged section 
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			if msg.String() == "esc" {
				m.isEngaged = false
				return m, nil
			}
		}
	}

	// 4. Forward keystrokes down to whichever panel is active
	switch m.focusArea {
	case FocusProfile:
		m.profileSection, cmd = m.profileSection.Update(msg)
	case FocusChatsList:
		m.chatsListSection, cmd = m.chatsListSection.Update(msg)
	case FocusActiveChat:
		m.activeChatSection, cmd = m.activeChatSection.Update(msg)
	}

	return m, cmd
}

func (m *ChatModel) View() tea.View {
	return m.ContentView(500, 500)
}

func (m *ChatModel) ContentView(width, height int) tea.View {
	leftWidth := int(float64(width) * 0.30)
	rightWidth := width - leftWidth

	// profile vs Chat List split heights
	profileHeight := 5
	chatsHeight := height - profileHeight

	// track layout highlights dynamically using focus colors
	profileFocused := m.focusArea == FocusProfile
	chatsFocused := m.focusArea == FocusChatsList
	activeChatFocused := m.focusArea == FocusActiveChat

	// Render the text surfaces of individual sub panels
	profileView := m.profileSection.View(leftWidth, profileHeight, profileFocused, m.isEngaged)
	chatsListView := m.chatsListSection.View(leftWidth, chatsHeight, chatsFocused, m.isEngaged)
	activeChatView := m.activeChatSection.View(rightWidth, height, activeChatFocused, m.isEngaged)

	// Combine left panel vertical stack
	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		profileView,
		chatsListView,
	)

	// Stitch left panel row cleanly with right chat viewport pane
	mainLayout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		activeChatView,
	)

	// 5. If profile dialog overlay is active, paint it directly over center frame coords
	if m.showProfileDialog {
		dialogBox := tui.DialogBoxStyle.
			BorderForeground(lipgloss.Color("#A3A3A3")).
			Padding(1, 2).
			Render(fmt.Sprintf(
				"👤 PROFILE SETTINGS\n\nUsername: %s\nUser ID:  %s\n\nPress [Esc] to exit settings pane.",
				m.profileSection.Username, m.profileSection.UserID,
			))
		
			// TODO: do i want to overlay it or replace?
		mainLayout = lipgloss.Place(
			width, 
			height,
			lipgloss.Center, 
			lipgloss.Center, 
			dialogBox,
		)
	}

	return tea.NewView(mainLayout)
}

type ProfileSection struct {
	Username string
	UserID   string
}

func (p ProfileSection) Update(msg tea.Msg) (ProfileSection, tea.Cmd) { 
	return p, nil 
}

func (p ProfileSection) View(
	width, height int, focused, engaged bool,
) string {
	borderColor := "#3C3C3C" // Dim gray default
	if focused {
		borderColor = "#00FF00" // Green when selected in nav ring
	}

	style := lipgloss.NewStyle().
		Width(width - 2).   // Account for borders
		Height(height - 2). // Account for borders
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	content := fmt.Sprintf("👤 %s\nID: %s", p.Username, p.UserID)
	return style.Render(content)
}

type ChatsListSection struct {
	SelectedIndex int
}

func (cl ChatsListSection) Update(msg tea.Msg) (ChatsListSection, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if cl.SelectedIndex > 0 {
				cl.SelectedIndex--
			}
		case "down", "j":
			if cl.SelectedIndex < 2 { // Boundary limits for mock entries
				cl.SelectedIndex++
			}
		}
	}
	return cl, nil
}

func (cl ChatsListSection) View(width, height int, focused, engaged bool) string {
	borderColor := "#3C3C3C"
	if engaged && focused {
		borderColor = "#FF007F" // Bright pink color when engaged (scrolling chat list)
	} else if focused {
		borderColor = "#00FF00" // Selected via tab ring
	}

	style := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	// Populate mock items to demonstrate highlight navigation mechanics
	mockChats := []string{"Alice (Direct)", "Dev Group Chat", "Bob (Direct)"}
	var renderedList []string
	
	renderedList = append(renderedList, "💬 CHATS LIST\n")
	for i, name := range mockChats {
		if i == cl.SelectedIndex && focused {
			// Visual cursor marker logic
			renderedList = append(renderedList, fmt.Sprintf("👉 \033[1m%s\033[0m", name))
		} else {
			renderedList = append(renderedList, fmt.Sprintf("   %s", name))
		}
	}

	return style.Render(strings.Join(renderedList, "\n"))
}

type ActiveChatSection struct{}

func (ac ActiveChatSection) Update(msg tea.Msg) (ActiveChatSection, tea.Cmd) { return ac, nil }

func (ac ActiveChatSection) View(width, height int, focused, engaged bool) string {
	borderColor := "#3C3C3C"
	if engaged && focused {
		borderColor = "#FF007F" // Bright pink when typing message input
	} else if focused {
		borderColor = "#00FF00"
	}

	outerStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(height - 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor))

	// Divide internal viewport heights for structural messaging vs inputs blocks
	messagesBoxHeight := height - 5
	
	msgStyle := lipgloss.NewStyle().
		Height(messagesBoxHeight).
		Padding(1, 2)
		
	inputStyle := lipgloss.NewStyle().
		Width(width - 6).
		Border(lipgloss.NormalBorder(), true, false, false, false). // Simple line partition divider 
		BorderForeground(lipgloss.Color("#2B2B2B")).
		Padding(1, 1)

	mockMessages := "System: Welcome to Go Messenger!\nAlice: Hey! Did you configure the local Sqlite stores yet?\nYou: Yeah, append-only messaging ledger is operating smoothly."
	renderedMsgs := msgStyle.Render(mockMessages)
	
	inputPrompt := "Type a message... (Press Enter)"
	if engaged && focused {
		inputPrompt = "Writing message... _"
	}
	renderedInput := inputStyle.Render(inputPrompt)

	body := lipgloss.JoinVertical(lipgloss.Left, renderedMsgs, renderedInput)
	return outerStyle.Render(body)
}
