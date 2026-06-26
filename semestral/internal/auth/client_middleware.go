package auth

import (
	"context"
	"crypto/rsa"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gomessenger/generated"
)

// ErrSessionExpired error indicating that session is expired
// and tokens can not be refreshed without user interaction
var ErrSessionExpired = status.Error(codes.Unauthenticated, "session_expired")

// TokenCredentialsInterecptor implements grpc.WithPerRPCCredentials
// and supplies requests with authorization header (bearer token)
type TokenCredentialsInterecptor struct {
	// mu used to sync access of possibly multiple routines to this same interceptor
	// 
	// NOTE yep could be just embedded
	mu sync.RWMutex

	// userId id of user for which tokens are granted
	userId string

	// accessToken, refreshToken token strings "as are"
	accessToken    string
	refreshToken   string

	// accessTokenExp parsed ones when tokens are set or refreshed and 
	// verified per request
	accessTokenExp time.Time

	// issuer is token issuer 
	issuer          string

	// verificationKey public key of the pair used to sign tokens
	verificationKey *rsa.PublicKey

	// profileClient grpc client to refresh tokens transperently 
	//
	// NOTE separate client should be used, which does not have this interceptor set,
	// in other case deadlock
	profileClient pb.ProfileServiceClient

	logger *zap.Logger
}

// NewTokenCredentialsInterecptor create auth interceptor with a map of public endpoints
func NewTokenCredentialsInterecptor(
	issuer string,
	verificationKey *rsa.PublicKey,
	logger *zap.Logger,
) *TokenCredentialsInterecptor {
	return &TokenCredentialsInterecptor{
		issuer:          issuer,
		verificationKey: verificationKey,

		logger: logger,
	}
}

// SetTokens parses, validates, and stores expiration time of incoming token
func (t *TokenCredentialsInterecptor) SetTokens(
	userId, access, refresh string,
) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	claims, err := ValidateToken(
		access, t.issuer, t.verificationKey, AccessTokenType,
	)
	if err != nil {
		return err
	}

	t.userId = userId
	t.accessToken = access
	t.refreshToken = refresh
	t.accessTokenExp = claims.ExpiresAt.Time

	return nil
}

func (t *TokenCredentialsInterecptor) SetProfileClient(
	client pb.ProfileServiceClient,
) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.profileClient = client
}

// GetRequestMetadata method which is called per each request and which
// handles tokens header insertion. 
// 1. under read lock gets token, if token is not expired - authenticates request 
// 2. if token is expired, acquires write lock, verifies whether in meantime
// other routine did not refresh tokens and tries to refresh.
// On success authenticates request, else ErrSessionExpired thrown
func (t *TokenCredentialsInterecptor) GetRequestMetadata(
	ctx context.Context, uri ...string,
) (map[string]string, error) {
	// acquire Read Lock to check expiration
	t.mu.RLock()
	isExpired := time.Now().After(t.accessTokenExp)
	access := t.accessToken
	t.mu.RUnlock()

	// no tokens, send unauthenticated request
	// and possibly get error from server
	if access == "" {
		return nil, nil
	}

	if !isExpired {
		return map[string]string{"authorization": "Bearer " + access}, nil
	}

	// token is expired, refresh under write lock
	t.logger.Debug("token expired, acquiring tokens lock")
	t.mu.Lock()
	defer t.mu.Unlock()

	t.logger.Debug("tokens lock acquired")
	// maybe other go routine already have updated it, verify again
	if t.accessToken != "" && time.Now().Before(t.accessTokenExp) {
		t.logger.Info("tokens were already refreshed")
		return map[string]string{
			"authorization": "Bearer " + t.accessToken,
		}, nil
	}

	// cannot refresh
	if t.profileClient == nil ||
		t.refreshToken == "" ||
		t.userId == "" {
		t.clearLocked()
		t.logger.Debug("could not refresh tokens")
		return nil, ErrSessionExpired
	}

	// call gRPC Refresh
	t.logger.Debug("grpc tokens refresh call")
	refreshCtx, cancel := context.WithTimeout(
		context.Background(), time.Second*5,
	)
	defer cancel()

	res, err := t.profileClient.Refresh(refreshCtx, &pb.RefreshRequest{
		RefreshToken: t.refreshToken,
	})

	if err != nil {
		t.clearLocked()
		t.logger.Debug("could not refresh tokens")
		return nil, ErrSessionExpired
	}

	claims, err := ValidateToken(
		res.Tokens.AccessToken, t.issuer, t.verificationKey, AccessTokenType,
	)

	if err != nil {
		t.clearLocked()
		return nil, ErrSessionExpired
	}

	t.accessToken = res.Tokens.AccessToken
	t.refreshToken = res.Tokens.RefreshToken
	t.accessTokenExp = claims.ExpiresAt.Time

	t.logger.Debug("tokens were refreshed")
	return map[string]string{
		"authorization": "Bearer " + t.accessToken,
	}, nil
}

func (t *TokenCredentialsInterecptor) RequireTransportSecurity() bool {
	return true
}

// clearLocked clears session fields. Caller must hold t.mu.Lock().
func (t *TokenCredentialsInterecptor) clearLocked() {
	t.userId = ""
	t.accessToken = ""
	t.refreshToken = ""
	t.accessTokenExp = time.Time{}
}


// Clear clears session fields
func (t *TokenCredentialsInterecptor) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.userId = ""
	t.accessToken = ""
	t.refreshToken = ""
	t.accessTokenExp = time.Time{}
}

