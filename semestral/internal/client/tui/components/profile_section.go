package components

import (
	"fmt"
	"gomessenger/internal/client/state"

	"charm.land/lipgloss/v2"
)

func RenderProfileSection(
	profile *state.Profile, width, height int, focused, engaged bool,
) string {
	borderColor := "#3C3C3C" // Dim gray default
	if focused {
		borderColor = "#00FF00" // Green when selected in nav ring
	}

	style := lipgloss.NewStyle().
		Width(width-2).   // borders
		Height(height-2). // borders
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	content := fmt.Sprintf("U: %s\nID: %s", "Your name", "Your id")
	if profile == nil {
		content = fmt.Sprintf("U: %s\nID: %s", profile.Username, profile.Id)
	}

	return style.Render(content)
}
