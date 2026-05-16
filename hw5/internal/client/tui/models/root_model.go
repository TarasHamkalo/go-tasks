package models

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"

	"gomessenger/internal/client/tui/app"
)

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	footerStyle = lipgloss.NewStyle().
			BorderTop(true).
			Padding(0, 1).
			Faint(true)
)

type RootModel struct {
	currentSubModel SubModel

	width  int
	height int

	appContext *app.AppContext
	session    *app.SessionState

	logger *zap.Logger

	onboardingSubModel OnboardingSubModel
}

func NewRootModel(appContext *app.AppContext) *RootModel {
	onboardingSubModel := NewOnboardingModel(appContext)
	return &RootModel{
		currentSubModel: onboardingSubModel,
		appContext:      appContext,
		session:         &app.SessionState{},
		logger:          appContext.RootLogger.With(zap.String("mvc", "root")),

		// to reuse all
		onboardingSubModel: onboardingSubModel,
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.currentSubModel.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+q":
			return m, tea.Quit
		}

	case AuthSucceededMsg:
		m.logger.Info("authentication succeeded", zap.String("userId", msg.UserID))

		// TODO: here you can store tokens to some keyring (or file ==))
		// inject new tokens into the active gRPC Interceptor
		err := m.appContext.CredentialsInterceptor.SetTokens(
			msg.UserID, msg.AccessToken, msg.RefreshToken,
		)
		if err != nil {
			// TODO: show error to user
			return m, tea.Quit
		}

		m.session.UserId = msg.UserID
		m.currentSubModel = TodoSubModel{}

		return m, tea.Quit
	}

	nextSubModel, cmd := m.currentSubModel.Update(msg)
	m.currentSubModel = nextSubModel.(SubModel)
	return m, cmd
}

func (m *RootModel) View() tea.View {
	if m.currentSubModel == nil {
		return tea.NewView("")
	}

	// footer size
	contentHeight := m.height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	content := m.currentSubModel.ContentView(m.width, contentHeight)
	footer := renderFooter(m.currentSubModel.ShortHelp())
	return tea.NewView(appStyle.
		Width(m.width).
		Height(m.height).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			content.Content,
			footer,
		)))
}

func renderFooter(bindings map[string]string) string {
	parts := make([]string, 0, len(bindings)+2)
	for k, v := range bindings {
		parts = append(parts, fmt.Sprintf("[%s] %s", k, v))
	}

	parts = append(parts, fmt.Sprintf("[%s] %s", "Ctrl+C", "Quit"))
	parts = append(parts, fmt.Sprintf("[%s] %s", "Ctrl+D", "Quit"))

	return footerStyle.Render(strings.Join(parts, " • "))
}
