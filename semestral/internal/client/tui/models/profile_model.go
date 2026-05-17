package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type ProfileUpdateSuccessMsg struct {
	Username string
	Bio      string
}

type ProfileSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	isEditable bool

	username    textinput.Model
	bio         textinput.Model
	isInvisible bool

	// Focus order:
	// Editable:     0 (Username) -> 1 (Bio) -> 2 (Invisible Toggle)
	// Non-Editable: 0 (Invisible Toggle)
	focusIndex int
}

func NewProfileSubModel(
	appContext *state.AppContext,
	isEditable bool,
	returnTo SubModel,
) *ProfileSubModel {
	uname := textinput.New()
	uname.CharLimit = 32
	uname.Placeholder = "Username"
	uname.SetWidth(30)
	uname.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) < 3 || len(trimmed) > 32 {
			return fmt.Errorf("username must be 3-32 characters")
		}
		return nil
	}

	bioInput := textinput.New()
	bioInput.CharLimit = 100
	bioInput.Placeholder = "Bio (Optional)"
	bioInput.SetWidth(30)

	userId := appContext.Session.GetUserId()
	if profile, ok := appContext.Session.GetProfile(userId); ok {
		uname.SetValue(profile.Username)
		bioInput.SetValue(profile.Bio)
	}

	m := &ProfileSubModel{
		appContext:  appContext,
		returnTo:    returnTo,
		isEditable:  isEditable,
		username:    uname,
		bio:         bioInput,
		isInvisible: false,
		focusIndex:  0,
	}

	if m.isEditable {
		m.username.Focus()
	}

	return m
}

func (m *ProfileSubModel) Id() SubModelId {
	return ScreenProfile
}

func (m *ProfileSubModel) Init() tea.Cmd {
	if m.isEditable {
		return textinput.Blink
	}
	return nil
}

func (m *ProfileSubModel) isUsernameFocused() bool {
	return m.isEditable && m.focusIndex == 0
}

func (m *ProfileSubModel) isBioFocused() bool {
	return m.isEditable && m.focusIndex == 1
}

func (m *ProfileSubModel) isInvisibleToggleFocused() bool {
	if m.isEditable {
		return m.focusIndex == 2
	}
	return m.focusIndex == 0
}

func (m *ProfileSubModel) nextFocus() {
	maxFields := 1
	if m.isEditable {
		maxFields = 3
	}
	m.focusIndex = (m.focusIndex + 1) % maxFields

	if m.isUsernameFocused() {
		m.username.Focus()
		m.bio.Blur()
	} else if m.isBioFocused() {
		m.username.Blur()
		m.bio.Focus()
	} else {
		m.username.Blur()
		m.bio.Blur()
	}
}

func (m *ProfileSubModel) ShortHelp() []tui.Binding {
	bindings := []tui.Binding{}

	if m.isEditable {
		if m.isUsernameFocused() || m.isBioFocused() {
			bindings = append(bindings, tui.Binding{Key: "Type", Description: "Edit text"})
		}
		bindings = append(bindings, tui.Binding{Key: "Tab", Description: "Next field"})
		if m.isInvisibleToggleFocused() {
			bindings = append(bindings, tui.Binding{Key: "Space", Description: "Toggle Status"})
		}
		bindings = append(bindings, tui.Binding{Key: "Ctrl+s", Description: "Save changes"})
	} else {
		bindings = append(bindings, tui.Binding{Key: "Tab", Description: "Focus Toggle"})
		if m.isInvisibleToggleFocused() {
			bindings = append(bindings, tui.Binding{Key: "Space", Description: "Toggle Status"})
		}
	}

	bindings = append(bindings, tui.Binding{Key: "Esc", Description: "Back"})
	return bindings
}

