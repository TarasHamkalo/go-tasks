package models

import (
	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"

	"gomessenger/internal/client/tui/app"
)

type Screen int

const (
	ScreenOnboarding Screen = iota
	TODOScreen Screen = iota
)

type RootModel struct {
	screen     Screen
	appContext *app.AppContext
	session    *app.SessionState

	logger *zap.Logger

	onboarding OnboardingModel
}

func NewRootModel(appContext *app.AppContext) *RootModel {
	return &RootModel{
		screen:     ScreenOnboarding,
		appContext: appContext,
		session:    &app.SessionState{},
		logger:     appContext.RootLogger.With(zap.String("mvc", "root")),
		onboarding: NewOnboardingModel(appContext),
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.onboarding.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))

	switch msg := msg.(type) {
	case AuthSucceededMsg:
		m.logger.Info("authentication succeeded", zap.String("userId", msg.UserID))

		// TODO: here you can store tokens to some keyring (or file ==))
		// inject new tokens into the active gRPC Interceptor
		m.appContext.CredentialsInterceptor.SetTokens(
			msg.UserID, msg.AccessToken, msg.RefreshToken,
		)

		m.session.UserId = msg.UserID
		m.screen = TODOScreen 

		return m, tea.Quit
	}

	switch m.screen {
	case ScreenOnboarding:
		model, cmd := m.onboarding.Update(msg)
		m.onboarding = model.(OnboardingModel)
		return m, cmd
	case TODOScreen:
		m.logger.Info("reached todo screen")
		return m, tea.Quit
	}

	return m, nil
}

func (m *RootModel) View() tea.View {
	switch m.screen {
	case ScreenOnboarding:
		return m.onboarding.View()

	case TODOScreen:
		return tea.NewView("TODO screen\n")

		// case StateActiveChat:
		// 	return tea.NewView("Active chat goes here.\n")
	}

	return tea.NewView("")
}
