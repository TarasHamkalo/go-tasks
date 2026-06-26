package auth

import "github.com/golang-jwt/jwt/v5"

// MessengerClaims custom claims to distinguish between access token and 
// refresh token. 
//
// NOTE not really sure that this is correct, probably enough to use JTI for
// separation (at least later discovered =))
type MessengerClaims struct {
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}
