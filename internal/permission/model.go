package permission

import (
	"time"

	"github.com/google/uuid"
)

type Permission struct {
	ID         uuid.UUID  `json:"id" example:"01912345-6789-7abc-def0-123456789abc"`
	RoleID     uuid.UUID  `json:"role_id" example:"01912345-6789-7abc-def0-123456789abc"`
	ResourceID uuid.UUID  `json:"resource_id" example:"01912345-6789-7abc-def0-123456789abc"`
	ActionID   uuid.UUID  `json:"action_id" example:"01912345-6789-7abc-def0-123456789abc"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	ResourceID uuid.UUID `json:"resource_id" example:"01912345-6789-7abc-def0-123456789abc"`
	ActionID   uuid.UUID `json:"action_id" example:"01912345-6789-7abc-def0-123456789abc"`
}

type ListResponse struct {
	Permissions []Permission `json:"permissions"`
}

type ListFilter struct {
	AppID      uuid.UUID
	RoleID     uuid.UUID
	ResourceID *uuid.UUID
	ActionID   *uuid.UUID
	Limit      int
	Offset     int
}
