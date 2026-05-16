package models

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ErrorSubModel struct {
	err      error
	returnTo SubModel // The state we pop back to when dismissed
}

func NewErrorSubModel(err error, returnTo SubModel) ErrorSubModel {
	return ErrorSubModel{
		err:      err,
		returnTo: returnTo,
	}
}

func (m ErrorSubModel) Id() SubModelId {
	return ScreenError
}

func (m ErrorSubModel) Init() tea.Cmd {
	return nil
}

func (m ErrorSubModel) ShortHelp() []Binding {
	return []Binding{
		{Key: "Enter/Esc", Description: "Dismiss Error"},
	}
}

func (m ErrorSubModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter", "esc":
			return m.returnTo, nil
		}
	}
	return m, nil
}

func (m ErrorSubModel) View() tea.View {
	return tea.NewView("")
}

func (m ErrorSubModel) ContentView(width, height int) tea.View {
	// Style the error specifically to look like an alert
	alertStyle := DialogBoxStyle.BorderForeground(lipgloss.Color("#FF0000"))

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000")).
			Render("ERROR"),
		lipgloss.JoinVertical(
			lipgloss.Center,
			"",
			"",
			m.err.Error(),
		),
	)

	box := alertStyle.Render(content)
	centered := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)

	return tea.NewView(centered)
}
