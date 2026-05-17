package messaging

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

// MockRepository testify mock
// This was generated fully by AI and reviewed
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Close() error {
	return nil
}

func (m *MockRepository) InitializeSchema(ctx context.Context) error {
	return nil
}

func (m *MockRepository) InsertMessage(ctx context.Context, message *Message) error {
	return nil
}

func (m *MockRepository) InsertMessageAcks(ctx context.Context, acks []MessageAck) error {
	return nil
}

func (m *MockRepository) SetMessageDelivered(ctx context.Context, msgId, userId string, t time.Time) error {
	return m.Called(ctx, msgId, userId, t).Error(0)
}

func (m *MockRepository) SetMessageRead(ctx context.Context, msgId, userId string, t time.Time) error {
	return m.Called(ctx, msgId, userId, t).Error(0)
}
func (m *MockRepository) GetMessageById(ctx context.Context, msgId string) (Message, error) {
	args := m.Called(ctx, msgId)
	return args.Get(0).(Message), args.Error(1)
}
func (m *MockRepository) GetChatMembers(ctx context.Context, chatId string) ([]string, error) {
	args := m.Called(ctx, chatId)
	return args.Get(0).([]string), args.Error(1)
}
func (m *MockRepository) GetMessageAcks(ctx context.Context, msgId string) ([]MessageAck, error) {
	args := m.Called(ctx, msgId)
	return args.Get(0).([]MessageAck), args.Error(1)
}
func (m *MockRepository) GetUserChats(ctx context.Context, userId string) ([]Chat, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]Chat), args.Error(1)
}
func (m *MockRepository) GetDirectChatByUsers(ctx context.Context, userA, userB string) (Chat, error) {
	args := m.Called(ctx, userA, userB)
	return args.Get(0).(Chat), args.Error(1)
}
func (m *MockRepository) GetChatById(ctx context.Context, chatId string) (Chat, error) {
	args := m.Called(ctx, chatId)
	return args.Get(0).(Chat), args.Error(1)
}
func (m *MockRepository) InsertChat(ctx context.Context, chat Chat, members []string) error {
	return m.Called(ctx, chat, members).Error(0)
}
func (m *MockRepository) AddChatMember(ctx context.Context, chatId, userId string) error {
	return m.Called(ctx, chatId, userId).Error(0)
}
func (m *MockRepository) RemoveChatMember(ctx context.Context, chatId, userId string) error {
	return m.Called(ctx, chatId, userId).Error(0)
}
func (m *MockRepository) InsertMessageWithAcks(ctx context.Context, msg *Message, acks []MessageAck) error {
	return m.Called(ctx, msg, acks).Error(0)
}
func (m *MockRepository) GetUndeliveredMessages(ctx context.Context, userId string) ([]Message, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]Message), args.Error(1)
}

// Helper to inject user claims into context
func contextWithUser(userId string) context.Context {
	return auth.ContextWithClaims(
		context.Background(),
		&auth.MessengerClaims{
			TokenType: string(auth.AccessTokenType),
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: userId,
			},
		})
}

