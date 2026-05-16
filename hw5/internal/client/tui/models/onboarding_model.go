package models

import (
	"errors"
	"fmt"
	pb "gomessenger/generated"
	"gomessenger/internal/client/tui/app"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"
)

type AuthSucceededMsg struct {
	UserID       string
	AccessToken  string
	RefreshToken string
}

type AuthSubState int

const (
	AuthPromptChoice AuthSubState = iota
	AuthLoginForm
	AuthRegisterForm
	AuthSubmitting
)

type OnboardingSubModel struct {
	profileClient pb.ProfileServiceClient

	subState AuthSubState

	username textinput.Model
	password textinput.Model

	logger *zap.Logger
	err    error
}

func NewOnboardingModel(appContext *app.AppContext) OnboardingSubModel {
	username := textinput.New()
	username.CharLimit = 32
	username.Placeholder = "Username"
	username.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) < 3 || len(trimmed) > 32 {
			return errors.New("username has to have between 3 to 32 non white characters")
		}
		return nil
	}
	username.SetWidth(30)

	password := textinput.New()
	password.CharLimit = 72
	password.Placeholder = "Password"
	password.EchoMode = textinput.EchoPassword
	password.SetWidth(30)
	password.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if strings.Contains(s, " ") {
			return errors.New("password should not contain whitespaces")
		}
		if len(trimmed) < 8 {
			return errors.New("min password length: 8")
		}
		return nil
	}

	return OnboardingSubModel{
		subState: AuthPromptChoice,
		username: username,
		password: password,
		logger:   appContext.RootLogger.With(zap.String("mvc", "onboarding")),
	}
}

func (m OnboardingSubModel) Id() SubModelId {
	return ScreenOnboarding
}

func (m OnboardingSubModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m OnboardingSubModel) ShortHelp() []Binding {
	switch m.subState {
	case AuthPromptChoice:
		return []Binding{
			{Key: "1", Description: "Login"},
			{Key: "2", Description: "Register"},
		}
	case AuthLoginForm, AuthRegisterForm:
		return []Binding{
			{Key: "Tab", Description: "Switch field"},
			{Key: "Enter", Description: "Submit"},
			{Key: "Esc", Description: "Back"},
		}
	default:
		return []Binding{}
	}
}

func (m OnboardingSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))
	var cmd tea.Cmd

	switch m.subState {
	case AuthPromptChoice:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "1":
				m.subState = AuthLoginForm
				m.username.Focus()
				return m, textinput.Blink
			case "2":
				m.subState = AuthRegisterForm
				m.username.Focus()
				return m, textinput.Blink
			}
		}

	case AuthLoginForm, AuthRegisterForm:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "tab":
				if m.username.Focused() {
					m.username.Blur()
					m.password.Focus()
				} else {
					m.password.Blur()
					m.username.Focus()
				}
				return m, nil
			case "enter":
				m.subState = AuthSubmitting
				return m, m.submit()
			case "esc":
				m.subState = AuthPromptChoice
				m.err = nil
				return m, nil
			}
		}

		if m.username.Focused() {
			m.username, cmd = m.username.Update(msg)
		} else {
			m.password, cmd = m.password.Update(msg)
		}
		return m, cmd

	case AuthSubmitting:
		switch msg := msg.(type) {
		case error:
			m.err = msg
			m.subState = AuthLoginForm
			return m, nil
		}
	}

	return m, nil
}

func (m OnboardingSubModel) View() tea.View {
	return tea.NewView("")
}

func (m OnboardingSubModel) ContentView(width, height int) tea.View {
	var content string

	switch m.subState {
	case AuthPromptChoice:
		content = lipgloss.JoinVertical(
			lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render("Go Messenger"),
			"",
			"1. Login   ",
			"2. Register",
		)
	case AuthLoginForm:
		content = m.formView("Login")
	case AuthRegisterForm:
		content = m.formView("Register")
	case AuthSubmitting:
		content = lipgloss.NewStyle().Bold(true).Render("Authenticating...")
	}

	// Render the box and place it directly in the center of the available space
	box := DialogBoxStyle.Render(content)
	centered := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)

	return tea.NewView(centered)
}

func (m OnboardingSubModel) formView(title string) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		"",
		"Username:",
		m.username.View(),
		"",
		"Password:",
		m.password.View(),
	)

	if m.err != nil {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("Error: %v", m.err))

		body = lipgloss.JoinVertical(lipgloss.Left, body, "", errorMsg)
	}

	return body
}

func (m OnboardingSubModel) submit() tea.Cmd {
	username := m.username.Value()
	password := m.password.Value()
	mode := m.subState

	return func() tea.Msg {
		_ = username
		_ = password
		_ = mode
		return AuthSucceededMsg{
			UserID:       "123456789",
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		}
	}
}
