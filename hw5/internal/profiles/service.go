package profiles

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"math/big"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	pb "gomessenger/generated"
)

type MessengerClaims struct {
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}

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
	verificationKey *rsa.PublicKey,
	signingKey *rsa.PrivateKey,
	logger *zap.Logger,
) *ProfileService {
	return &ProfileService{
		repo: repo,

		issuer: "hamkatar-gommessenger",

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

	// a few attempts generating unique random number,
	// should have just used incremental sequence
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

			// TODO: Automatically log the user in and return tokens on registration if desired,
			// or just return the UserID as per the current proto structure.
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

	token, err := jwt.ParseWithClaims(
		req.RefreshToken,
		&MessengerClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return s.verificationKey, nil
		},
	)

	if err != nil || !token.Valid {
		s.logger.Debug(
			"invalid refresh token signature or expired",
			zap.Error(err),
			zap.String("addr", peerAddress(ctx)),
		)
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	claims, ok := token.Claims.(*MessengerClaims)
	if !ok || claims.TokenType != "refresh" || claims.Issuer != s.issuer {
		s.logger.Debug(
			"invalid refresh token claims",
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

	// fetch user profile 
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

	// TODO: request has to be authenticated
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
	now := time.Now()

	accessClaims := MessengerClaims{
		TokenType: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserId,
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodRS256, accessClaims,
	).SignedString(s.signingKey)

	if err != nil {
		return "", "", err
	}

	jti := uuid.New().String()

	refreshClaims := MessengerClaims{
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   p.UserId,
			Issuer:    s.issuer,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * 7 * time.Hour)), // 7 days
		},
	}

	refreshToken, err := jwt.NewWithClaims(
		jwt.SigningMethodRS256, refreshClaims,
	).SignedString(s.signingKey)
	
	if err != nil {
		return "", "", err
	}

	// register the new refresh token in the active list
	s.mu.Lock()
	s.activeRefreshTokens[jti] = p.UserId
	s.mu.Unlock()

	return accessToken, refreshToken, nil
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
