package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/models"
	"github.com/stym06/rebuf/server/internal/worker"
)

type MessageHandler struct {
	db       *pgxpool.Pool
	delivery *worker.DeliveryWorker
}

func NewMessageHandler(db *pgxpool.Pool, delivery *worker.DeliveryWorker) *MessageHandler {
	return &MessageHandler{db: db, delivery: delivery}
}

func (h *MessageHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{msg_id}", h.Get)
	r.Get("/{msg_id}/attempt", h.ListAttempts)
	return r
}

func (h *MessageHandler) Create(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	var req models.CreateMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EventType == "" {
		writeErr(w, http.StatusBadRequest, "event_type is required")
		return
	}
	if req.Payload == nil {
		writeErr(w, http.StatusBadRequest, "payload is required")
		return
	}

	ctx := r.Context()

	// Insert message
	var msg models.Message
	err := h.db.QueryRow(ctx,
		`INSERT INTO messages (app_id, event_type, event_id, payload)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, app_id, event_type, event_id, payload, created_at`,
		appID, req.EventType, req.EventID, req.Payload,
	).Scan(&msg.ID, &msg.AppID, &msg.EventType, &msg.EventID, &msg.Payload, &msg.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create message")
		return
	}

	// Find all active endpoints for this app that match the event type
	rows, err := h.db.Query(ctx,
		`SELECT id, app_id, url, secret, filter_types
		 FROM endpoints
		 WHERE app_id = $1 AND disabled = FALSE`,
		appID,
	)
	if err != nil {
		slog.Error("failed to query endpoints", "error", err)
		// Message is stored, delivery will be retried
		writeJSON(w, http.StatusAccepted, msg)
		return
	}
	defer rows.Close()

	var matchedEndpoints []struct {
		ID     uuid.UUID
		URL    string
		Secret string
	}

	for rows.Next() {
		var epID uuid.UUID
		var epAppID uuid.UUID
		var url, secret string
		var filterTypes []string
		if err := rows.Scan(&epID, &epAppID, &url, &secret, &filterTypes); err != nil {
			continue
		}

		// Check if endpoint subscribes to this event type
		if len(filterTypes) > 0 {
			matched := false
			for _, ft := range filterTypes {
				if ft == req.EventType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		matchedEndpoints = append(matchedEndpoints, struct {
			ID     uuid.UUID
			URL    string
			Secret string
		}{epID, url, secret})
	}

	// Create message attempts for each matched endpoint
	for _, ep := range matchedEndpoints {
		var attempt models.MessageAttempt
		err := h.db.QueryRow(ctx,
			`INSERT INTO message_attempts (message_id, endpoint_id, status, attempt_number)
			 VALUES ($1, $2, 'pending', 1)
			 RETURNING id, message_id, endpoint_id, status, attempt_number, created_at, updated_at`,
			msg.ID, ep.ID,
		).Scan(&attempt.ID, &attempt.MessageID, &attempt.EndpointID, &attempt.Status,
			&attempt.AttemptNumber, &attempt.CreatedAt, &attempt.UpdatedAt)
		if err != nil {
			slog.Error("failed to create message attempt", "error", err, "endpoint_id", ep.ID)
			continue
		}

		// Enqueue delivery
		payload, _ := json.Marshal(msg.Payload)
		h.delivery.Enqueue(worker.DeliveryTask{
			AttemptID:  attempt.ID,
			MessageID:  msg.ID,
			EndpointID: ep.ID,
			URL:        ep.URL,
			Secret:     ep.Secret,
			Payload:    payload,
			EventType:  msg.EventType,
		})
	}

	writeJSON(w, http.StatusAccepted, msg)
}

func (h *MessageHandler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	rows, err := h.db.Query(r.Context(),
		`SELECT id, app_id, event_type, event_id, payload, created_at
		 FROM messages WHERE app_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		appID, limit+1, offset,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list messages")
		return
	}
	defer rows.Close()

	msgs := []models.Message{}
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.AppID, &msg.EventType, &msg.EventID, &msg.Payload, &msg.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan message")
			return
		}
		msgs = append(msgs, msg)
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.Message]{
		Data:       msgs,
		TotalCount: len(msgs),
		HasMore:    hasMore,
	})
}

func (h *MessageHandler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(chi.URLParam(r, "msg_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	var msg models.Message
	err = h.db.QueryRow(r.Context(),
		`SELECT id, app_id, event_type, event_id, payload, created_at
		 FROM messages WHERE id = $1 AND app_id = $2`,
		msgID, appID,
	).Scan(&msg.ID, &msg.AppID, &msg.EventType, &msg.EventID, &msg.Payload, &msg.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "message not found")
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

func (h *MessageHandler) ListAttempts(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	msgID, err := uuid.Parse(chi.URLParam(r, "msg_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid message ID")
		return
	}

	// Verify message belongs to app
	var exists bool
	err = h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM messages WHERE id = $1 AND app_id = $2)`,
		msgID, appID,
	).Scan(&exists)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "message not found")
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, message_id, endpoint_id, status, response_status_code, response_body,
		        attempt_number, next_retry_at, created_at, updated_at
		 FROM message_attempts WHERE message_id = $1
		 ORDER BY created_at DESC`,
		msgID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list attempts")
		return
	}
	defer rows.Close()

	attempts := []models.MessageAttempt{}
	for rows.Next() {
		var a models.MessageAttempt
		if err := rows.Scan(&a.ID, &a.MessageID, &a.EndpointID, &a.Status,
			&a.ResponseStatusCode, &a.ResponseBody, &a.AttemptNumber,
			&a.NextRetryAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan attempt")
			return
		}
		attempts = append(attempts, a)
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.MessageAttempt]{
		Data:       attempts,
		TotalCount: len(attempts),
	})
}

func (h *MessageHandler) resolveAppID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	appID, err := uuid.Parse(chi.URLParam(r, "app_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid application ID")
		return uuid.Nil, false
	}

	orgID := middleware.OrgID(r.Context())
	var exists bool
	err = h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1 AND org_id = $2)`,
		appID, orgID,
	).Scan(&exists)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "application not found")
		return uuid.Nil, false
	}

	return appID, true
}
