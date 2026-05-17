package tui

import (
	"context"
	"fmt"
	pb "gomessenger/generated"
	"gomessenger/internal/client/state"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fetch companion profile info from ProfileService
func ResolveDirectChat(appContext *state.AppContext, chatId string) error {
	session := appContext.Session
	memCtx, cancelMem := context.WithTimeout(
		appContext.Ctx,
		5*time.Second,
	)
	defer cancelMem()

	membersResp, err := appContext.MessagingClient.GetChatMembers(
		memCtx, &pb.GetChatMembersRequest{ChatId: chatId},
	)

	if err != nil {
		return CleanGrpcError(err)
	}

	session.SetChatMembers(chatId, membersResp.MemberIds)
	userId := session.GetUserId()

	var companionId string
	for _, uid := range membersResp.MemberIds {
		if uid != userId {
			companionId = uid
			break
		}
	}

	if companionId == "" {
		return nil
	}

	profCtx, cancelProf := context.WithTimeout(
		appContext.Ctx,
		5*time.Second,
	)

	defer cancelProf()

	profRes, err := appContext.ProfileClient.GetUserProfile(
		profCtx,
		&pb.GetUserProfileRequest{
			UserId: companionId,
		},
	)

	if err != nil {
		// populate a placeholder profile so rendering doesn't crash
		placeholder := &state.Profile{
			Id:       companionId,
			Username: fmt.Sprintf("User %s", companionId),
		}

		session.SetProfile(placeholder)
		session.InsertChat(&state.DirectChat{
			ChatId:       chatId,
			OtherProfile: placeholder,
		})
	}

	profile := &state.Profile{
		Id:       profRes.UserId,
		Username: profRes.Username,
	}

	session.SetProfile(profile)
	session.InsertChat(&state.DirectChat{
		ChatId:       chatId,
		OtherProfile: profile,
	})

	return nil
}

func CleanGrpcError(err error) error {
	if s, ok := status.FromError(err); ok {
		return fmt.Errorf("%s", s.Message())
	}
	return err
}

// CalculateBackoff provides exponential backoff: 1s, 2s, 4s, 8s, 16s... capped at 30s.
func CalculateBackoff(retryCount int) time.Duration {
	delay := time.Duration(1<<retryCount) * time.Second
	if delay > 30*time.Second {
		return 30 * time.Second
	}
	return delay
}


// Helper to strip out standard gRPC client failures that should never trigger automated retries
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return true
	}
	switch st.Code() {
	case codes.InvalidArgument,
		codes.Unauthenticated,
		codes.PermissionDenied,
		codes.NotFound,
		codes.AlreadyExists,
		codes.FailedPrecondition,
		codes.Unimplemented:
		// Client errors / permanent structure failures cannot be fixed by trying again
		return false
	default:
		return true
	}
}
