package access

import (
	"github.com/google/uuid"
)

type CheckInput struct {
	UserID   uuid.UUID `json:"user_id" example:"01912345-6789-7abc-def0-123456789abc"`
	TenantID uuid.UUID `json:"tenant_id" example:"01912345-6789-7abc-def0-123456789abc"`
	Resource string    `json:"resource" example:"documents"`
	Action   string    `json:"action" example:"read"`
}

type CheckResponse struct {
	Allowed bool `json:"allowed"`
}
