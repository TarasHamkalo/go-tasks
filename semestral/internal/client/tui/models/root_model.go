// Package models contains Bubble Tea models composing the terminal user
// interface. The UI is organized as nested models that react to messages
// and execute asynchronous commands to communicate with gRPC services.
// With a some practice and time, it is possible to make this communication nice, i did not manage though :)
// See readme for better explanation.
package models

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type RootHandleErrorMsg struct {
	Err error
}

type RootClientAuthenticatedMsg struct {
	UserId       string
	AccessToken  string
	RefreshToken string
}

type RootUserDataInitializedMsg struct {
}

type RootClientConfigurationSuccessMsg struct{}

type RootModel struct {
	currentSubModel SubModel

	width  int
	height int

	appContext *state.AppContext

	logger *zap.Logger

	onboardingSubModel *OnboardingSubModel

	pullDataModel *PullDataModel

	configModel *ConfigSubModel

	chatModel *ChatModel
}

func NewRootModel(appContext *state.AppContext) *RootModel {
	appContext.Session = state.NewSession() // empty state

	onboardingSubModel := NewOnboardingModel(appContext)
	pullDataModel := NewPullDataModel(appContext)
	chatModel := NewChatModel(appContext)
	configModel := NewConfigSubModel(appContext)

	return &RootModel{
		currentSubModel: onboardingSubModel,
		appContext:      appContext,
		logger:          appContext.RootLogger.With(zap.String("mvc", "root")),

		// to reuse all
		onboardingSubModel: onboardingSubModel,
		pullDataModel:      pullDataModel,
		chatModel:          chatModel,
		configModel:        configModel,
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.currentSubModel.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			m.appContext.Session = state.NewSession() // empty state
			return m, tea.Quit
		}

	case RootHandleErrorMsg:
		m.logger.Info("root handling error")
		m.appContext.Session = state.NewSession() // empty state
		m.currentSubModel = NewErrorSubModel(msg.Err, m.onboardingSubModel)
		return m, m.currentSubModel.Init()

	case RootClientAuthenticatedMsg:
		m.logger.Info("authentication succeeded", zap.String("userId", msg.UserId))

		// TODO: here you can store tokens to some keyring (or file ==))

		// inject new tokens into the active gRPC Interceptor
		err := m.appContext.CredentialsInterceptor.SetTokens(
			msg.UserId, msg.AccessToken, msg.RefreshToken,
		)
		if err != nil {
			m.currentSubModel = NewErrorSubModel(err, m.onboardingSubModel)
			return m, nil
		}

		m.appContext.Session.SetUserId(msg.UserId)
		m.currentSubModel = m.pullDataModel
		return m, m.currentSubModel.Init()

	case RootUserDataInitializedMsg:
		m.logger.Info("user data initialization complete")
		m.currentSubModel = m.configModel
		return m, m.currentSubModel.Init()
	case RootClientConfigurationSuccessMsg:
		m.currentSubModel = m.chatModel
		return m, m.currentSubModel.Init()
	}

	nextSubModel, cmd := m.currentSubModel.Update(msg)
	m.currentSubModel, _ = nextSubModel.(SubModel)

	return m, cmd
}

func (m *RootModel) View() tea.View {
	if m.currentSubModel == nil {
		return tea.NewView("")
	}

	// allocate space for footer
	contentHeight := max(m.height-5, 1)
	content := m.currentSubModel.ContentView(m.width, contentHeight)
	footer := renderFooter(m.currentSubModel.ShortHelp())
	view := tea.NewView(tui.AppStyle.
		Width(m.width).
		Height(m.height).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			content.Content,
			footer,
		)))

	view.AltScreen = true
	return view
}

func renderFooter(bindings []tui.Binding) string {
	parts := make([]string, 0, len(bindings)+2)
	for _, b := range bindings {
		parts = append(
			parts,
			fmt.Sprintf("[%s] %s", b.Key, b.Description),
		)
	}

	parts = append(parts, fmt.Sprintf("[%s] %s", "Ctrl+C", "Quit"))
	parts = append(parts, fmt.Sprintf("[%s] %s", "Ctrl+D", "Quit"))
	return tui.FooterStyle.Render(strings.Join(parts, " • "))
}
