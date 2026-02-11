package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/models"
)

type EndpointHandler struct {
	db *pgxpool.Pool
}

func NewEndpointHandler(db *pgxpool.Pool) *EndpointHandler {
	return &EndpointHandler{db: db}
}

func (h *EndpointHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{endpoint_id}", h.Get)
	r.Put("/{endpoint_id}", h.Update)
	r.Delete("/{endpoint_id}", h.Delete)
	r.Get("/{endpoint_id}/secret", h.GetSecret)
	r.Post("/{endpoint_id}/secret/rotate", h.RotateSecret)
	return r
}

func (h *EndpointHandler) Create(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	var req models.CreateEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.URL == "" {
		writeErr(w, http.StatusBadRequest, "url is required")
		return
	}

	secret := generateEndpointSecret()
	metadata := req.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	var ep models.Endpoint
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO endpoints (app_id, url, description, secret, filter_types, rate_limit, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, app_id, url, description, filter_types, rate_limit, disabled, metadata, created_at, updated_at`,
		appID, req.URL, req.Description, secret, req.FilterTypes, req.RateLimit, metadata,
	).Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Description, &ep.FilterTypes, &ep.RateLimit,
		&ep.Disabled, &ep.Metadata, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create endpoint")
		return
	}

	writeJSON(w, http.StatusCreated, ep)
}

func (h *EndpointHandler) List(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	rows, err := h.db.Query(r.Context(),
		`SELECT id, app_id, url, description, filter_types, rate_limit, disabled, metadata, created_at, updated_at
		 FROM endpoints WHERE app_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		appID, limit+1, offset,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}
	defer rows.Close()

	endpoints := []models.Endpoint{}
	for rows.Next() {
		var ep models.Endpoint
		if err := rows.Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Description,
			&ep.FilterTypes, &ep.RateLimit, &ep.Disabled, &ep.Metadata,
			&ep.CreatedAt, &ep.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan endpoint")
			return
		}
		endpoints = append(endpoints, ep)
	}

	hasMore := len(endpoints) > limit
	if hasMore {
		endpoints = endpoints[:limit]
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.Endpoint]{
		Data:       endpoints,
		TotalCount: len(endpoints),
		HasMore:    hasMore,
	})
}

func (h *EndpointHandler) Get(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	var ep models.Endpoint
	err = h.db.QueryRow(r.Context(),
		`SELECT id, app_id, url, description, filter_types, rate_limit, disabled, metadata, created_at, updated_at
		 FROM endpoints WHERE id = $1 AND app_id = $2`,
		endpointID, appID,
	).Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Description, &ep.FilterTypes, &ep.RateLimit,
		&ep.Disabled, &ep.Metadata, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	writeJSON(w, http.StatusOK, ep)
}

func (h *EndpointHandler) Update(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	var req models.UpdateEndpointRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var ep models.Endpoint
	err = h.db.QueryRow(r.Context(),
		`UPDATE endpoints
		 SET url = COALESCE($3, url),
		     description = COALESCE($4, description),
		     filter_types = COALESCE($5, filter_types),
		     rate_limit = COALESCE($6, rate_limit),
		     disabled = COALESCE($7, disabled),
		     metadata = COALESCE($8, metadata),
		     updated_at = NOW()
		 WHERE id = $1 AND app_id = $2
		 RETURNING id, app_id, url, description, filter_types, rate_limit, disabled, metadata, created_at, updated_at`,
		endpointID, appID, req.URL, req.Description, req.FilterTypes, req.RateLimit, req.Disabled, req.Metadata,
	).Scan(&ep.ID, &ep.AppID, &ep.URL, &ep.Description, &ep.FilterTypes, &ep.RateLimit,
		&ep.Disabled, &ep.Metadata, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	writeJSON(w, http.StatusOK, ep)
}

func (h *EndpointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM endpoints WHERE id = $1 AND app_id = $2`, endpointID, appID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete endpoint")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *EndpointHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	var secret string
	err = h.db.QueryRow(r.Context(),
		`SELECT secret FROM endpoints WHERE id = $1 AND app_id = $2`,
		endpointID, appID,
	).Scan(&secret)
	if err != nil {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"secret": secret})
}

func (h *EndpointHandler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	appID, ok := h.resolveAppID(w, r)
	if !ok {
		return
	}

	endpointID, err := uuid.Parse(chi.URLParam(r, "endpoint_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid endpoint ID")
		return
	}

	newSecret := generateEndpointSecret()
	tag, err := h.db.Exec(r.Context(),
		`UPDATE endpoints SET secret = $3, updated_at = NOW() WHERE id = $1 AND app_id = $2`,
		endpointID, appID, newSecret,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to rotate secret")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "endpoint not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"secret": newSecret})
}

func (h *EndpointHandler) resolveAppID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	appID, err := uuid.Parse(chi.URLParam(r, "app_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid application ID")
		return uuid.Nil, false
	}

	// Verify app belongs to org
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

func generateEndpointSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "whsec_" + base64.RawURLEncoding.EncodeToString(b)
}
