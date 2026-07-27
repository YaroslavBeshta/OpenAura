package resource

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Resource struct {
	ID        uuid.UUID       `json:"id" example:"01912345-6789-7abc-def0-123456789abc"`
	AppID     uuid.UUID       `json:"app_id" example:"01912345-6789-7abc-def0-123456789abc"`
	Name      string          `json:"name" example:"documents"`
	Metadata  json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	Name     string          `json:"name" example:"documents"`
	Metadata json.RawMessage `json:"metadata" swaggertype:"object"`
}

type UpdateInput struct {
	Name     *string          `json:"name,omitempty" example:"documents"`
	Metadata *json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type ListResponse struct {
	Resources []Resource `json:"resources"`
}

type ListFilter struct {
	AppID  uuid.UUID
	Limit  int
	Offset int
}
