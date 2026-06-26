// Auth package provides utils to track user identity. It is
// JWT tokens creation, verification and middleware/interceptors to simplify
// usage of authentication in services/clients.
package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ErrorInvalidToken thrown when validated token is invalid
var ErrorInvalidToken = errors.New("invalid token")

// TokenType used to distinguish between access and refresh tokens
// see auth.MessengerClaims
type TokenType string

const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

// BuildTokens constructs access, refresh tokens pair with given issuer
// and signature by signingKey and S256.
// Rerfresh token is valid for 7d and access for 1h.
func BuildTokens(
	userId string,
	issuer string,
	signingKey *rsa.PrivateKey,
) (string, string, string, error) {
	now := time.Now()

	accessClaims := MessengerClaims{
		TokenType: string(AccessTokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId,
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodRS256, accessClaims,
	).SignedString(signingKey)
	if err != nil {
		return "", "", "", err
	}

	jti := uuid.New().String()

	refreshClaims := MessengerClaims{
		TokenType: string(RefreshTokenType),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userId,
			Issuer:    issuer,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * 7 * time.Hour)),
		},
	}

	refreshToken, err := jwt.NewWithClaims(
		jwt.SigningMethodRS256, refreshClaims,
	).SignedString(signingKey)
	if err != nil {
		return "", "", "", err
	}

	return accessToken, refreshToken, jti, nil
}

// ValidateToken, includes validation of issuer and token type claims.
// Returns parsed claims or ErrorInvalidToken
func ValidateToken(
	tokenString string,
	issuer string,
	verificationKey *rsa.PublicKey,
	tokenType TokenType,
) (*MessengerClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&MessengerClaims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return verificationKey, nil
		},
	)

	if err != nil || !token.Valid {
		return nil, ErrorInvalidToken
	}

	claims, ok := token.Claims.(*MessengerClaims)
	if !ok ||
		claims.TokenType != string(tokenType) ||
		claims.Issuer != issuer {
		return nil, ErrorInvalidToken
	}

	return claims, nil
}
