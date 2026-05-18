package messaging

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

type MessagingService struct {
	repo Repository

	broker *Broker

	profileClient pb.ProfileServiceClient

	logger *zap.Logger

	pb.UnimplementedMessagingServiceServer
}

func NewMessagingService(
	repo Repository,
	profileClient pb.ProfileServiceClient,
	logger *zap.Logger,
) *MessagingService {
	broker := NewBroker(logger.With(zap.String("module", "broker")))
	return &MessagingService{
		repo:          repo,
		broker:        broker,
		profileClient: profileClient,
		logger:        logger,
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

	err := s.repo.AcknowledgeAndCleanupMessage(
		ctx, req.MsgId, userId, deliveredAt,
	)
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

func (s *MessagingService) SetMessagesRead(
	ctx context.Context, req *pb.SetMessagesReadRequest,
) (*pb.SetMessagesReadResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	if len(req.MsgIds) == 0 {
		return &pb.SetMessagesReadResponse{}, nil
	}

	userId := claims.Subject
	readAt := time.Now().UTC()

	err := s.repo.SetMessagesRead(ctx, req.MsgIds, userId, readAt)
	if err != nil {
		s.logger.Error(
			"could not set batch read status for messages",
			zap.Error(err),
			zap.Int("messageCount", len(req.MsgIds)),
		)
		return nil, status.Error(
			codes.Internal, "could not set read status for messages",
		)
	}

	return &pb.SetMessagesReadResponse{}, nil
}

func (s *MessagingService) GetMessageAcks(
	ctx context.Context, req *pb.GetMessageAcksRequest,
) (*pb.GetMessageAcksResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	acks, err := s.repo.GetMessageAcks(ctx, req.MsgId)
	if err != nil {
		s.logger.Error(
			"could not retrieve message acks",
			zap.Error(err),
			zap.String("messageId", req.MsgId),
		)
		return nil, status.Error(codes.Internal, "could not retrieve message acks")
	}

	if len(acks) == 0 {
		// e.g. empty chat
		return &pb.GetMessageAcksResponse{Acks: []*pb.MessageAckInfo{}}, nil
	}

	// extract chat id from the first ack record to verify permissions
	targetChatId := acks[0].ChatId

	members, err := s.repo.GetChatMembers(ctx, targetChatId)
	if err != nil {
		return nil, status.Error(
			codes.Internal, "could not retrieve chat members for verification",
		)
	}

	if !slices.Contains(members, claims.Subject) {
		return nil, status.Error(
			codes.PermissionDenied,
			"user cannot query acks for a chat they do not belong to",
		)
	}

	// map to protobuf
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

	// verify the caller is part of the chat
	if !slices.Contains(members, claims.Subject) {
		return nil, status.Error(codes.PermissionDenied, "user is not a member of this chat")
	}

	return &pb.GetChatMembersResponse{MemberIds: members}, nil
}

func (s *MessagingService) CreateDirectChat(
	ctx context.Context, req *pb.CreateDirectChatRequest,
) (*pb.CreateDirectChatResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing authentication claims")
	}

	userId := claims.Subject
	targetId := req.TargetUserId

	// cannot create a direct chat with yourself
	if userId == targetId {
		return nil, status.Error(
			codes.InvalidArgument, "cannot create a direct chat with yourself",
		)
	}

	// should be present, already verified above
	md, _ := metadata.FromIncomingContext(ctx)
	outboundCtx := metadata.NewOutgoingContext(ctx, md)

	verifyResp, err := s.profileClient.VerifyUsers(
		outboundCtx,
		&pb.VerifyUsersRequest{
			UserIds: []string{targetId},
		},
	)

	if err != nil {
		s.logger.Error("failed call to verify users", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not verify chat members")
	}

	if len(verifyResp.ExistingUserIds) != 1 {
		return nil, status.Error(
			codes.InvalidArgument, "target user profile does not exist",
		)
	}

	// validate whether direct chat already exists
	// TODO: ideally should be under transaction.... (read-check-write)
	chat, err := s.repo.GetDirectChatByUsers(ctx, userId, targetId)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.logger.Error("could not create direct chat", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create chat")
	}

	if err == nil && len(chat.Id) != 0 {
		s.logger.Debug("user attempted to create existing chat")
		return &pb.CreateDirectChatResponse{ChatId: chat.Id}, nil
	}

	chatId := uuid.New().String()
	chat = Chat{
		Id:      chatId,
		IsGroup: false,
		Name:    "", // client should resolve user profile
	}

	err = s.repo.InsertChat(ctx, chat, []string{userId, targetId})
	if err != nil {
		s.logger.Error("could not create direct chat", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create chat")
	}

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
	// ensure valid group name
	groupName := strings.TrimSpace(req.Name)
	if len(groupName) < 1 || len(groupName) > 100 {
		return nil, status.Error(
			codes.InvalidArgument, "group name must be between 1 and 100 characters",
		)
	}

	// Use a map to filter out duplicate member IDs provided by the user
	seen := make(map[string]bool)
	seen[claims.Subject] = true // ensure creator isn't duplicated

	memberIds := make([]string, 0, len(req.MemberIds)+1)
	memberIds = append(memberIds, claims.Subject)

	for _, mId := range req.MemberIds {
		trimmedId := strings.TrimSpace(mId)
		if trimmedId != "" && !seen[trimmedId] {
			seen[trimmedId] = true
			memberIds = append(memberIds, trimmedId)
		}
	}

	// forward incoming authorization metadata to the outbound context
	md, _ := metadata.FromIncomingContext(ctx)
	outboundCtx := metadata.NewOutgoingContext(ctx, md)

	// cross-service validation: batch verify all clean member IDs
	verifyResp, err := s.profileClient.VerifyUsers(
		outboundCtx,
		&pb.VerifyUsersRequest{
			UserIds: memberIds,
		},
	)
	if err != nil {
		s.logger.Error("failed call to verify users", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not verify chat members")
	}

	if len(verifyResp.ExistingUserIds) != len(memberIds) {
		return nil, status.Error(
			codes.InvalidArgument, "one or more provided user Ids do not exist",
		)
	}

	chatId := uuid.New().String()
	chat := Chat{
		Id:      chatId,
		IsGroup: true,
		Name:    groupName,
	}

	err = s.repo.InsertChat(ctx, chat, memberIds)
	if err != nil {
		s.logger.Error("could not create group chat", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create chat")
	}

	return &pb.CreateGroupChatResponse{ChatId: chatId}, nil
}

func (s *MessagingService) AddChatMember(
	ctx context.Context, req *pb.AddChatMemberRequest,
) (*pb.AddChatMemberResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	targetUserId := strings.TrimSpace(req.TargetUserId)
	if targetUserId == "" {
		return nil, status.Error(
			codes.InvalidArgument, "target user id cannot be empty",
		)
	}

	members, err := s.repo.GetChatMembers(ctx, req.ChatId)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not verify chat metadata")
	}

	// caller must be inside the group to invite someone else
	if !slices.Contains(members, claims.Subject) {
		return nil, status.Error(
			codes.PermissionDenied, "user is not part of the target chat",
		)
	}

	// target user is already a member
	if slices.Contains(members, targetUserId) {
		return nil, status.Error(
			codes.AlreadyExists, "target user is already a member of this chat",
		)
	}

	chat, err := s.repo.GetChatById(ctx, req.ChatId)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not verify chat metadata")
	}

	if !chat.IsGroup {
		return nil, status.Error(
			codes.InvalidArgument, "cannot add members to a direct chat",
		)
	}

	// forward incoming authorization metadata to the outbound context
	md, _ := metadata.FromIncomingContext(ctx)
	outboundCtx := metadata.NewOutgoingContext(ctx, md)

	// verify the target user profile actually exists
	verifyResp, err := s.profileClient.VerifyUsers(
		outboundCtx,
		&pb.VerifyUsersRequest{
			UserIds: []string{targetUserId},
		},
	)
	if err != nil {
		s.logger.Error("failed call to verify target user", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not verify target user presence")
	}

	if len(verifyResp.ExistingUserIds) != 1 {
		return nil, status.Error(codes.NotFound, "target user profile does not exist")
	}

	err = s.repo.AddChatMember(ctx, req.ChatId, targetUserId)
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

func (s *MessagingService) GetChatById(
	ctx context.Context,
	req *pb.GetChatByIdRequest,
) (*pb.GetChatByIdResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	chat, err := s.repo.GetChatById(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "chat %q not found", req.Id)
	}

	members, err := s.repo.GetChatMembers(ctx, req.Id)
	if err != nil {
		return nil, status.Error(
			codes.Internal,
			"failed to load chat members",
		)
	}

	if !slices.Contains(members, claims.Subject) {
		return nil, status.Error(
			codes.PermissionDenied,
			"user is not a member of this chat",
		)
	}

	return &pb.GetChatByIdResponse{
		Chat: &pb.ChatInfo{
			Id:      chat.Id,
			Name:    chat.Name,
			IsGroup: chat.IsGroup,
		},
	}, nil
}

func (s *MessagingService) LeaveChat(
	ctx context.Context, req *pb.LeaveChatRequest,
) (*pb.LeaveChatResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing authentication claims")
	}

	chat, err := s.repo.GetChatById(ctx, req.ChatId)
	if err != nil {
		s.logger.Error(
			"could not leave chat",
			zap.Error(err),
			zap.String("chatId", req.ChatId),
		)
		return nil, status.Error(codes.Internal, "could not leave chat")
	}

	if !chat.IsGroup {
		return nil, status.Error(
			codes.InvalidArgument, "can not leave direct chat",
		)
	}

	err = s.repo.RemoveChatMember(ctx, req.ChatId, claims.Subject)
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
	// if chat exists then message is valid, no need to verify recepients
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
			ChatId:    message.ChatId,
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

	// extract authorization metadata to pass to the Profile service
	// will be surely present, see above check
	md, _ := metadata.FromIncomingContext(stream.Context())

	userId := claims.Subject

	s.logger.Info(
		"user subscribed to message stream",
		zap.String("userId", userId),
		zap.String("addr", peerAddress(stream.Context())),
		zap.Bool("isInvisible", req.IsInvisible),
	)

	if !req.IsInvisible {
		outCtx, cancel := context.WithTimeout(
			metadata.NewOutgoingContext(stream.Context(), md), 5*time.Second,
		)

		_, err := s.profileClient.UpdateStatus(outCtx, &pb.UpdateStatusRequest{
			Status: pb.UserStatus_ONLINE,
		})
		cancel()

		if err != nil {
			s.logger.Warn(
				"failed to set user online status",
				zap.Error(err),
				zap.String("userId", userId),
			)
		}
	}
	defer func() {
		// when the client disconnects, stream.Context() is canceled.
		outCtx, cancel := context.WithTimeout(
			metadata.NewOutgoingContext(context.Background(), md), 5*time.Second,
		)
		defer cancel()

		_, err := s.profileClient.UpdateStatus(outCtx, &pb.UpdateStatusRequest{
			Status: pb.UserStatus_OFFLINE,
		})

		if err != nil {
			s.logger.Warn(
				"failed to set user offline status",
				zap.Error(err),
				zap.String("userId", userId),
			)
		}
	}()

	// TODO: change status of user here and set defer
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
