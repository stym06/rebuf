package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/stym06/rebuf/server/internal/middleware"
	"github.com/stym06/rebuf/server/internal/models"
)

type AuthHandler struct {
	db        *pgxpool.Pool
	jwtSecret string
}

func NewAuthHandler(db *pgxpool.Pool, jwtSecret string) *AuthHandler {
	return &AuthHandler{db: db, jwtSecret: jwtSecret}
}

func (h *AuthHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/signup", h.Signup)
	r.Post("/login", h.Login)
	return r
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.OrgName == "" {
		writeErr(w, http.StatusBadRequest, "email, password, and org_name are required")
		return
	}

	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	ctx := r.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	// Create user
	var user models.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name) VALUES ($1, $2, $3)
		 RETURNING id, email, name, created_at, updated_at`,
		req.Email, string(hash), req.Name,
	).Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeErr(w, http.StatusConflict, "email already registered")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Create organization
	slug := slugify(req.OrgName)
	var org models.Organization
	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2)
		 RETURNING id, name, slug, created_at, updated_at`,
		req.OrgName, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create organization")
		return
	}

	// Add user as org owner
	_, err = tx.Exec(ctx,
		`INSERT INTO org_members (org_id, user_id, role) VALUES ($1, $2, 'owner')`,
		org.ID, user.ID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to add org member")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to commit")
		return
	}

	token, err := h.generateToken(user.ID, org.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, models.AuthResponse{
		Token: token,
		User:  user,
		Org:   &org,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeErr(w, http.StatusBadRequest, "email and password are required")
		return
	}

	ctx := r.Context()
	var user models.User
	err := h.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, created_at, updated_at FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Get user's org (first one for now)
	var org models.Organization
	err = h.db.QueryRow(ctx,
		`SELECT o.id, o.name, o.slug, o.created_at, o.updated_at
		 FROM organizations o
		 JOIN org_members om ON o.id = om.org_id
		 WHERE om.user_id = $1
		 ORDER BY om.created_at ASC LIMIT 1`,
		user.ID,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to fetch organization")
		return
	}

	token, err := h.generateToken(user.ID, org.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, models.AuthResponse{
		Token: token,
		User:  user,
		Org:   &org,
	})
}

func (h *AuthHandler) generateToken(userID, orgID uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID.String(),
		"org": orgID.String(),
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(72 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}

// APIKeyHandler manages API keys (requires auth).
type APIKeyHandler struct {
	db *pgxpool.Pool
}

func NewAPIKeyHandler(db *pgxpool.Pool) *APIKeyHandler {
	return &APIKeyHandler{db: db}
}

func (h *APIKeyHandler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Delete("/{key_id}", h.Delete)
	return r
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateAPIKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}

	orgID := middleware.OrgID(r.Context())

	// Generate API key: rbf_<8-char-prefix>_<32-char-secret>
	prefix := randomHex(4)
	secret := randomHex(16)
	fullKey := fmt.Sprintf("rbf_%s_%s", prefix, secret)
	keyHash := middleware.HashAPIKey(fullKey)

	var apiKey models.APIKey
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO api_keys (org_id, name, key_hash, prefix, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, org_id, name, prefix, expires_at, created_at`,
		orgID, req.Name, keyHash, prefix, req.ExpiresAt,
	).Scan(&apiKey.ID, &apiKey.OrgID, &apiKey.Name, &apiKey.Prefix, &apiKey.ExpiresAt, &apiKey.CreatedAt)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create API key")
		return
	}

	writeJSON(w, http.StatusCreated, models.CreateAPIKeyResponse{
		APIKey: apiKey,
		Key:    fullKey,
	})
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := middleware.OrgID(r.Context())

	rows, err := h.db.Query(r.Context(),
		`SELECT id, org_id, name, prefix, expires_at, created_at
		 FROM api_keys WHERE org_id = $1 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list API keys")
		return
	}
	defer rows.Close()

	keys := []models.APIKey{}
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.OrgID, &k.Name, &k.Prefix, &k.ExpiresAt, &k.CreatedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to scan API key")
			return
		}
		keys = append(keys, k)
	}

	writeJSON(w, http.StatusOK, models.ListResponse[models.APIKey]{
		Data:       keys,
		TotalCount: len(keys),
	})
}

func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	keyID, err := uuid.Parse(chi.URLParam(r, "key_id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid key ID")
		return
	}

	orgID := middleware.OrgID(r.Context())
	tag, err := h.db.Exec(r.Context(),
		`DELETE FROM api_keys WHERE id = $1 AND org_id = $2`, keyID, orgID,
	)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete API key")
		return
	}
	if tag.RowsAffected() == 0 {
		writeErr(w, http.StatusNotFound, "API key not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	slug := strings.ToLower(strings.TrimSpace(s))
	slug = nonAlphaNum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "org"
	}
	return slug + "-" + randomHex(3)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
