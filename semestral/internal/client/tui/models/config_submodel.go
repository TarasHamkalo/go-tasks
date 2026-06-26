package models

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

// ConfigSubModel asks whether the user wants to appear invisible
// (i.e. not update presence to "online") during this session.
type ConfigSubModel struct {
	appContext *state.AppContext

	// false = visible, true = invisible
	isInvisible bool
}

func NewConfigSubModel(appContext *state.AppContext) *ConfigSubModel {
	return &ConfigSubModel{
		appContext:  appContext,
		isInvisible: appContext.Session.IsInvisible(),
	}
}

func (m *ConfigSubModel) Id() SubModelId {
	return ScreenConfiguration
}

func (m *ConfigSubModel) Init() tea.Cmd {
	return nil
}

func (m *ConfigSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.isInvisible = false

	case "down", "j":
		m.isInvisible = true

	case "enter":
		m.appContext.Session.SetInvisible(m.isInvisible)

		return m, func() tea.Msg {
			return RootClientConfigurationSuccessMsg{}
		}
	}

	return m, nil
}

func (m *ConfigSubModel) View() tea.View {
	return tea.NewView("")
}

func (m *ConfigSubModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "up/down, j/k", Description: "Toggle"},
		{Key: "Enter", Description: "Continue"},
	}
}

func (m *ConfigSubModel) ContentView(width, height int) tea.View {
	title := lipgloss.NewStyle().
		Bold(true).
		Render("Session Configuration")

	description := "Choose whether your status should be visible to others."

	visibleOption := "[ ] Visible (show online status)"
	invisibleOption := "[ ] Invisible (appear offline)"

	if !m.isInvisible {
		visibleOption = "[x] Visible (show online status)"
	} else {
		invisibleOption = "[x] Invisible (appear offline)"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		description,
		"",
		visibleOption,
		invisibleOption,
		"",
		"Press Enter to continue.",
	)

	box := tui.DialogBoxStyle.Render(content)

	centered := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		box,
	)

	return tea.NewView(centered)
}
