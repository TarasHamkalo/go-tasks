package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
)

type DatabaseInitializedMsg struct {
	Repo storage.Repository
}

type PullDataSubState int

const (
	InitDatabase PullDataSubState = iota
	PullUserData PullDataSubState = iota
)

type PullDataModel struct {
	subState PullDataSubState

	appContext *state.AppContext

	spin spinner.Model

	logger *zap.Logger
}

func NewPullDataModel(appContext *state.AppContext) *PullDataModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &PullDataModel{
		subState:   InitDatabase,
		appContext: appContext,
		spin:       s,
		logger:     appContext.RootLogger.With(zap.String("mvc", "pull")),
	}
}

func (m *PullDataModel) Id() SubModelId {
	return ScreenPullData
}

func (m *PullDataModel) Init() tea.Cmd {
	// return m.initDatabase()
	return tea.Batch(m.spin.Tick, m.initDatabase())
}

func (m *PullDataModel) ShortHelp() []Binding {
	return []Binding{}
}

func (m *PullDataModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))
	var cmds []tea.Cmd

	var spinCmd tea.Cmd
	m.spin, spinCmd = m.spin.Update(msg)
	cmds = append(cmds, spinCmd)

	switch m.subState {
	case InitDatabase:
		switch msg := msg.(type) {
		case DatabaseInitializedMsg:
			m.logger.Info("local database initialized")
			m.appContext.LocalRepo = msg.Repo
			return TodoSubModel{}, nil

		case error:
			m.logger.Debug("got error")
			cmd := func() tea.Msg {
				return RootHandleErrorMsg(
					fmt.Errorf("could not initialize database: %w", msg),
				)
			}

			cmds := append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *PullDataModel) View() tea.View {
	return tea.NewView("")
}

func (m *PullDataModel) ContentView(width, height int) tea.View {
	return tea.NewView(
		fmt.Sprintf("%s Initializing local database...", m.spin.View()),
	)
}

func (m *PullDataModel) initDatabase() tea.Cmd {
	return func() tea.Msg {
		m.logger.Debug("attempting to init database")
		repo, err := storage.NewSqliteRepository(
			m.appContext.Config.LocalDataDir, m.appContext.Session.UserId,
		)
		if err != nil {
			return err
		}

		err = repo.InitializeSchema(m.appContext.Ctx)
		if err != nil {
			return err
		}

		m.logger.Debug("database initialized")
		return DatabaseInitializedMsg{
			Repo: repo,
		}
	}
}

// func (m OnboardingSubModel) submitLogin() tea.Cmd {
// 	id := strings.TrimSpace(m.userId.Value())
// 	pass := []byte(m.password.Value())
//
// 	return func() tea.Msg {
// 		res, err := m.profileClient.Login(m.ctx, &pb.LoginRequest{
// 			UserId:   id,
// 			Password: pass,
// 		})
//
// 		if err != nil {
// 			// extract gRPC status
// 			if stat, ok := status.FromError(err); ok {
// 				return errors.New(stat.Message())
// 			}
// 			return err
// 		}
//
// 		return AuthSucceededMsg{
// 			UserId:       id,
// 			AccessToken:  res.Tokens.AccessToken,
// 			RefreshToken: res.Tokens.RefreshToken,
// 		}
// 	}
// }
//
// func (m OnboardingSubModel) submitRegister() tea.Cmd {
// 	uname := strings.TrimSpace(m.username.Value())
// 	pass := []byte(m.password.Value())
//
// 	return func() tea.Msg {
// 		regRes, err := m.profileClient.RegisterProfile(
// 			m.ctx,
// 			&pb.RegisterProfileRequest{
// 				Username: uname,
// 				Password: pass,
// 			})
//
// 		if err != nil {
// 			if stat, ok := status.FromError(err); ok {
// 				return errors.New(stat.Message())
// 			}
// 			return err
// 		}
//
// 		// login to get tokens
// 		logRes, err := m.profileClient.Login(m.ctx, &pb.LoginRequest{
// 			UserId:   regRes.UserId,
// 			Password: pass,
// 		})
//
// 		if err != nil {
// 			return fmt.Errorf(
// 				"account created (ID: %s), but login failed: %v", regRes.UserId, err,
// 			)
// 		}
//
// 		return AuthSucceededMsg{
// 			UserId:       regRes.UserId,
// 			AccessToken:  logRes.Tokens.AccessToken,
// 			RefreshToken: logRes.Tokens.RefreshToken,
// 		}
// 	}
// }
//
// func (m OnboardingSubModel) ContentView(width, height int) tea.View {
// 	var content string
//
// 	switch m.subState {
// 	case AuthPromptChoice:
// 		content = lipgloss.JoinVertical(
// 			lipgloss.Center,
// 			lipgloss.NewStyle().Bold(true).Render("Go Messenger"),
// 			"",
// 			"1. Login   ",
// 			"2. Register",
// 		)
// 	case AuthLoginForm:
// 		content = m.formView("Login", "User ID (9 digits):", m.userId)
// 	case AuthRegisterForm:
// 		content = m.formView("Register", "Username:", m.username)
// 	case AuthSubmitting:
// 		content = lipgloss.JoinHorizontal(
// 			lipgloss.Center,
// 			m.spin.View(),
// 			" Authenticating...",
// 		)
// 	}
//
// 	box := tui.DialogBoxStyle.Render(content)
// 	centered := lipgloss.Place(
// 		width, height, lipgloss.Center, lipgloss.Center, box,
// 	)
//
// 	return tea.NewView(centered)
// }
//
// func (m OnboardingSubModel) formView(
// 	title, topLabel string, topInput textinput.Model,
// ) string {
// 	body := lipgloss.JoinVertical(
// 		lipgloss.Left,
// 		lipgloss.NewStyle().Bold(true).Render(title),
// 		"",
// 		topLabel,
// 		topInput.View(),
// 		"",
// 		"Password:",
// 		m.password.View(),
// 	)
//
// 	// display validation errors
// 	var errStr string
// 	if topInput.Err != nil {
// 		errStr = topInput.Err.Error()
// 	} else if m.password.Err != nil {
// 		errStr = m.password.Err.Error()
// 	}
//
// 	if errStr != "" {
// 		errorMsg := lipgloss.NewStyle().
// 			Foreground(lipgloss.Color("#FF0000")).
// 			Render(fmt.Sprintf("Error: %v", errStr))
// 		body = lipgloss.JoinVertical(lipgloss.Left, body, "", errorMsg)
// 	}
//
// 	return body
// }
