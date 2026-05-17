package models

import (
	"context"
	"fmt"
	"time"

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
			m.subState = PullUserData
			cmds = append(cmds, m.pullUserData())

		case DataPullFailedMsg:
			m.subState = InitDatabase
			cmds = append(cmds, func() tea.Msg {
				return RootHandleErrorMsg{
					Err: fmt.Errorf("could not initialize database: %w", msg.Err),
				}
			})
		}

	case PullUserData:
		switch msg := msg.(type) {
		case DataPullSucceededMsg:
			m.subState = InitDatabase
			return m, func() tea.Msg {
				return RootUserDataInitializedMsg{}
			}

		case DataPullFailedMsg:
			m.subState = InitDatabase
			return m, func() tea.Msg {
				return RootHandleErrorMsg{
					Err: fmt.Errorf("failed synchronizing account data: %w", msg.Err),
				}
			}
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
	return func() tea.Msg {
		m.logger.Debug("attempting to init database")
		repo, err := storage.NewSqliteRepository(
			m.appContext.Config.LocalDataDir, m.appContext.Session.GetUserId(),
		)

		if err != nil {
			return DataPullFailedMsg{
				Err: err,
			}
		}

		ctx, cancel := context.WithTimeout(
			m.appContext.Ctx,
			5*time.Second,
		)
		defer cancel()

		err = repo.InitializeSchema(ctx)
		if err != nil {
			return DataPullFailedMsg{Err: err}
		}

		m.logger.Debug("database initialized")
		return DatabaseInitializedMsg{Repo: repo}
	}
}
func (m *PullDataModel) pullUserData() tea.Cmd {
	return func() tea.Msg {
		m.logger.Debug("starting to pull remote chat data")

		profCtx, cancelProf := context.WithTimeout(
			m.appContext.Ctx,
			5*time.Second,
		)
		defer cancelProf()

		profRes, err := m.appContext.ProfileClient.GetUserProfile(
			profCtx,
			&pb.GetUserProfileRequest{
				UserId: m.appContext.Session.GetUserId(),
			},
		)

		if err != nil {
			return DataPullFailedMsg{
				Err: fmt.Errorf("could not resolve user profile: %w", err),
			}
		}

		m.appContext.Session.SetProfile(&state.Profile{
			Id: profRes.UserId,
			Username: profRes.Username,
		})		

		chatCtx, cancelChat := context.WithTimeout(
			m.appContext.Ctx,
			5*time.Second,
		)
		defer cancelChat()
		chatsRes, err := m.appContext.MessagingClient.GetUserChats(
			chatCtx, &pb.GetUserChatsRequest{},
		)

		if err != nil {
			return DataPullFailedMsg{Err: tui.CleanGrpcError(err)}
		}

		for _, remoteChat := range chatsRes.Chats {
			chatId := remoteChat.Id
			if remoteChat.IsGroup {
				// chat data is self-contained via server tracking names
				// members for groups are pull when requested
				m.appContext.Session.InsertChat(&state.GroupChat{
					ChatId:    chatId,
					GroupName: remoteChat.Name,
				})
			} else {
				err := tui.ResolveDirectChat(m.appContext, chatId)
				if err != nil {
					return DataPullFailedMsg{Err: err}
				}
			}
		}

		m.logger.Debug("finished pulling remote data")
		return DataPullSucceededMsg{}
	}
}
