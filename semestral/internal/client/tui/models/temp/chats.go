package temp 

// import (
// 	"fmt"
// 	"strings"
//
// 	tea "charm.land/bubbletea/v2"
// 	"charm.land/lipgloss/v2"
// )
//
// type ChatsListSection struct {
// 	SelectedIndex int
// 	Chats         []string
// }
//
// func NewChatsListSection() *ChatsListSection {
// 	return &ChatsListSection{
// 		SelectedIndex: 0,
// 		Chats:         []string{"Alice (Direct)", "Dev Group Chat", "Bob (Direct)"},
// 	}
// }
//
// func (cl *ChatsListSection) Update(msg tea.Msg) (*ChatsListSection, tea.Cmd) {
// 	switch msg := msg.(type) {
// 	case tea.KeyPressMsg:
// 		switch msg.String() {
// 		case "up", "k":
// 			if cl.SelectedIndex > 0 {
// 				cl.SelectedIndex--
// 			}
// 		case "down", "j":
// 			if cl.SelectedIndex < len(cl.Chats)-1 {
// 				cl.SelectedIndex++
// 			}
// 		case "d":
// 			return cl, func() tea.Msg { return OpenCreateChatMsg{IsGroup: false} }
// 		case "g":
// 			return cl, func() tea.Msg { return OpenCreateChatMsg{IsGroup: true} }
// 		}
// 	}
// 	return cl, nil
// }
//
// func (cl *ChatsListSection) View(width, height int, focused, engaged bool) string {
// 	borderColor := "#3C3C3C"
// 	if engaged && focused {
// 		borderColor = "#FF007F"
// 	} else if focused {
// 		borderColor = "#00FF00"
// 	}
//
// 	style := lipgloss.NewStyle().Width(width - 2).Height(height - 2).
// 		Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(borderColor)).Padding(0, 1)
//
// 	var renderedList []string
// 	renderedList = append(renderedList, "💬 CHATS LIST\n")
// 	
// 	for i, name := range cl.Chats {
// 		if i == cl.SelectedIndex && focused {
// 			renderedList = append(renderedList, fmt.Sprintf("👉 \033[1m%s\033[0m", name))
// 		} else {
// 			renderedList = append(renderedList, fmt.Sprintf("   %s", name))
// 		}
// 	}
//
// 	return style.Render(strings.Join(renderedList, "\n"))
// }
