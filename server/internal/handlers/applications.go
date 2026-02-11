package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/models"
)

type ApplicationHandler struct {
	db *pgxpool.Pool
}

func NewApplicationHandler(db *pgxpool.Pool) *ApplicationHandler {
	return &ApplicationHandler{db: db}
}

func (h *ApplicationHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{app_id}", h.Get)
	r.Put("/{app_id}", h.Update)
	r.Delete("/{app_id}", h.Delete)
	return r
}

func (h *ApplicationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateApplicationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	orgID := middleware.OrgID(r.Context())
	metadata := req.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	var app models.Application
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO applications (org_id, uid, name, metadata)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, org_id, uid, name, metadata, created_at, updated_at`,
		orgID, req.UID, req.Name, metadata,
	).Scan(&app.ID, &app.OrgID, &app.UID, &app.Name, &app.Metadata, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create application")
		return
	}

	writeJSON(w, http.StatusCreated, app)
}

func (h *ApplicationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgID(r.Context())
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	rows, err := h.db.Query(r.Context(),
		`SELECT id, org_id, uid, name, metadata, created_at, updated_at
		 FROM applications WHERE org_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		orgID, limit+1, offset,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list applications")
		return
	}
	defer rows.Close()

	apps := []models.Application{}
	for rows.Next() {
		var app models.Application
		if err := rows.Scan(&app.ID, &app.OrgID, &app.UID, &app.Name, &app.Metadata, &app.CreatedAt, &app.UpdatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan application")
			return
		}
		apps = append(apps, app)
	}

	hasMore := len(apps) > limit
	if hasMore {
		apps = apps[:limit]
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.Application]{
		Data:       apps,
		TotalCount: len(apps),
		HasMore:    hasMore,
	})
}

func (h *ApplicationHandler) Get(w http.ResponseWriter, r *http.Request) {
	app, ok := h.loadApp(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Update(w http.ResponseWriter, r *http.Request) {
	appID, orgID, ok := h.parseAppID(w, r)
	if !ok {
		return
	}

	var req models.UpdateApplicationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var app models.Application
	err := h.db.QueryRow(r.Context(),
		`UPDATE applications
		 SET name = COALESCE($3, name),
		     metadata = COALESCE($4, metadata),
		     updated_at = NOW()
		 WHERE id = $1 AND org_id = $2
		 RETURNING id, org_id, uid, name, metadata, created_at, updated_at`,
		appID, orgID, req.Name, req.Metadata,
	).Scan(&app.ID, &app.OrgID, &app.UID, &app.Name, &app.Metadata, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "application not found")
		return
	}

	writeJSON(w, http.StatusOK, app)
}

func (h *ApplicationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	appID, orgID, ok := h.parseAppID(w, r)
	if !ok {
		return
	}

	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM applications WHERE id = $1 AND org_id = $2`, appID, orgID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete application")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "application not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ApplicationHandler) loadApp(w http.ResponseWriter, r *http.Request) (models.Application, bool) {
	appID, orgID, ok := h.parseAppID(w, r)
	if !ok {
		return models.Application{}, false
	}

	var app models.Application
	err := h.db.QueryRow(r.Context(),
		`SELECT id, org_id, uid, name, metadata, created_at, updated_at
		 FROM applications WHERE id = $1 AND org_id = $2`,
		appID, orgID,
	).Scan(&app.ID, &app.OrgID, &app.UID, &app.Name, &app.Metadata, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusNotFound, "application not found")
		return models.Application{}, false
	}

	return app, true
}

func (h *ApplicationHandler) parseAppID(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	appID, err := uuid.Parse(chi.URLParam(r, "app_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid application ID")
		return uuid.Nil, uuid.Nil, false
	}
	orgID := middleware.OrgID(r.Context())
	return appID, orgID, true
}
