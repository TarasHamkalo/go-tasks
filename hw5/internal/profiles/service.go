package profiles

import (
	"context"
	pb "gomessenger/generated"

	"go.uber.org/zap"
)

type ProfileService struct {
	profileRepository Repository

	logger *zap.Logger;

	pb.UnimplementedProfileServiceServer
}

func NewProfileService(
	profileRepository Repository, logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		profileRepository: profileRepository,
		logger: logger,
	}
}
// type ProfileServiceServer interface {
// 	RegisterProfile(context.Context, *RegisterProfileRequest) (*RegisterProfileResponse, error)
// 	Login(context.Context, *LoginRequest) (*LoginResponse, error)
// 	Refresh(context.Context, *RefreshRequest) (*RefreshResponse, error)
// 	GetUserProfile(context.Context, *GetUserProfileRequest) (*GetUserProfileResponse, error)
// 	mustEmbedUnimplementedProfileServiceServer()
// }

func RegisterProfile(
	ctx context.Context, 
	req *pb.RegisterProfileRequest,
) (*pb.RegisterProfileResponse, error) {
	
	return nil, nil;	
}
