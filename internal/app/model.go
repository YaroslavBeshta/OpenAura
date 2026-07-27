package app

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type App struct {
	ID        uuid.UUID       `json:"id" example:"01912345-6789-7abc-def0-123456789abc"`
	Name      string          `json:"name" example:"acme"`
	Metadata  json.RawMessage `json:"metadata" swaggertype:"object"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	DeletedAt *time.Time      `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	Name     string          `json:"name" example:"acme"`
	Metadata json.RawMessage `json:"metadata" swaggertype:"object"`
}

type UpdateInput struct {
	Name     *string          `json:"name,omitempty"`
	Metadata *json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type ListResponse struct {
	Apps []App `json:"apps"`
}

type ListFilter struct {
	Limit  int
	Offset int
}
