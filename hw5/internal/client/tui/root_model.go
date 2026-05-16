package tui

import (
	"context"
	"crypto/tls"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gomessenger/generated" 
)

const PROFILES_API_URL = "localhost:8081"
const MESSAGING_API_URL = "localhost:8082"

type RootModel struct {
	state ProgramState
	ctx   context.Context

	logger *zap.Logger

	// Sub-models
	onboarding OnboardingModel

	// gRPC clients and sec
	tlsCfg *tls.Config

	profileClient   pb.ProfileServiceClient
	messagingClient pb.MessagingServiceClient
}

func NewRootModel(
	ctx context.Context,
	tlsCfg *tls.Config,
	logger *zap.Logger,
) *RootModel {
	return &RootModel{
		state: ProgramState{
			Id:     StateOnboarding,
			UserId: "",
		},
		ctx:        ctx,
		logger:     logger.With(zap.String("module", "root")),
		onboarding: NewOnboardingModel(ctx, logger.With(zap.String("module", "onboarding"))),
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.onboarding.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: i am logging REFRESH TOKEN ==)
	m.logger.Info("received message", zap.Any("msg", msg))

	switch msg := msg.(type) {
	case AuthSucceededMsg:
		m.logger.Info(
			"authentication succeeded, initializing client state",
			zap.String("userId", msg.UserID),
		)

		// 1. Save tokens to tokens-<userId>.json
		err := saveTokens(msg.UserID, msg.AccessToken, msg.RefreshToken)
		if err != nil {
			m.logger.Error("failed to save tokens", zap.Error(err))
			return m, tea.Quit
		}

		m.logger.Info("tokens saved to disk")

		// 2. Create authenticated gRPC connection configs
		authCreds := TokenAuth{
			accessToken: msg.AccessToken,
			refreshToken: msg.RefreshToken,
		}

		opts := []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(m.tlsCfg)),
			grpc.WithPerRPCCredentials(authCreds),
		}

		// 3. Dial Profiles (8081)
		// TODO: define which paths require authorization 
		// TODO: after creation of profile client path it to tokens auth object
		profileConn, err := grpc.NewClient(PROFILES_API_URL, opts...)
		if err != nil {
			m.logger.Fatal("failed to connect to profile server", zap.Error(err))
		}
		m.profileClient = pb.NewProfileServiceClient(profileConn)

		// 4. Dial Messaging (8082)
		messagingConn, err := grpc.NewClient(MESSAGING_API_URL, opts...)
		if err != nil {
			m.logger.Fatal("failed to connect to messaging server", zap.Error(err))
		}
		m.messagingClient = pb.NewMessagingServiceClient(messagingConn)

		m.logger.Info("gRPC clients configured and ready. Exiting as requested.")

		fmt.Printf("Success! Logged in as %s. Tokens saved.\n", msg.UserID)
		return m, tea.Quit
	}

	switch m.state.Id {
	case StateOnboarding:
		model, cmd := m.onboarding.Update(msg)
		m.onboarding = model.(OnboardingModel)
		return m, cmd

	case StateChatList:
		return m, tea.Quit
	}

	return m, nil
}

func (m *RootModel) View() tea.View {
	switch m.state.Id {
	case StateOnboarding:
		return m.onboarding.View()

	case StateChatList:
		return tea.NewView("Chat list goes here.\n")

		// case StateActiveChat:
		// 	return tea.NewView("Active chat goes here.\n")
	}

	return tea.NewView("")
}
