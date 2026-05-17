package models

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	"gomessenger/internal/client/state"
	"gomessenger/internal/client/storage"
	"gomessenger/internal/client/tui"

	pb "gomessenger/generated"
)

// messages
type DataPullFailedMsg struct {
	Err error
}

type DatabaseInitializedMsg struct {
	Repo storage.Repository
}

type DataPullSucceededMsg struct{}

// states
type PullDataSubState int

const (
	InitDatabase PullDataSubState = iota
	PullUserData PullDataSubState = iota
)

type PullDataModel struct {
	subState PullDataSubState

	appContext *state.AppContext

	spin spinner.Model

	logger *zap.Logger
}

func NewPullDataModel(appContext *state.AppContext) *PullDataModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return &PullDataModel{
		subState:   InitDatabase,
		appContext: appContext,
		spin:       s,
		logger:     appContext.RootLogger.With(zap.String("mvc", "pull")),
	}
}

func (m *PullDataModel) Id() SubModelId {
	return ScreenPullData
}

func (m *PullDataModel) Init() tea.Cmd {
	return tea.Batch(m.spin.Tick, m.initDatabase())
}

func (m *PullDataModel) ShortHelp() []Binding {
	return []Binding{}
}

func (m *PullDataModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))
	var cmds []tea.Cmd

	var spinCmd tea.Cmd
	m.spin, spinCmd = m.spin.Update(msg)
	cmds = append(cmds, spinCmd)

	switch m.subState {
	case InitDatabase:
		switch msg := msg.(type) {
		case DatabaseInitializedMsg:
			m.logger.Info("local database initialized")
			m.appContext.LocalRepo = msg.Repo
			// return m, tea.Batch(m.spin.Tick, m.pullUserData())
			cmds = append(cmds, m.pullUserData())

		case DataPullFailedMsg:
			cmds = append(cmds, func() tea.Msg {
				return RootHandleErrorMsg{
					Err: fmt.Errorf("could not initialize database: %w", msg.Err),
				}
			})
		}

	case PullUserData:
		switch msg := msg.(type) {
		case DataPullFailedMsg:
			cmd := func() tea.Msg {
				return RootHandleErrorMsg{
					Err: fmt.Errorf("failed synchronizing account data: %w", msg.Err),
				}
			}
			return m, cmd
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *PullDataModel) View() tea.View {
	return tea.NewView("")
}

func (m *PullDataModel) ContentView(width, height int) tea.View {
	var statusText string
	switch m.subState {
	case InitDatabase:
		statusText = "Initializing local database..."
	case PullUserData:
		statusText = "Syncing chats and companion profiles..."
	}

	content := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.spin.View(),
		" ",
		statusText,
	)

	box := tui.DialogBoxStyle.Render(content)
	centered := lipgloss.Place(
		width, height, lipgloss.Center, lipgloss.Center, box,
	)
	return tea.NewView(centered)
}

func (m *PullDataModel) initDatabase() tea.Cmd {
	m.subState = InitDatabase
	return func() tea.Msg {
		m.logger.Debug("attempting to init database")
		repo, err := storage.NewSqliteRepository(
			m.appContext.Config.LocalDataDir, m.appContext.Session.UserId,
		)

		if err != nil {
			return DataPullFailedMsg{
				Err: err,
			}
		}

		err = repo.InitializeSchema(m.appContext.Ctx)
		if err != nil {
			return DataPullFailedMsg{Err: err}
		}

		m.logger.Debug("database initialized")
		return DatabaseInitializedMsg{Repo: repo}
	}
}

func (m *PullDataModel) pullUserData() tea.Cmd {
	m.subState = PullUserData
	return func() tea.Msg {
		m.logger.Debug("starting to pull remote chat data")

		// TODO: derive contexts with timeouts
		ctx := m.appContext.Ctx
		chatsRes, err := m.appContext.MessagingClient.GetUserChats(
			ctx, &pb.GetUserChatsRequest{},
		)

		if err != nil {
			return DataPullFailedMsg{Err: cleanGrpcError(err)}
		}

		for _, remoteChat := range chatsRes.Chats {
			chatId := remoteChat.Id
			if remoteChat.IsGroup {
				// chat data is self-contained via server tracking names
				// members for groups are pull when requested
				m.appContext.Session.Chats[chatId] = &state.GroupChat{
					ChatId:    chatId,
					GroupName: remoteChat.Name,
				}
			} else {
				err := m.resolveDirectChat(chatId)
				if err != nil {
					return DataPullFailedMsg{Err: err}
				}
			}
		}

		m.logger.Debug("finished pulling remote data")
		return DataPullSucceededMsg{}
	}
}

// fetch companion profile info from ProfileService
func (m *PullDataModel) resolveDirectChat(
	chatId string,
) error {
	ctx := m.appContext.Ctx
	membersResp, err := m.appContext.MessagingClient.GetChatMembers(
		ctx, &pb.GetChatMembersRequest{ChatId: chatId},
	)
	if err != nil {
		return cleanGrpcError(err)
	}

	m.appContext.Session.ChatMembers[chatId] = membersResp.MemberIds
	var companionId string
	for _, uid := range membersResp.MemberIds {
		if uid != m.appContext.Session.UserId {
			companionId = uid
			break
		}
	}

	if companionId == "" {
		// should not occur
		m.logger.Warn(
			"companion id is empty for chat", zap.String("id", chatId),
		)
		return nil
	}

	// TODO: timeout context
	ctx = m.appContext.Ctx
	session := m.appContext.Session

	profRes, err := m.appContext.ProfileClient.GetUserProfile(
		ctx,
		&pb.GetUserProfileRequest{
			UserId: companionId,
		},
	)

	if err != nil {
		m.logger.Warn("could not resolve direct chat profile",
			zap.String("companionId", companionId),
			zap.Error(err),
		)

		// populate a placeholder profile so rendering doesn't crash
		placeholder := &state.Profile{
			Id:       companionId,
			Username: fmt.Sprintf("User %s", companionId),
		}

		session.Profiles[companionId] = placeholder
		session.Chats[chatId] = &state.DirectChat{
			ChatId:       chatId,
			OtherProfile: placeholder,
		}
	}

	profile := &state.Profile{
		Id:       profRes.UserId,
		Username: profRes.Username,
	}
	session.Profiles[companionId] = profile
	session.Chats[chatId] = &state.DirectChat{
		ChatId:       chatId,
		OtherProfile: profile,
	}

	return nil
}

func cleanGrpcError(err error) error {
	if s, ok := status.FromError(err); ok {
		return fmt.Errorf("%s", s.Message())
	}
	return err
}
