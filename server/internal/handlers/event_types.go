package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/models"
)

type EventTypeHandler struct {
	db *pgxpool.Pool
}

func NewEventTypeHandler(db *pgxpool.Pool) *EventTypeHandler {
	return &EventTypeHandler{db: db}
}

func (h *EventTypeHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Put("/{event_type_name}", h.Update)
	r.Delete("/{event_type_name}", h.Delete)
	return r
}

func (h *EventTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateEventTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	orgID := middleware.OrgID(r.Context())

	var et models.EventType
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO event_types (org_id, name, description, schema)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, name, description, schema, created_at, updated_at`,
		orgID, req.Name, req.Description, req.Schema,
	).Scan(&et.ID, &et.OrgID, &et.Name, &et.Description, &et.Schema, &et.CreatedAt, &et.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeErr(w, http.StatusConflict, "event type already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create event type")
		return
	}

	writeJSON(w, http.StatusCreated, et)
}

func (h *EventTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgID(r.Context())
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	rows, err := h.db.Query(r.Context(),
		`SELECT id, org_id, name, description, schema, created_at, updated_at
		 FROM event_types WHERE org_id = $1
		 ORDER BY name ASC LIMIT $2 OFFSET $3`,
		orgID, limit+1, offset,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list event types")
		return
	}
	defer rows.Close()

	types := []models.EventType{}
	for rows.Next() {
		var et models.EventType
		if err := rows.Scan(&et.ID, &et.OrgID, &et.Name, &et.Description, &et.Schema, &et.CreatedAt, &et.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan event type")
			return
		}
		types = append(types, et)
	}

	hasMore := len(types) > limit
	if hasMore {
		types = types[:limit]
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.EventType]{
		Data:       types,
		TotalCount: len(types),
		HasMore:    hasMore,
	})
}

func (h *EventTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "event_type_name")
	orgID := middleware.OrgID(r.Context())

	var req models.UpdateEventTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var et models.EventType
	err := h.db.QueryRow(r.Context(),
		`UPDATE event_types
		 SET description = COALESCE($3, description),
		     schema = COALESCE($4, schema),
		     updated_at = NOW()
		 WHERE name = $1 AND org_id = $2
		 RETURNING id, org_id, name, description, schema, created_at, updated_at`,
		name, orgID, req.Description, req.Schema,
	).Scan(&et.ID, &et.OrgID, &et.Name, &et.Description, &et.Schema, &et.CreatedAt, &et.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "event type not found")
		return
	}

	writeJSON(w, http.StatusOK, et)
}

func (h *EventTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "event_type_name")
	orgID := middleware.OrgID(r.Context())

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM event_types WHERE name = $1 AND org_id = $2`, name, orgID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete event type")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "event type not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
