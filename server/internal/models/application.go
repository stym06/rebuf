package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Application struct {
	ID        uuid.UUID       `json:"id"`
	OrgID     uuid.UUID       `json:"org_id"`
	UID       *string         `json:"uid,omitempty"`
	Name      string          `json:"name"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type CreateApplicationRequest struct {
	UID      *string         `json:"uid,omitempty"`
	Name     string          `json:"name"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

type UpdateApplicationRequest struct {
	Name     *string         `json:"name,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
