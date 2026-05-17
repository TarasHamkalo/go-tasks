package auth

import (
	"crypto/rsa"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)


var ErrorInvalidToken = errors.New("invalid token")

type TokenType string
const (
	AccessTokenType  TokenType = "access"
	RefreshTokenType TokenType = "refresh"
)

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
