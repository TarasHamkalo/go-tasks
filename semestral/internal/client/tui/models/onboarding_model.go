package models

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type AuthFailedMsg struct {
	Err error
}

type AuthSucceededMsg struct {
	UserId       string
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
	ctx           context.Context

	subState AuthSubState

	username textinput.Model
	userId   textinput.Model
	password textinput.Model
	spin     spinner.Model

	logger *zap.Logger
}

// Ensure 9 digits only
var userIdRegex = regexp.MustCompile(`^\d{9}$`)

func NewOnboardingModel(appContext *state.AppContext) *OnboardingSubModel {
	// Username Field (For Registration)
	username := textinput.New()
	username.CharLimit = 33
	username.Placeholder = "Username"
	username.SetWidth(30)
	username.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) < 3 || len(trimmed) > 32 {
			return errors.New("username must be 3-32 characters")
		}
		return nil
	}

	// User ID Field (For Login)
	userId := textinput.New()
	userId.CharLimit = 9
	userId.Placeholder = "9-Digit ID (e.g. 100000000)"
	userId.SetWidth(30)
	userId.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if !userIdRegex.MatchString(trimmed) {
			return errors.New("ID must be exactly 9 digits")
		}
		return nil
	}

	// Password Field (Shared)
	password := textinput.New()
	password.CharLimit = 73
	password.Placeholder = "Password"
	password.EchoMode = textinput.EchoPassword
	password.SetWidth(30)
	password.Validate = func(s string) error {
		trimmed := strings.TrimSpace(s)
		if strings.Contains(s, " ") {
			return errors.New("password cannot contain spaces")
		}
		if len(trimmed) < 8 || len(trimmed) > 72 {
			return errors.New("password must be 8-72 characters")
		}
		return nil
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	//TODO: remove
	userId.SetValue("100000000")
	password.SetValue("taras123")
	return &OnboardingSubModel{
		profileClient: appContext.ProfileClient,
		ctx:           appContext.Ctx,
		subState:      AuthPromptChoice,
		username:      username,
		userId:        userId,
		password:      password,
		spin:          s,
		logger:        appContext.RootLogger.With(zap.String("mvc", "onboarding")),
	}
}

func (m *OnboardingSubModel) Id() SubModelId {
	return ScreenOnboarding
}

func (m *OnboardingSubModel) Init() tea.Cmd {
	m.subState = AuthPromptChoice
	return textinput.Blink
}

func (m *OnboardingSubModel) ShortHelp() []Binding {
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

func (m *OnboardingSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// always tick the spinner if we are submitting
	if m.subState == AuthSubmitting {
		var spinCmd tea.Cmd
		m.spin, spinCmd = m.spin.Update(msg)
		cmds = append(cmds, spinCmd)
	}

	switch m.subState {
	case AuthPromptChoice:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "1":
				m.subState = AuthLoginForm
				m.userId.Focus()
				return m, textinput.Blink
			case "2":
				m.subState = AuthRegisterForm
				m.username.Focus()
				return m, textinput.Blink
			}
		}

	case AuthLoginForm:
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "tab":
				if m.userId.Focused() {
					m.userId.Blur()
					m.password.Focus()
				} else {
					m.password.Blur()
					m.userId.Focus()
				}
				return m, nil
			case "enter":
				if err := m.userId.Validate(m.userId.Value()); err != nil {
					return m, nil // Don't submit if invalid
				}
				if err := m.password.Validate(m.password.Value()); err != nil {
					return m, nil
				}

				m.subState = AuthSubmitting
				return m, tea.Batch(m.spin.Tick, m.submitLogin())

			case "esc":
				m.subState = AuthPromptChoice
				return m, nil
			}
		}

		if m.userId.Focused() {
			m.userId, cmd = m.userId.Update(msg)
		} else {
			m.password, cmd = m.password.Update(msg)
		}
		return m, cmd

	case AuthRegisterForm:
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
				if err := m.username.Validate(m.username.Value()); err != nil {
					return m, nil
				}
				if err := m.password.Validate(m.password.Value()); err != nil {
					return m, nil
				}

				m.subState = AuthSubmitting
				return m, tea.Batch(m.spin.Tick, m.submitRegister())
			case "esc":
				m.subState = AuthPromptChoice
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
		case AuthFailedMsg:
			m.subState = AuthPromptChoice
			m.password.SetValue("")
			m.password.Blur()
			return NewErrorSubModel(msg.Err, m), nil

		case AuthSucceededMsg:
			m.subState = AuthPromptChoice
			m.password.SetValue("")
			m.password.Blur()
			return m, func() tea.Msg { 
				return RootClientAuthenticatedMsg(msg)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *OnboardingSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *OnboardingSubModel) submitLogin() tea.Cmd {
	id := strings.TrimSpace(m.userId.Value())
	pass := []byte(m.password.Value())

	return func() tea.Msg {
		logCtx, cancelLog := context.WithTimeout(
			m.ctx,
			5*time.Second,
		)
		defer cancelLog()
		res, err := m.profileClient.Login(
			logCtx,
			&pb.LoginRequest{
				UserId:   id,
				Password: pass,
			},
		)

		if err != nil {
			// extract gRPC status
			if stat, ok := status.FromError(err); ok {
				return AuthFailedMsg{Err: errors.New(stat.Message())}
			}
			return AuthFailedMsg{Err: err}
		}

		return AuthSucceededMsg{
			UserId:       id,
			AccessToken:  res.Tokens.AccessToken,
			RefreshToken: res.Tokens.RefreshToken,
		}
	}
}

