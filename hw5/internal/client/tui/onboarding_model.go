package tui

import (
	"context"
	"fmt"

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

type OnboardingModel struct {
	ctx context.Context

	subState AuthSubState

	username textinput.Model
	password textinput.Model

	logger *zap.Logger
	err    error
}

// TODO: a lot of verification of field inputs
func NewOnboardingModel(
	ctx context.Context,
	logger *zap.Logger,
) OnboardingModel {
	username := textinput.New()
	username.Placeholder = "Username"
	username.SetWidth(30)

	password := textinput.New()
	password.Placeholder = "Password"
	password.SetWidth(30)
	password.EchoMode = textinput.EchoPassword

	return OnboardingModel{
		ctx:      ctx,
		subState: AuthPromptChoice,
		username: username,
		password: password,
		logger:   logger,
	}
}

func (m OnboardingModel) Init() tea.Cmd {
	return nil
}

func (m OnboardingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			case "esc":
				return m, tea.Quit
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

func (m OnboardingModel) submit() tea.Cmd {
	username := m.username.Value()
	password := m.password.Value()
	mode := m.subState

	return func() tea.Msg {
		// TODO:
		// if mode == AuthRegisterForm {
		//     RegisterProfile(...)
		// }
		//
		// Login(...)
		//
		_ = username
		_ = password
		_ = mode
		// Temporary fake success for UI development.
		return AuthSucceededMsg{
			UserID:       "123456789",
			AccessToken:  "access-token",
			RefreshToken: "refresh-token",
		}
	}
}

func (m OnboardingModel) View() tea.View {
	switch m.subState {
	case AuthPromptChoice:
		return tea.NewView(
			"Go Messenger\n\n" +
				"1. Login\n" +
				"2. Register\n\n" +
				"[1|2] Select\n" +
				"[Esc] Quit",
		)

	case AuthLoginForm:
		return m.formView("Login")

	case AuthRegisterForm:
		return m.formView("Register")

	case AuthSubmitting:
		return tea.NewView("Authenticating...\n")
	}

	return tea.NewView("")
}

func (m OnboardingModel) formView(title string) tea.View {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Username:",
		m.username.View(),
		"",
		"Password:",
		m.password.View(),
		"",
		"[Tab] switch field",
		"[Enter] submit",
		"[Esc] back",
	)

	if m.err != nil {
		body += fmt.Sprintf("\n\nError: %v", m.err)
	}

	view := tea.NewView(body)
	return view
}
