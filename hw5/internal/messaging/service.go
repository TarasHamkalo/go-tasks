package messaging

import (
	"slices"
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

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

// --- Delivery & Read Tracking ---
func (s *MessagingService) AckMessage(
	ctx context.Context, req *pb.AckMessageRequest,
) (*pb.AckMessageResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	userId := claims.Subject
	deliveredAt := time.Now().UTC()

	err := s.repo.SetMessageDelivered(ctx, req.MsgId, userId, deliveredAt)
	if err != nil {
		s.logger.Error(
			"could not set delivery for message",
			zap.Error(err),
			zap.String("messageId", req.MsgId),
		)
		return nil, status.Error(
			codes.Internal, "could not acknowledge message delivery",
		)
	}

	return &pb.AckMessageResponse{}, nil
}

func (s *MessagingService) SetMessageRead(
	ctx context.Context, req *pb.SetMessageReadRequest,
) (*pb.SetMessageReadResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	userId := claims.Subject
	readAt := time.Now().UTC()

	err := s.repo.SetMessageRead(ctx, req.MsgId, userId, readAt)
	if err != nil {
		s.logger.Error(
			"could not set read status for message",
			zap.Error(err),
			zap.String("messageId", req.MsgId),
		)
		return nil, status.Error(
			codes.Internal, "could not set read status for message",
		)
	}

	return &pb.SetMessageReadResponse{}, nil
}

func (s *MessagingService) GetMessageAcks(
	ctx context.Context, req *pb.GetMessageAcksRequest,
) (*pb.GetMessageAcksResponse, error) {
	// TODO: Check if user is part of the chat the message belongs to.

	acks, err := s.repo.GetMessageAcks(ctx, req.MsgId)
	if err != nil {
		s.logger.Error(
			"could not retrieve message acks",
			zap.Error(err),
			zap.String("messageId", req.MsgId),
		)
		return nil, status.Error(
			codes.Internal, "could not retrieve message acks",
		)
	}

	var pbAcks []*pb.MessageAckInfo
	for _, ack := range acks {
		ackInfo := &pb.MessageAckInfo{
			UserId: ack.UserId,
		}

		if ack.DeliveredAt.Valid {
			ackInfo.DeliveredAt = timestamppb.New(ack.DeliveredAt.Time)
		}

		if ack.ReadAt.Valid {
			ackInfo.ReadAt = timestamppb.New(ack.ReadAt.Time)
		}

		pbAcks = append(pbAcks, ackInfo)
	}

	return &pb.GetMessageAcksResponse{Acks: pbAcks}, nil
}

// --- Chat Management ---
func (s *MessagingService) GetUserChats(
	ctx context.Context, req *pb.GetUserChatsRequest,
) (*pb.GetUserChatsResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	chats, err := s.repo.GetUserChats(ctx, claims.Subject)
	if err != nil {
		s.logger.Error(
			"could not get user chats",
			zap.Error(err),
			zap.String("userId", claims.Subject),
		)
		return nil, status.Error(codes.Internal, "could not retrieve chats")
	}

	var pbChats []*pb.ChatInfo
	for _, chat := range chats {
		pbChats = append(pbChats, &pb.ChatInfo{
			Id:      chat.Id,
			Name:    chat.Name,
			IsGroup: chat.IsGroup,
		})
	}

	return &pb.GetUserChatsResponse{Chats: pbChats}, nil
}

func (s *MessagingService) GetChatMembers(
	ctx context.Context, req *pb.GetChatMembersRequest,
) (*pb.GetChatMembersResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	members, err := s.repo.GetChatMembers(ctx, req.ChatId)
	if err != nil {
		s.logger.Error(
			"could not get chat members",
			zap.Error(err),
			zap.String("chatId", req.ChatId),
		)
		return nil, status.Error(
			codes.Internal, "could not retrieve chat members",
		)
	}

	// Verify the caller is part of the chat
	isMember := slices.Contains(members, claims.Subject)

	if !isMember {
		return nil, status.Error(
			codes.PermissionDenied, "user is not a member of this chat",
		)
	}

	return &pb.GetChatMembersResponse{MemberIds: members}, nil
}

func (s *MessagingService) CreateDirectChat(
	ctx context.Context, req *pb.CreateDirectChatRequest,
) (*pb.CreateDirectChatResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	chatId := uuid.New().String()
	chat := Chat{
		Id:      chatId,
		IsGroup: false,
		Name:    "", // client should resolve user profile 
	}

	// TODO: InsertChat and AddChatMembers = InsertDirectChat (single transaction)
	if err := s.repo.InsertChat(ctx, chat); err != nil {
		s.logger.Error("could not create direct chat", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create chat")
	}

	_ = s.repo.AddChatMember(ctx, chatId, claims.Subject)
	_ = s.repo.AddChatMember(ctx, chatId, req.TargetUserId)

	return &pb.CreateDirectChatResponse{ChatId: chatId}, nil
}

func (s *MessagingService) CreateGroupChat(
	ctx context.Context, req *pb.CreateGroupChatRequest,
) (*pb.CreateGroupChatResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}
	// TODO: name validation?
	chatId := uuid.New().String()
	chat := Chat{
		Id:      chatId,
		IsGroup: true,
		Name:    req.Name,
	}

	// TODO: refactor this to single transaction
	if err := s.repo.InsertChat(ctx, chat); err != nil {
		s.logger.Error("could not create group chat", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create chat")
	}

	// Add the creator
	_ = s.repo.AddChatMember(ctx, chatId, claims.Subject)

	// Add all requested members
	for _, memberId := range req.MemberIds {
		if memberId != claims.Subject { // prevent duplicate insert
			_ = s.repo.AddChatMember(ctx, chatId, memberId)
		}
	}

	return &pb.CreateGroupChatResponse{ChatId: chatId}, nil
}

func (s *MessagingService) AddChatMember(
	ctx context.Context, req *pb.AddChatMemberRequest,
) (*pb.AddChatMemberResponse, error) {
	// TODO: verify that caller is in group 

	err := s.repo.AddChatMember(ctx, req.ChatId, req.TargetUserId)
	if err != nil {
		s.logger.Error(
			"could not add chat member",
			zap.Error(err),
			zap.String("chatId", req.ChatId),
		)
		return nil, status.Error(codes.Internal, "could not add user to chat")
	}

	return &pb.AddChatMemberResponse{}, nil
}

func (s *MessagingService) LeaveChat(
	ctx context.Context, req *pb.LeaveChatRequest,
) (*pb.LeaveChatResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	err := s.repo.RemoveChatMember(ctx, req.ChatId, claims.Subject)
	if err != nil {
		s.logger.Error(
			"could not leave chat",
			zap.Error(err),
			zap.String("chatId", req.ChatId),
		)
		return nil, status.Error(codes.Internal, "could not leave chat")
	}

	return &pb.LeaveChatResponse{}, nil
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

			err := stream.Send(toIncomingMessageEvent(message))
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
		err := stream.Send(toIncomingMessageEvent(&message))
		if err != nil {
			return err
		}
	}

	return nil
}

func toIncomingMessageEvent(message *Message) *pb.ServerEvent {
	incomingMessage := pb.IncomingMessage{
		Id:       message.Id,
		SenderId: message.SenderId,
		ChatId:   message.ChatId,
		Content:  message.Content,
		SentAt:   timestamppb.New(message.SentAt),
	}

	// pretty nice syntax :)
	return &pb.ServerEvent{
		Event: &pb.ServerEvent_IncomingMessage{
			IncomingMessage: &incomingMessage,
		},
	}
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
