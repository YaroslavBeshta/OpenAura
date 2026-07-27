package apikey

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type APIKey struct {
	ID        uuid.UUID       `json:"id"`
	AppID     uuid.UUID       `json:"app_id"`
	Name      *string         `json:"name,omitempty"`
	Metadata  json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	RevokedAt *time.Time      `json:"revoked_at,omitempty"`
	Key       string          `json:"key,omitempty"` // plaintext, only on create
}

type CreateInput struct {
	Name     *string         `json:"name"`
	Metadata json.RawMessage `json:"metadata" swaggertype:"object"`
}

type ListResponse struct {
	APIKeys []APIKey `json:"api_keys"`
}

type ListFilter struct {
	AppID  uuid.UUID
	Limit  int
	Offset int
}
