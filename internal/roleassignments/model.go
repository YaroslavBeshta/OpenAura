package roleassignments

import (
	"time"

	"github.com/google/uuid"
)

type RoleAssignment struct {
	ID        uuid.UUID  `json:"id" example:"01912345-6789-7abc-def0-123456789abc"`
	UserID    uuid.UUID  `json:"user_id" example:"01912345-6789-7abc-def0-123456789abc"`
	RoleID    uuid.UUID  `json:"role_id" example:"01912345-6789-7abc-def0-123456789abc"`
	TenantID  uuid.UUID  `json:"tenant_id" example:"01912345-6789-7abc-def0-123456789abc"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

type CreateInput struct {
	UserID   uuid.UUID `json:"user_id" example:"01912345-6789-7abc-def0-123456789abc"`
	RoleID   uuid.UUID `json:"role_id" example:"01912345-6789-7abc-def0-123456789abc"`
	TenantID uuid.UUID `json:"tenant_id" example:"01912345-6789-7abc-def0-123456789abc"`
}

type UpdateInput struct {
	UserID   *uuid.UUID `json:"user_id,omitempty"`
	RoleID   *uuid.UUID `json:"role_id,omitempty"`
	TenantID *uuid.UUID `json:"tenant_id,omitempty"`
}

type ListResponse struct {
	RoleAssignments []RoleAssignment `json:"roleassignments"`
}

type ListFilter struct {
	AppID    uuid.UUID
	UserID   *uuid.UUID
	RoleID   *uuid.UUID
	TenantID *uuid.UUID
	Limit    int
	Offset   int
}
