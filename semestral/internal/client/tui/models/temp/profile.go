package temp 

// import (
// 	"fmt"
//
// 	tea "charm.land/bubbletea/v2"
// 	"charm.land/lipgloss/v2"
// )
// type ProfileSection struct {
// 	Username string
// 	UserID   string
// }
//
// func (p ProfileSection) Update(msg tea.Msg) (ProfileSection, tea.Cmd) { return p, nil }
//
// func (p ProfileSection) View(width, height int, focused, engaged bool) string {
// 	borderColor := "#3C3C3C" // Dim gray default
// 	if focused {
// 		borderColor = "#00FF00" // Green when selected in nav ring
// 	}
//
// 	style := lipgloss.NewStyle().
// 		Width(width - 2).   // Account for borders
// 		Height(height - 2). // Account for borders
// 		Border(lipgloss.RoundedBorder()).
// 		BorderForeground(lipgloss.Color(borderColor)).
// 		Padding(0, 1)
//
// 	content := fmt.Sprintf("👤 %s\nID: %s", p.Username, p.UserID)
// 	return style.Render(content)
// }
