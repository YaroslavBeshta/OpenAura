package userauth

import (
	"encoding/json"

	"github.com/openaura/openaura/internal/user"
)

type RegisterInput struct {
	Email    string          `json:"email" example:"ada@example.com"`
	Password string          `json:"password" example:"correct-horse-battery-staple"`
	Metadata json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type LoginInput struct {
	Email    string `json:"email" example:"ada@example.com"`
	Password string `json:"password" example:"correct-horse-battery-staple"`
}

type TokenResponse struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type" example:"Bearer"`
	ExpiresIn   int64     `json:"expires_in" example:"86400"`
	User        user.User `json:"user"`
}
