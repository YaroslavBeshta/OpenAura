package useridentity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/openaura/openaura/internal/user"
)

const ProviderPassword = "password"

type Identity struct {
	ID              uuid.UUID       `json:"id"`
	AppID           uuid.UUID       `json:"app_id"`
	UserID          uuid.UUID       `json:"user_id"`
	Provider        string          `json:"provider"`
	ProviderSubject string          `json:"provider_subject"`
	SecretHash      string          `json:"-"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	UserID          uuid.UUID
	Provider        string
	ProviderSubject string
	SecretHash      string // required for password; empty for SSO
	Metadata        json.RawMessage
}

// PasswordCredential is a user joined with their password identity for login.
type PasswordCredential struct {
	User       user.User
	SecretHash string
}
