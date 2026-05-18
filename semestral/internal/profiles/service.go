// Package profiles implements the profile service layer, including
// user registration, authentication, JWT token management,
// and profile CRUD operations backed by a repository.
package profiles

import (
	"context"
	"crypto/rsa"
	"errors"
	"strings"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
	"gomessenger/internal/auth"
)

// ProfileService implements user registration, authentication,
// token refresh, and profile management operations.
type ProfileService struct {
	// Persistent storage backend for profiles.
	repo Repository

	// JWT issuer claim used when generating and validating tokens.
	issuer string

	// Public key used to verify JWT signatures.
	verificationKey *rsa.PublicKey

	// Private key used to sign JWT tokens.
	signingKey *rsa.PrivateKey

	// Structured application logger.
	logger *zap.Logger

	// Maps refresh token JTI to owning user Id.
	// Used to enforce one-time refresh token usage.
	// 
	// NOTE ideally will be stored on separate key value db were will not be 
	// affected by chosen server (LB) or restart. Leaving as is for demo 
	// 
	// NOTE should be accessed under mutex below
	activeRefreshTokens map[string]string
	mu sync.RWMutex

	pb.UnimplementedProfileServiceServer
}

// NewProfileService construct new ProfileService object instance
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

// RegisterProfile registers user profile provided in request.
// The password is hashed using bcrypt and the UserId is generated
// by the repository layer.
// 
// NOTE tokens are not generated on success, request separately.
//
// NOTE not really sure that so much logging should be present. Maybe should
// be refactored to interceptor or so.
func (s *ProfileService) RegisterProfile(
	ctx context.Context,
	req *pb.RegisterProfileRequest,
) (*pb.RegisterProfileResponse, error) {
	trimmedUsername := strings.TrimSpace(req.Username)
	if len(trimmedUsername) < 3 || len(trimmedUsername) > 32 {
		return nil, status.Error(
			codes.InvalidArgument,
			"username has to have between 3 to 32 non empty characters",
		)
	}

	if len(req.Password) < 8 || len(req.Password) > 72 {
		return nil, status.Error(
			codes.InvalidArgument, "password has to have between 8 and 72 chars",
		)
	}

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
		Username: trimmedUsername,
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

// Login verify user credential (for password bcrypt is used).
// On success return pair of tokens
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
		codes.Unauthenticated, "userId or password does not match",
	)
}

// Refresh creates new pair of tokens for given user. 
//
// NOTE User is extracted from refresh token claims.
// 
// NOTE Refresh tokens are of one use, JTI used to track active refresh tokens.
// At the moment active JTIs are stored in memory so server restart requires 
// user to login again.
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
	_, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
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

	statusEnum := pb.UserStatus_OFFLINE
	if p.Status == "online" {
		statusEnum = pb.UserStatus_ONLINE
	}

	return &pb.GetUserProfileResponse{
		UserId:   p.UserId,
		Username: p.Username,
		Bio:      p.Bio,
		Status:   statusEnum,
	}, nil
}

func (s *ProfileService) UpdateStatus(
	ctx context.Context, req *pb.UpdateStatusRequest,
) (*pb.UpdateStatusResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	userId := claims.Subject

	statusStr := "offline"
	if req.Status == pb.UserStatus_ONLINE {
		statusStr = "online"
	}
	
	err := s.repo.UpdateStatus(ctx, userId, statusStr)
	if err != nil {
		s.logger.Error(
			"could not update user status",
			zap.Error(err),
			zap.String("userId", userId),
		)
		return nil, status.Error(codes.Internal, "could not update user status")
	}

	return &pb.UpdateStatusResponse{}, nil
}

func (s *ProfileService) UpdateUserProfile(
	ctx context.Context, req *pb.UpdateUserProfileRequest,
) (*pb.UpdateUserProfileResponse, error) {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	// Username Validation
	trimmedUsername := strings.TrimSpace(req.Username)
	if len(trimmedUsername) < 3 || len(trimmedUsername) > 32 {
		return nil, status.Error(
			codes.InvalidArgument,
			"username has to have between 3 to 32 non empty characters",
		)
	}

	err := s.repo.UpdateProfile(ctx, claims.Subject, trimmedUsername, req.Bio)
	if err != nil {
		if errors.Is(err, ErrorUniqueConstraintViolated) {
			return nil, status.Error(codes.AlreadyExists, "username is already taken")
		}
		s.logger.Error(
			"could not update user profile",
			zap.Error(err),
			zap.String("userId", claims.Subject),
		)
		return nil, status.Error(codes.Internal, "could not update user profile")
	}

	s.logger.Info("user profile updated", zap.String("userId", claims.Subject))

	return &pb.UpdateUserProfileResponse{}, nil
}

func (s *ProfileService) VerifyUsers(
	ctx context.Context, req *pb.VerifyUsersRequest,
) (*pb.VerifyUsersResponse, error) {
	_, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(
			codes.Unauthenticated, "missing authentication claims",
		)
	}

	uniqueIds := make(map[string]bool)
	var cleanIds []string
	for _, id := range req.UserIds {
		if !uniqueIds[id] && id != "" {
			uniqueIds[id] = true
			cleanIds = append(cleanIds, id)
		}
	}

	existing, err := s.repo.CheckUsersExist(ctx, cleanIds)
	if err != nil {
		s.logger.Error("could not verify user Ids", zap.Error(err))
		return nil, status.Error(codes.Internal, "failed to verify users")
	}

	return &pb.VerifyUsersResponse{
		ExistingUserIds: existing,
	}, nil
}

// buildTokens builds pair of tokens for given user and stores
// refresh token JTI
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

// peerAddress helper used to pars ip address from context
func peerAddress(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}

	return p.Addr.String()
}
