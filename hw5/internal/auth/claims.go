package auth

import "github.com/golang-jwt/jwt/v5"

type MessengerClaims struct {
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}