func TestCreateDirectChat(t *testing.T) {
	logger := zap.NewNop() // keeps test logs clean

	tests := []struct {
		name          string
		ctx           context.Context
		req           *pb.CreateDirectChatRequest
		setupMock     func(m *MockRepository)
		expectedCode  codes.Code
		expectSuccess bool
	}{
		{
			name:         "Unauthenticated error",
			ctx:          context.Background(),
			req:          &pb.CreateDirectChatRequest{TargetUserId: "user_b"},
			setupMock:    func(m *MockRepository) {},
			expectedCode: codes.Unauthenticated,
		},
		{
			name:         "Cannot chat with yourself",
			ctx:          contextWithUser("user_a"),
			req:          &pb.CreateDirectChatRequest{TargetUserId: "user_a"},
			setupMock:    func(m *MockRepository) {},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "Idempotent fallback if chat already exists",
			ctx:  contextWithUser("user_a"),
			req:  &pb.CreateDirectChatRequest{TargetUserId: "user_b"},
			setupMock: func(m *MockRepository) {
				m.On("GetDirectChatByUsers", mock.Anything, "user_a", "user_b").
					Return(Chat{Id: "existing_chat_id"}, nil)
			},
			expectedCode:  codes.OK,
			expectSuccess: true,
		},
		{
			name: "Successful direct chat creation",
			ctx:  contextWithUser("user_a"),
			req:  &pb.CreateDirectChatRequest{TargetUserId: "user_b"},
			setupMock: func(m *MockRepository) {
				m.On("GetDirectChatByUsers", mock.Anything, "user_a", "user_b").
					Return(Chat{}, sql.ErrNoRows)
				m.On("InsertChat", mock.Anything, mock.Anything, []string{"user_a", "user_b"}).
					Return(nil)
			},
			expectedCode:  codes.OK,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			service := &MessagingService{repo: mockRepo, logger: logger}
			res, err := service.CreateDirectChat(tt.ctx, tt.req)

			if tt.expectedCode == codes.OK {
				assert.NoError(t, err)
				if tt.expectSuccess {
					assert.NotEmpty(t, res.ChatId)
				}
			} else {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestAddChatMember(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name         string
		ctx          context.Context
		req          *pb.AddChatMemberRequest
		setupMock    func(m *MockRepository)
		expectedCode codes.Code
	}{
		{
			name: "Caller not in chat - PermissionDenied",
			ctx:  contextWithUser("intruder"),
			req:  &pb.AddChatMemberRequest{ChatId: "group_1", TargetUserId: "user_c"},
			setupMock: func(m *MockRepository) {
				m.On("GetChatMembers", mock.Anything, "group_1").
					Return([]string{"user_a", "user_b"}, nil)
			},
			expectedCode: codes.PermissionDenied,
		},
		{
			name: "Target user already in chat - AlreadyExists",
			ctx:  contextWithUser("user_a"),
			req:  &pb.AddChatMemberRequest{ChatId: "group_1", TargetUserId: "user_b"},
			setupMock: func(m *MockRepository) {
				m.On("GetChatMembers", mock.Anything, "group_1").
					Return([]string{"user_a", "user_b"}, nil)
			},
			expectedCode: codes.AlreadyExists,
		},
		{
			name: "Cannot add member to direct chat room",
			ctx:  contextWithUser("user_a"),
			req:  &pb.AddChatMemberRequest{ChatId: "direct_1", TargetUserId: "user_c"},
			setupMock: func(m *MockRepository) {
				m.On("GetChatMembers", mock.Anything, "direct_1").
					Return([]string{"user_a", "user_b"}, nil)
				m.On("GetChatById", mock.Anything, "direct_1").
					Return(Chat{Id: "direct_1", IsGroup: false}, nil)
			},
			expectedCode: codes.InvalidArgument,
		},
		{
			name: "Successful member addition",
			ctx:  contextWithUser("user_a"),
			req:  &pb.AddChatMemberRequest{ChatId: "group_1", TargetUserId: "user_c"},
			setupMock: func(m *MockRepository) {
				m.On("GetChatMembers", mock.Anything, "group_1").
					Return([]string{"user_a", "user_b"}, nil)
				m.On("GetChatById", mock.Anything, "group_1").
					Return(Chat{Id: "group_1", IsGroup: true}, nil)
				m.On("AddChatMember", mock.Anything, "group_1", "user_c").
					Return(nil)
			},
			expectedCode: codes.OK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			service := &MessagingService{repo: mockRepo, logger: logger}
			_, err := service.AddChatMember(tt.ctx, tt.req)

			if tt.expectedCode == codes.OK {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				st, ok := status.FromError(err)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedCode, st.Code())
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
