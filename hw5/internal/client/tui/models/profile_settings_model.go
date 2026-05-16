package models

import (
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"
)

type ProfileSettingsDialog struct {
	appContext *state.AppContext
	logger     *zap.Logger
	username   textinput.Model
}

func NewProfileSettingsDialog(appContext *state.AppContext, currentUsername string) ProfileSettingsDialog {
	ti := textinput.New()
	ti.SetValue(currentUsername)
	ti.Focus()
	ti.CharLimit = 32

	return ProfileSettingsDialog{
		appContext: appContext,
		logger:     appContext.RootLogger.With(zap.String("mvc", "profile_dialog")),
		username:   ti,
	}
}

func (m ProfileSettingsDialog) Id() SubModelId { return 999 } 

func (m ProfileSettingsDialog) Init() tea.Cmd  { return textinput.Blink }

func (m ProfileSettingsDialog) ShortHelp() []Binding {
	return []Binding{
		{Key: "Enter", Description: "Save"},
		{Key: "Esc", Description: "Cancel"},
	}
}

func (m ProfileSettingsDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "enter":
			m.logger.Info("user requested profile update", zap.String("new_name", m.username.Value()))
			// TODO: Call ProfileService to update username
			return m, func() tea.Msg { return CloseDialogMsg{} }
		}
	}

	m.username, cmd = m.username.Update(msg)
	return m, cmd
}

func (m ProfileSettingsDialog) View() tea.View { return tea.NewView("") }

func (m ProfileSettingsDialog) ContentView(width, height int) tea.View {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("👤 EDIT PROFILE"),
		"",
		"New Username:",
		m.username.View(),
		"",
		"[Enter] Save  •  [Esc] Cancel",
	)

	box := tui.DialogBoxStyle.Render(content)
	centered := lipgloss.Place(
		width, height, lipgloss.Center, lipgloss.Center, box,
	)

	return tea.NewView(centered)
}