func (m *OnboardingSubModel) submitRegister() tea.Cmd {
	uname := strings.TrimSpace(m.username.Value())
	pass := []byte(m.password.Value())

	return func() tea.Msg {
		regCtx, cancelReg := context.WithTimeout(
			m.ctx,
			5*time.Second,
		)
		defer cancelReg()

		regRes, err := m.profileClient.RegisterProfile(
			regCtx,
			&pb.RegisterProfileRequest{
				Username: uname,
				Password: pass,
			},
		)

		if err != nil {
			if stat, ok := status.FromError(err); ok {
				return AuthFailedMsg{Err: errors.New(stat.Message())}
			}

			return AuthFailedMsg{Err: err}
		}

		logCtx, cancelLog := context.WithTimeout(
			m.ctx,
			5*time.Second,
		)
		defer cancelLog()

		logRes, err := m.profileClient.Login(
			logCtx,
			&pb.LoginRequest{
				UserId:   regRes.UserId,
				Password: pass,
			},
		)

		if err != nil {
			return AuthFailedMsg{
				Err: fmt.Errorf(
					"account created (ID: %s), but login failed: %v", regRes.UserId, err,
				),
			}
		}

		return AuthSucceededMsg{
			UserId:       regRes.UserId,
			AccessToken:  logRes.Tokens.AccessToken,
			RefreshToken: logRes.Tokens.RefreshToken,
		}
	}
}

func (m *OnboardingSubModel) ContentView(width, height int) tea.View {
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
		content = m.formView("Login", "User ID (9 digits):", m.userId)
	case AuthRegisterForm:
		content = m.formView("Register", "Username:", m.username)
	case AuthSubmitting:
		content = lipgloss.JoinHorizontal(
			lipgloss.Center,
			m.spin.View(),
			" Authenticating...",
		)
	}

	box := tui.DialogBoxStyle.Render(content)
	centered := lipgloss.Place(
		width, height, lipgloss.Center, lipgloss.Center, box,
	)

	return tea.NewView(centered)
}

func (m *OnboardingSubModel) formView(
	title, topLabel string, topInput textinput.Model,
) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(title),
		"",
		topLabel,
		topInput.View(),
		"",
		"Password:",
		m.password.View(),
	)

	// display validation errors
	var errStr string
	if topInput.Err != nil {
		errStr = topInput.Err.Error()
	} else if m.password.Err != nil {
		errStr = m.password.Err.Error()
	}

	if errStr != "" {
		errorMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Render(fmt.Sprintf("Error: %v", errStr))
		body = lipgloss.JoinVertical(lipgloss.Left, body, "", errorMsg)
	}

	return body
}
