package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type TokenStore struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// saveTokens writes the auth tokens to a local JSON file
func saveTokens(userId, access, refresh string) error {
	store := TokenStore{
		AccessToken:  access,
		RefreshToken: refresh,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("tokens-%s.json", userId)
	return os.WriteFile(filename, data, 0600)
}

// TokenAuth implements credentials.PerRPCCredentials
type TokenAuth struct {
	accessToken string
	refreshToken string
	// TODO: later, put an auth client here so GetRequestMetadata can auto-refresh
}

// GetRequestMetadata is called by gRPC before EVERY outgoing request (unary and stream)
func (t TokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	// TODO: check if t.token is expired here. If yes, refresh it, update tokens-*.json, update t.token.
	
	return map[string]string{
		"authorization": "Bearer " + t.accessToken,
	}, nil
}

func (t TokenAuth) RequireTransportSecurity() bool {
	return true 
}
