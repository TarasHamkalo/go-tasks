package profiles

import (
	"context"
	"crypto/rsa"
	"errors"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

type ProfileService struct {
	repo Repository

	issuer string

	verificationKey *rsa.PublicKey
	signingKey      *rsa.PrivateKey

	logger *zap.Logger

	activeRefreshTokens map[string]string
	mu                  sync.RWMutex

	pb.UnimplementedProfileServiceServer
}

func NewProfileService(
	repo Repository,
	issuer string,
	verificationKey *rsa.PublicKey,
	signingKey *rsa.PrivateKey,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		repo: repo,

		issuer: issuer,

		verificationKey: verificationKey,
		signingKey:      signingKey,

		logger: logger,

		activeRefreshTokens: make(map[string]string, 10),
	}
}

// TODO: move errors to interceptor
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

	// database generates id
	profile := Profile{
		Username: req.Username,
		Password: hash,
	}

	err = s.repo.InsertProfile(ctx, &profile)
	
	if err != nil {
		if errors.Is(err, ErrorUniqueConstraintViolated) {
			s.logger.Warn(
				"username already exists",
				zap.String("username", req.Username),
				zap.String("addr", peerAddress(ctx)),
			)
			return nil, status.Error(codes.AlreadyExists, "username is already taken")
		}

		s.logger.Error(
			"could not store profile",
			zap.Error(err),
			zap.String("username", req.Username),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Internal, "could not store profile")
	}

	s.logger.Info(
		"profile registered",
		zap.String("userId", profile.UserId),
		zap.String("username", req.Username),
		zap.String("addr", peerAddress(ctx)),
	)

	return &pb.RegisterProfileResponse{
		UserId: profile.UserId,
	}, nil
}

func (s *ProfileService) Login(
	ctx context.Context, req *pb.LoginRequest,
) (*pb.LoginResponse, error) {

	p, err := s.repo.GetProfileByUserId(ctx, req.UserId)
	if err != nil {
		s.logger.Debug(
			"could not retrieve user profile",
			zap.String("userId", req.UserId),
			zap.String("addr", peerAddress(ctx)),
		)

		return nil, status.Error(codes.NotFound, "could not retrieve user profile")
	}

	err = bcrypt.CompareHashAndPassword(p.Password, req.Password)
	if err == nil {
		s.logger.Info(
			"user matched, generating tokens",
			zap.String("userId", req.UserId),
			zap.String("addr", peerAddress(ctx)),
		)

		access, refresh, err := s.buildTokens(p)
		if err != nil {
			s.logger.Error(
				"could not generate tokens",
				zap.String("userId", req.UserId),
				zap.String("addr", peerAddress(ctx)),
			)
			return nil, status.Error(codes.Internal, "could not generate tokens")
		}

		return &pb.LoginResponse{
			Tokens: &pb.Tokens{AccessToken: access, RefreshToken: refresh},
		}, nil
	}

	s.logger.Debug(
		"password was not matched",
		zap.String("userId", req.UserId),
		zap.String("addr", peerAddress(ctx)),
	)
	return nil, status.Error(
		codes.InvalidArgument, "userId or password does not match",
	)
}

func (s *ProfileService) Refresh(
	ctx context.Context, req *pb.RefreshRequest,
) (*pb.RefreshResponse, error) {

	claims, err := auth.ValidateToken(
		req.RefreshToken, s.issuer, s.verificationKey, auth.RefreshTokenType,
	)
	if err != nil {
		s.logger.Debug(
			"invalid refresh token",
			zap.Error(err),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	jti := claims.ID
	userId := claims.Subject

	// check JTI against active list and remove it (one-time use)
	s.mu.Lock()
	expectedUserId, exists := s.activeRefreshTokens[jti]
	if exists && expectedUserId == userId {
		delete(s.activeRefreshTokens, jti)
	}
	s.mu.Unlock()

	if !exists || expectedUserId != userId {
		s.logger.Warn(
			"attempted to use revoked or unknown refresh token",
			zap.String("jti", jti),
			zap.String("userId", userId),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Unauthenticated, "token has been revoked")
	}

	p, err := s.repo.GetProfileByUserId(ctx, userId)
	if err != nil {
		s.logger.Error(
			"could not retrieve user profile during refresh",
			zap.String("userId", userId),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Internal, "could not retrieve user profile")
	}

	access, refresh, err := s.buildTokens(p)
	if err != nil {
		s.logger.Error(
			"could not generate new tokens during refresh",
			zap.String("userId", userId),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Internal, "could not generate tokens")
	}

	s.logger.Info(
		"tokens refreshed successfully",
		zap.String("userId", userId),
		zap.String("addr", peerAddress(ctx)),
	)

	return &pb.RefreshResponse{
		Tokens: &pb.Tokens{AccessToken: access, RefreshToken: refresh},
	}, nil
}

func (s *ProfileService) GetUserProfile(
	ctx context.Context, req *pb.GetUserProfileRequest,
) (*pb.GetUserProfileResponse, error) {
	// we are not interested in which user requesting this information
	_, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Internal,
			"missing authentication claims",
		)
	}

	p, err := s.repo.GetProfileByUserId(ctx, req.UserId)
	if err != nil {
		s.logger.Debug(
			"could not retrieve user profile",
			zap.Error(err),
			zap.String("userId", req.UserId),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.NotFound, "user profile not found")
	}

	return &pb.GetUserProfileResponse{
		UserId:   p.UserId,
		Username: p.Username,
	}, nil
}

func (s *ProfileService) buildTokens(p Profile) (string, string, error) {
	access, refresh, jti, err := auth.BuildTokens(
		p.UserId, s.issuer, s.signingKey,
	)
	if err != nil {
		return "", "", err
	}

	// register the new refresh token in the active list
	s.mu.Lock()
	s.activeRefreshTokens[jti] = p.UserId
	s.mu.Unlock()

	return access, refresh, nil
}

func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
