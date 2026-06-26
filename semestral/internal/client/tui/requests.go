// Package tui provides handles terminal client implementation
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

// ResolveDirectChatToSession featchs companion profile info from ProfileService
// and stores in session
func ResolveDirectChatToSession(
	appContext *state.AppContext, chatId string,
) error {
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

	profile, err := ResolveProfileToSession(appContext, companionId)
	if err != nil {
		// populate a placeholder profile so rendering doesn't crash
		profile = &state.Profile{
			Id:       companionId,
			Username: fmt.Sprintf("User %s", companionId),
		}

		session.SetProfile(profile)
	}

	session.InsertChat(&state.DirectChat{
		ChatId:       chatId,
		OtherProfile: profile,
	})

	return nil
}

// ResolveProfileToSession loads a profile from the server
// and stores it in the local session cache.
func ResolveProfileToSession(
	appContext *state.AppContext, profileId string,
) (*state.Profile, error) {
	session := appContext.Session
	profCtx, cancelProf := context.WithTimeout(
		appContext.Ctx,
		5*time.Second,
	)

	defer cancelProf()

	profRes, err := appContext.ProfileClient.GetUserProfile(
		profCtx,
		&pb.GetUserProfileRequest{
			UserId: profileId,
		},
	)

	if err != nil {
		return nil, err

	}
	status := "offline"
	if profRes.Status == *pb.UserStatus_ONLINE.Enum() {
		status = "online"
	}

	profile := &state.Profile{
		Id:       profRes.UserId,
		Username: profRes.Username,
		Bio:      profRes.Bio,
		Status:   status,
	}

	session.SetProfile(profile)
	return profile, nil

}

// CleanGrpcError returns only the gRPC status message.
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
