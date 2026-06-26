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

	// True when invisible flag changed and the chat model
	// should reconnect / resubscribe to apply new presence mode.
	ReconnectRequired bool
}

type ProfileLoadedMsg struct {
	Profile *state.Profile
}

type ProfileSubModel struct {
	appContext *state.AppContext
	returnTo   SubModel
	isEditable bool

	userId string

	username    textinput.Model
	bio         textinput.Model
	status      string
	isInvisible bool

	loading    bool
	focusIndex int
}

func NewProfileSubModel(
	appContext *state.AppContext,
	userId string,
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

	m := &ProfileSubModel{
		appContext:  appContext,
		returnTo:    returnTo,
		isEditable:  isEditable,
		userId:      userId,
		username:    uname,
		bio:         bioInput,
		isInvisible: appContext.Session.IsInvisible(),
		loading:     true,
		focusIndex:  0,
	}

	if isEditable {
		m.username.Focus()
	}

	return m
}

func (m *ProfileSubModel) Id() SubModelId {
	return ScreenProfile
}

func (m *ProfileSubModel) Init() tea.Cmd {
	return m.loadProfile()
}

func (m *ProfileSubModel) loadProfile() tea.Cmd {
	return func() tea.Msg {
		profile, err := tui.ResolveProfileToSession(
			m.appContext,
			m.userId,
		)
		if err != nil {
			return NewErrorSubModel(tui.CleanGrpcError(err), m.returnTo)
		}

		return ProfileLoadedMsg{
			Profile: profile,
		}
	}
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
	case ProfileLoadedMsg:
		m.loading = false
		m.username.SetValue(msg.Profile.Username)
		m.bio.SetValue(msg.Profile.Bio)
		m.status = msg.Profile.Status
		return m, textinput.Blink

	case ProfileUpdateSuccessMsg:
		// update session cache
		if profile, ok := m.appContext.Session.GetProfile(m.userId); ok {
			profile.Username = msg.Username
			profile.Bio = msg.Bio
		}

		return m.returnTo, func() tea.Msg {
			return msg
		}

	case tea.KeyPressMsg:
		keyStr := msg.String()

		switch keyStr {
		case "esc":
			return m.returnTo, nil

		case "tab":
			m.nextFocus()
			return m, nil

		case "space":
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
	if m.loading {
		content := tui.DialogBoxStyle.Render("Loading profile...")
		return tea.NewView(
			lipgloss.Place(
				width,
				height,
				lipgloss.Center,
				lipgloss.Center,
				content,
			),
		)
	}

	title := "User Profile"
	if m.isEditable {
		title = "Edit Profile"
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#E5E5E5"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A0A0A0"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

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

	sections = append(
		sections,
		titleStyle.Render(title),
		"",
		labelStyle.Render("User ID"),
		valueStyle.Render(m.userId),
		"",
		labelStyle.Render("Status"),
		valueStyle.Render(m.status),
		"",
		labelStyle.Render("Username"),
	)

	if m.isEditable {
		sections = append(
			sections,
			makeInputStyle(m.isUsernameFocused()).Render(m.username.View()),
		)
	} else {
		sections = append(sections, valueStyle.Render(m.username.Value()))
	}

	sections = append(
		sections,
		"",
		labelStyle.Render("Biography"),
	)

	if m.isEditable {
		sections = append(
			sections,
			makeInputStyle(m.isBioFocused()).Render(m.bio.View()),
		)
	} else {
		bio := m.bio.Value()
		if bio == "" {
			bio = "No biography written yet."
		}
		sections = append(sections, valueStyle.Render(bio))
	}

	// Show visibility toggle only for current user profile.
	if m.isEditable && m.userId == m.appContext.Session.GetUserId() {
		checkbox := "[ ] Invisible Mode"
		if m.isInvisible {
			checkbox = "[x] Invisible Mode"
		}

		if m.isInvisibleToggleFocused() {
			checkbox = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FFFF")).
				Bold(true).
				Render("> " + checkbox)
		}

		sections = append(
			sections,
			"",
			labelStyle.Render("Privacy"),
			checkbox,
		)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		sections...,
	)

	dialog := tui.DialogBoxStyle.Render(content)

	centered := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)

	return tea.NewView(centered)
}

func (m *ProfileSubModel) submitProfileUpdate() tea.Cmd {
	uname := strings.TrimSpace(m.username.Value())
	bioText := strings.TrimSpace(m.bio.Value())

	oldInvisible := m.appContext.Session.IsInvisible()
	newInvisible := m.isInvisible
	reconnectRequired := oldInvisible != newInvisible

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			3*time.Second,
		)
		defer cancel()

		_, err := m.appContext.ProfileClient.UpdateUserProfile(
			ctx,
			&pb.UpdateUserProfileRequest{
				Username: uname,
				Bio:      bioText,
			},
		)
		if err != nil {
			return NewErrorSubModel(
				tui.CleanGrpcError(err),
				m,
			)
		}

		m.appContext.Session.SetInvisible(newInvisible)
		return ProfileUpdateSuccessMsg{
			Username:          uname,
			Bio:               bioText,
			ReconnectRequired: reconnectRequired,
		}
	}
}
