package tui

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

type RootModel struct {
	state ProgramState
	ctx   context.Context

	logger *zap.Logger

	// Sub-models
	onboarding OnboardingModel

	// gRPC clients and sec
	verificationKey *rsa.PublicKey
	tlsCfg          *tls.Config

	tokenCredentialsInterceptor *auth.TokenCredentialsInterecptor

	profileClient   pb.ProfileServiceClient
	messagingClient pb.MessagingServiceClient
}

func NewRootModel(
	ctx context.Context,
	issuer string,
	verificationKey *rsa.PublicKey,
	tlsCfg *tls.Config,
	logger *zap.Logger,
) *RootModel {
	state := ProgramState{
		Id:              StateOnboarding,
		UserId:          "",
		ProfilesApiUrl:  "localhost:8081",
		MessagingApiUrl: "localhost:8082",
	}

	authInterceptor := auth.NewTokenCredentialsInterecptor(
		issuer,
		verificationKey,
		map[string]bool{
			pb.ProfileService_RegisterProfile_FullMethodName: true,
			pb.ProfileService_Login_FullMethodName:           true,
			pb.ProfileService_Refresh_FullMethodName:         true,
		},
		logger.With(zap.String("module", "auth-interceptor")),
	)

	// dial both servers at application startup
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithPerRPCCredentials(authInterceptor),
	}

	profileConn, err := grpc.NewClient(state.ProfilesApiUrl, opts...)
	if err != nil {
		logger.Fatal("failed to connect to profile server", zap.Error(err))
	}
	profileClient := pb.NewProfileServiceClient(profileConn)

	messagingConn, err := grpc.NewClient(state.MessagingApiUrl, opts...)
	if err != nil {
		logger.Fatal("failed to connect to messaging server", zap.Error(err))
	}
	messagingClient := pb.NewMessagingServiceClient(messagingConn)

	authInterceptor.SetProfileClient(profileClient)

	return &RootModel{
		state:  state,
		ctx:    ctx,
		logger: logger.With(zap.String("module", "root")),

		tlsCfg: tlsCfg,

		tokenCredentialsInterceptor: authInterceptor,

		profileClient:   profileClient,
		messagingClient: messagingClient,

		// Pass the actual profileClient down so Onboarding can run the Login RPC!
		onboarding: NewOnboardingModel(
			ctx,
			logger.With(zap.String("module", "onboarding")),
			profileClient,
		),
	}
}

func (m *RootModel) Init() tea.Cmd {
	return m.onboarding.Init()
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.logger.Info("received message", zap.Any("msg", msg))

	switch msg := msg.(type) {
	case AuthSucceededMsg:
		m.logger.Info("authentication succeeded", zap.String("userId", msg.UserID))

		// TODO: here you can store tokens to some keyring (or file ==))
		// inject new tokens into the active gRPC Interceptor
		m.tokenCredentialsInterceptor.SetTokens(
			msg.UserID, msg.AccessToken, msg.RefreshToken,
		)

		m.state.UserId = msg.UserID
		m.state.Id = StateChatList

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
