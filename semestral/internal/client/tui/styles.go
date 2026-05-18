package tui 

import "charm.land/lipgloss/v2"

var (
	AppStyle = lipgloss.NewStyle().
			Padding(1, 2)

	FooterStyle = lipgloss.NewStyle().
			BorderTop(true).
			Padding(0, 1).
			Faint(true)

	DialogBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2).
		Width(55) 

)

