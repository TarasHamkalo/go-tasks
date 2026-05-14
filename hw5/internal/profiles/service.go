package profiles

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"math/big"
	"strconv"
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

	signingKey *rsa.PrivateKey

	logger *zap.Logger

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
	}
}

// type ProfileServiceServer interface {
// 	RegisterProfile(context.Context, *RegisterProfileRequest) (*RegisterProfileResponse, error)
// 	Login(context.Context, *LoginRequest) (*LoginResponse, error)
// 	Refresh(context.Context, *RefreshRequest) (*RefreshResponse, error)
// 	GetUserProfile(context.Context, *GetUserProfileRequest) (*GetUserProfileResponse, error)
// 	mustEmbedUnimplementedProfileServiceServer()
// }

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

	err = bcrypt.CompareHashAndPassword(p.Password, req.Password);
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

// TODO: store active list of jti
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
