package profiles

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strconv"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/peer"
	"go.uber.org/zap"

	pb "gomessenger/generated"
)

type ProfileService struct {
	repo Repository

	logger *zap.Logger

	pb.UnimplementedProfileServiceServer
}

func NewProfileService(repo Repository, logger *zap.Logger) *ProfileService {
	return &ProfileService{
		repo:   repo,
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

func (s *ProfileService) RegisterProfile(
	ctx context.Context,
	req *pb.RegisterProfileRequest,
) (*pb.RegisterProfileResponse, error) {
	hash, err := bcrypt.GenerateFromPassword(req.Password, bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error(
			"could not create password hash",
			zap.Error(err),
			zap.String("addr", peerAddress(ctx)),
		)

		return nil, status.Error(codes.Internal, "could not create password hash")
	}

	// a few attempts generating unique random number
	for range 10 {
		userId, err := generateUserID()
		if err != nil {
			s.logger.Error(
				"could not generate user ID",
				zap.Error(err),
				zap.String("addr", peerAddress(ctx)),
			)

			return nil, status.Error(codes.Internal, "could not generate user ID")
		}

		profile := Profile{
			UserId:   userId,
			Username: req.Username,
			Password: hash,
		}

		err = s.repo.InsertProfile(ctx, profile)
		if err == nil {
			s.logger.Info(
				"profile registered",
				zap.String("userId", userId),
				zap.String("username", req.Username),
				zap.String("addr", peerAddress(ctx)),
			)

			return &pb.RegisterProfileResponse{
				UserId: userId,
			}, nil
		}

		if errors.Is(err, ErrorUniqueConstraintViolated) {
			continue
		}

		s.logger.Error(
			"could not store profile",
			zap.Error(err),
			zap.String("userId", userId),
			zap.String("username", req.Username),
			zap.String("addr", peerAddress(ctx)),
		)

		return nil, status.Error(codes.Internal, "could not store profile")
	}

	s.logger.Error(
		"could not generate unique user ID",
		zap.String("username", req.Username),
		zap.String("addr", peerAddress(ctx)),
	)

	return nil, status.Error(codes.Internal, "could not generate unique user ID")
}

func generateUserID() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(900000000))
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(n.Int64()+100000000, 10), nil
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