func (m *ProfileSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ProfileUpdateSuccessMsg:
		userId := m.appContext.Session.GetUserId()
		if profile, ok := m.appContext.Session.GetProfile(userId); ok {
			profile.Username = msg.Username
			profile.Bio = msg.Bio
		}
		return m.returnTo, nil

	case tea.KeyPressMsg:
		keyStr := msg.String()

		switch keyStr {
		case "esc":
			return m.returnTo, nil

		case "tab":
			m.nextFocus()
			return m, nil

		case " ":
			if m.isInvisibleToggleFocused() {
				m.isInvisible = !m.isInvisible
				return m, nil
			}

		case "ctrl+s":
			if m.isEditable {
				if err := m.username.Validate(m.username.Value()); err != nil {
					errModel := NewErrorSubModel(err, m)
					return errModel, errModel.Init()
				}
				return m, m.submitProfileUpdate()
			}
		}

		if m.isUsernameFocused() {
			m.username, cmd = m.username.Update(msg)
			return m, cmd
		}
		if m.isBioFocused() {
			m.bio, cmd = m.bio.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *ProfileSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *ProfileSubModel) ContentView(width, height int) tea.View {
	title := "User Profile (View Only)"
	if m.isEditable {
		title = "Edit Profile Settings"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5")).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0")).
		MarginTop(1)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1)

	toggleStyle := lipgloss.NewStyle().
		Padding(0, 1)

	makeInputStyle := func(focused bool) lipgloss.Style {
		color := "#3C3C3C"
		if focused {
			color = "#FF007F"
		}
		return lipgloss.NewStyle().
			Width(34).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(color))
	}

	var sections []string
	sections = append(sections, titleStyle.Render(title), "")

	// 1. Account User ID Field (Always Read Only)
	sections = append(
		sections,
		labelStyle.Render("Account User ID:"),
		valueStyle.Render(fmt.Sprintf("ID: %s", m.appContext.Session.GetUserId())),
	)

	// 2. Username Field View Calculation
	sections = append(sections, labelStyle.Render("Username:"))
	if m.isEditable {
		sections = append(sections, makeInputStyle(m.isUsernameFocused()).Render(m.username.View()))
	} else {
		sections = append(sections, valueStyle.Render(m.username.Value()))
	}

	// 3. Biography Field View Calculation
	sections = append(sections, labelStyle.Render("Biography Status:"))
	if m.isEditable {
		sections = append(sections, makeInputStyle(m.isBioFocused()).Render(m.bio.View()))
	} else {
		bioVal := m.bio.Value()
		if bioVal == "" {
			bioVal = "No biography written yet."
		}
		sections = append(sections, valueStyle.Render(bioVal))
	}

	if m.isEditable {
		// 4. Invisible Status Checkbox Component Block
		sections = append(sections, labelStyle.Render("Privacy Options:"))
		checkboxSymbol := "[ ]"
		if m.isInvisible {
			checkboxSymbol = "[x]"
		}

		toggleContent := fmt.Sprintf("%s Go Invisible Mode", checkboxSymbol)
		if m.isInvisibleToggleFocused() {
			toggleContent = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FFFF")).
				Bold(true).
				Render(fmt.Sprintf("> %s Go Invisible Mode", checkboxSymbol))
		} else {
			toggleContent = toggleStyle.Foreground(lipgloss.Color("#888888")).Render(toggleContent)
		}
		sections = append(sections, toggleContent)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	dialog := tui.DialogBoxStyle.BorderForeground(lipgloss.Color("#3C3C3C")).Render(content)

	return tea.NewView(lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog))
}

func (m *ProfileSubModel) submitProfileUpdate() tea.Cmd {
	uname := strings.TrimSpace(m.username.Value())
	bioText := strings.TrimSpace(m.bio.Value())
	m.appContext.RootLogger.Info("i am here")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.appContext.Ctx, 3*time.Second)
		defer cancel()

		_, err := m.appContext.ProfileClient.UpdateUserProfile(
			ctx,
			&pb.UpdateUserProfileRequest{
				Username: uname,
				Bio:      bioText,
			})

		if err != nil {
			return NewErrorSubModel(tui.CleanGrpcError(err), m)
		}

		return ProfileUpdateSuccessMsg{
			Username: uname,
			Bio:      bioText,
		}
	}
}
