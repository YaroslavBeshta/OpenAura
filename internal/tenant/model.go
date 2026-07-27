package tenant

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID        uuid.UUID       `json:"id" example:"01912345-6789-7abc-def0-123456789abc"`
	AppID     uuid.UUID       `json:"app_id" example:"01912345-6789-7abc-def0-123456789abc"`
	Metadata  json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	Metadata json.RawMessage `json:"metadata" swaggertype:"object"`
}

type UpdateInput struct {
	Metadata *json.RawMessage `json:"metadata" swaggertype:"object"`
}

type ListResponse struct {
	Tenants []Tenant `json:"tenants"`
}

type ListFilter struct {
	AppID  uuid.UUID
	Limit  int
	Offset int
}
