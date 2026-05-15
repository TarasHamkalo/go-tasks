package messaging

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
	// "gomessenger/internal/auth"
)

type MessagingService struct {
	repo Repository 

	logger *zap.Logger

	pb.UnimplementedMessagingServiceServer
}

func NewMessagingService(
	repo Repository,
	logger *zap.Logger,
) *MessagingService{
	return &MessagingService{
		repo: repo,
		logger: logger,
	}
}

func (s *MessagingService) SendMessage(
	ctx context.Context, req *pb.SendMessageRequest,
) (*pb.SendMessageResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		s.logger.Error(
			"missing authentication claims in context",
		)

		return nil, status.Error(
			codes.Internal,
			"missing authentication claims",
		)
	}

	userId := claims.Subject
	sentAt := time.Now().UTC()

	message := Message{
		Id:       uuid.New().String(),
		ChatId:   req.ChatId,
		SenderId: userId,
		Content:  req.Content,
		SentAt:   sentAt,
	}

	chatMembers, err := s.repo.GetChatMembers(ctx, req.ChatId)
	if err != nil {
		s.logger.Error(
			"could not retrieve chat members",
			zap.Error(err),
			zap.String("chatId", req.ChatId),
			zap.String("userId", userId),
		)

		return nil, status.Error(codes.Internal, "could not retrieve chat members")
	}

	userIsPartOfChat := false
	acks := make([]MessageAck, 0, len(chatMembers))

	for _, chatMember := range chatMembers {
		if chatMember == userId {
			// verify that the sender belongs to the chat and
			// do not create an ack entry for the sender
			userIsPartOfChat = true
			continue
		}

		acks = append(acks, MessageAck{
			MessageId: message.Id,
			UserId:    chatMember,
		})
	}

	if !userIsPartOfChat {
		s.logger.Warn(
			"user attempted to send message to a chat they do not belong to",
			zap.String("chatId", req.ChatId),
			zap.String("userId", userId),
		)

		return nil, status.Error(
			codes.PermissionDenied,
			"user is not a part of given chat",
		)
	}

	err = s.repo.InsertMessageWithAcks(ctx, &message, acks)
	if err != nil {
		s.logger.Error(
			"could not store message and acknowledgements",
			zap.Error(err),
			zap.String("messageId", message.Id),
			zap.String("chatId", req.ChatId),
			zap.String("userId", userId),
		)

		return nil, status.Error(
			codes.Internal,
			"could not store message, try again later",
		)
	}

	s.logger.Info(
		"message stored successfully",
		zap.String("messageId", message.Id),
		zap.String("chatId", req.ChatId),
		zap.String("userId", userId),
		zap.Int("recipientCount", len(acks)),
	)

	// TODO: Push the message to all currently connected recipients.
	// The session manager should iterate over recipients from `acks`
	// and forward the message to all active streams for each user.

	return &pb.SendMessageResponse{
		MsgId: message.Id,
	}, nil
}
