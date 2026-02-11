package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID          uuid.UUID       `json:"id"`
	AppID       uuid.UUID       `json:"app_id"`
	URL         string          `json:"url"`
	Description string          `json:"description"`
	Secret      string          `json:"-"`
	FilterTypes []string        `json:"filter_types,omitempty"`
	RateLimit   *int            `json:"rate_limit,omitempty"`
	Disabled    bool            `json:"disabled"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateEndpointRequest struct {
	URL         string          `json:"url"`
	Description string          `json:"description,omitempty"`
	FilterTypes []string        `json:"filter_types,omitempty"`
	RateLimit   *int            `json:"rate_limit,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

type UpdateEndpointRequest struct {
	URL         *string         `json:"url,omitempty"`
	Description *string         `json:"description,omitempty"`
	FilterTypes []string        `json:"filter_types,omitempty"`
	RateLimit   *int            `json:"rate_limit,omitempty"`
	Disabled    *bool           `json:"disabled,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}
