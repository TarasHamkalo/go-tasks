package models

import (
	"context"
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"gomessenger/internal/client/tui"
)

type ChatSelectedMsg struct {
	ChatId string
}

type ChatInfoPulledMsg struct {
	ChatId string
}

type ChatsListModel struct {
	appContext *state.AppContext

	cursor  int
	engaged bool

	// Cached session version to avoid rebuilding rows unnecessarily
	lastVersion int64

	rows []chatRow
}

type chatRow struct {
	ChatId  string
	Label   string
	Unread  int
	IsGroup bool
}

func NewChatsListModel(appContext *state.AppContext) *ChatsListModel {
	return &ChatsListModel{
		appContext:  appContext,
		cursor:      0,
		lastVersion: -1,
		rows:        []chatRow{},
	}
}

func (m *ChatsListModel) Init() tea.Cmd {
	return nil
}

func (m *ChatsListModel) View() tea.View {
	return tea.NewView(m.ContentView(80, 24, false))
}

func (m *ChatsListModel) SetEngaged(engaged bool) tea.Cmd {
	m.engaged = engaged
	return nil
}

func (m *ChatsListModel) ShortHelp() []tui.Binding {
	return []tui.Binding{
		{Key: "j, down", Description: "Move down"},
		{Key: "k, up", Description: "Move up"},
		{Key: "Enter", Description: "Open chat"},
	}
}

func (m *ChatsListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ChatInfoPulledMsg:
		return m, nil
	case IncomingMessageMsg:
		chatId := msg.Message.ChatId
		_, ok := m.appContext.Session.GetChat(chatId)
		if !ok {
			return m, m.pullChatInfo(chatId)
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if len(m.rows) == 0 {
				return m, nil
			}

			selected := m.rows[m.cursor]
			return m, func() tea.Msg {
				return ChatSelectedMsg{
					ChatId: selected.ChatId,
				}
			}
		}
	}

	return m, nil
}

func (m *ChatsListModel) pullChatInfo(chatId string) tea.Cmd {
	return func() tea.Msg {
		chatCtx, cancelChat := context.WithTimeout(
			m.appContext.Ctx,
			5*time.Second,
		)
		defer cancelChat()

		chatsRes, err := m.appContext.MessagingClient.GetChatById(
			chatCtx, &pb.GetChatByIdRequest{Id: chatId},
		)
		if err != nil {
			return ChatModelHandleErrorMsg{Err: err}
		}

		if chatsRes.Chat.IsGroup {
			// chat data is self-contained via server tracking names
			// members for groups are pull when requested
			m.appContext.Session.InsertChat(&state.GroupChat{
				ChatId:    chatId,
				GroupName: chatsRes.Chat.Name,
			})
		} else {
			err := tui.ResolveDirectChatToSession(m.appContext, chatId)
			if err != nil {
				return ChatModelHandleErrorMsg{Err: err}
			}
		}
		return ChatInfoPulledMsg{ChatId: chatId}
	}
}

func (m *ChatsListModel) ContentView(width int, height int, focused bool) string {
	m.refreshRows()

	borderColor := "#3C3C3C"
	if m.engaged && focused {
		borderColor = "#FF007F"
	} else if focused {
		borderColor = "#00FF00"
	}

	style := lipgloss.NewStyle().
		Width(width-2).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(0, 1)

	if len(m.rows) == 0 {
		return style.Render("No chats")
	}

	innerWidth := max(1, width-6)
	visibleRows := max(1, height-2)

	start := 0
	if m.cursor >= visibleRows {
		start = m.cursor - visibleRows + 1
	}

	end := min(len(m.rows), start+visibleRows)
	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		row := m.rows[i]
		prefix := "  "
		if m.engaged && i == m.cursor {
			prefix = "> "
		}

		label := row.Label
		if row.IsGroup {
			label = "(G) " + label
		}

		if row.Unread > 0 {
			label += " (*)"
		}

		maxLabelWidth := max(1, innerWidth-len(prefix))
		label = truncate(label, maxLabelWidth)
		lines = append(lines, prefix+label)
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// refreshRows rebuilds the rendered chat list only when session data changes
func (m *ChatsListModel) refreshRows() {
	version := m.appContext.Session.Version()
	if version == m.lastVersion {
		return
	}

	chats := m.appContext.Session.GetChatsSnapshot()

	rows := make([]chatRow, 0, len(chats))

	for chatId, chat := range chats {
		rows = append(rows, chatRow{
			ChatId: chatId,
			Label:  chat.Name(),
			Unread: m.appContext.Session.GetUnread(chatId),
		})
	}

	// ordering by label
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Label < rows[j].Label
	})

	m.rows = rows
	m.lastVersion = version
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}

	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

func truncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}

	if width <= 3 {
		return string(runes[:width])
	}

	return string(runes[:width-3]) + "..."
}
