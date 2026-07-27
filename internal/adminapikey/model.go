package adminapikey

import (
	"time"

	"github.com/google/uuid"
)

type AdminAPIKey struct {
	ID        uuid.UUID  `json:"id"`
	Name      *string    `json:"name,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	Key       string     `json:"key,omitempty"` // plaintext, only on create
}

type CreateInput struct {
	Name *string `json:"name"`
}

type ListResponse struct {
	AdminAPIKeys []AdminAPIKey `json:"admin_api_keys"`
}

type ListFilter struct {
	Limit  int
	Offset int
}
