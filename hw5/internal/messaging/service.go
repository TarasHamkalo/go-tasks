package messaging

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

type MessagingService struct {
	repo Repository

	broker *Broker

	logger *zap.Logger

	pb.UnimplementedMessagingServiceServer
}

func NewMessagingService(
	repo Repository,
	logger *zap.Logger,
) *MessagingService {
	broker := NewBroker(logger.With(zap.String("module", "broker")))
	return &MessagingService{
		repo:   repo,
		broker: broker,
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

	// TODO: refactor all references / pass by values
	go s.broker.Publish(&message, acks)

	return &pb.SendMessageResponse{
		MsgId: message.Id,
	}, nil
}

func (s *MessagingService) Subscribe(
	req *pb.SubscribeRequest,
	stream grpc.ServerStreamingServer[pb.ServerEvent],
) error {
	claims, ok := auth.ClaimsFromContext(stream.Context())
	if !ok {
		s.logger.Error(
			"missing authentication claims in context",
			zap.String("addr", peerAddress(stream.Context())),
		)

		return status.Error(codes.Internal, "missing authentication claims")
	}

	userId := claims.Subject

	s.logger.Info(
		"user subscribed to message stream",
		zap.String("userId", userId),
		zap.String("addr", peerAddress(stream.Context())),
	)

	session := s.broker.Subscribe(userId)
	defer s.broker.Unsubscribe(userId, session)

	// send undelivered messages immediately after subscription
	if err := s.sendUndeliveredMessages(stream, userId); err != nil {
		s.logger.Error(
			"could not send undelivered messages",
			zap.Error(err),
			zap.String("userId", userId),
			zap.String("addr", peerAddress(stream.Context())),
		)

		return status.Error(codes.Internal, "could not load undelivered messages")
	}

	for {
		select {
		case <-session.Done():
			// client is probably slow and broker closes session, force client
			// to reconnect and retrieve messages from db
			s.logger.Info(
				"user session closed by broker",
				zap.String("userId", userId),
				zap.String("addr", peerAddress(stream.Context())),
			)

			return nil

		case message, ok := <-session.GetMessageChan():
			if !ok {
				s.logger.Error(
					"session messages channel was closed (should never occur)",
				)
				return nil
			}

			err := stream.Send(ToIncomingMessageEvent(message))
			if err != nil {
				s.logger.Info(
					"stream send failed",
					zap.Error(err),
					zap.String("userId", userId),
					zap.String("addr", peerAddress(stream.Context())),
				)

				// message was not delivered, so not acked, will pick it up
				// when client reconnects
				return err
			}

		case <-stream.Context().Done():
			s.logger.Info(
				"user unsubscribed from message stream",
				zap.String("userId", userId),
				zap.String("addr", peerAddress(stream.Context())),
			)

			return nil
		}
	}
}

func (s *MessagingService) sendUndeliveredMessages(
	stream grpc.ServerStreamingServer[pb.ServerEvent],
	userId string,
) error {
	messages, err := s.repo.GetUndeliveredMessages(stream.Context(), userId)
	if err != nil {
		return err
	}

	for _, message := range messages {
		err := stream.Send(ToIncomingMessageEvent(message))
		if err != nil {
			return err
		}
	}

	return nil
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
