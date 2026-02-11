package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID       `json:"id"`
	AppID     uuid.UUID       `json:"app_id"`
	EventType string          `json:"event_type"`
	EventID   *string         `json:"event_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type CreateMessageRequest struct {
	EventType string          `json:"event_type"`
	EventID   *string         `json:"event_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

type MessageAttempt struct {
	ID                 uuid.UUID  `json:"id"`
	MessageID          uuid.UUID  `json:"message_id"`
	EndpointID         uuid.UUID  `json:"endpoint_id"`
	Status             string     `json:"status"`
	ResponseStatusCode *int       `json:"response_status_code,omitempty"`
	ResponseBody       *string    `json:"response_body,omitempty"`
	AttemptNumber      int        `json:"attempt_number"`
	NextRetryAt        *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
