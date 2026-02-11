package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/stym06/rebuf/server/internal/models"
)

type contextKey string

const (
	ContextKeyUserID contextKey = "user_id"
	ContextKeyOrgID  contextKey = "org_id"
)

func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ContextKeyUserID).(uuid.UUID)
	return id
}

func OrgID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ContextKeyOrgID).(uuid.UUID)
	return id
}

// Auth authenticates requests via JWT Bearer token or API key.
func Auth(jwtSecret string, pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 {
				writeError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			scheme := strings.ToLower(parts[0])
			token := parts[1]

			var userID, orgID uuid.UUID
			var err error

			switch scheme {
			case "bearer":
				userID, orgID, err = validateJWT(token, jwtSecret, pool, r.Context())
			case "apikey":
				orgID, err = validateAPIKey(token, pool, r.Context())
			default:
				writeError(w, http.StatusUnauthorized, "unsupported auth scheme")
				return
			}

			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyOrgID, orgID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func validateJWT(tokenStr, secret string, pool *pgxpool.Pool, ctx context.Context) (uuid.UUID, uuid.UUID, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, uuid.Nil, jwt.ErrTokenInvalidClaims
	}

	userID, err := uuid.Parse(claims["sub"].(string))
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	orgID, err := uuid.Parse(claims["org"].(string))
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	return userID, orgID, nil
}

func validateAPIKey(key string, pool *pgxpool.Pool, ctx context.Context) (uuid.UUID, error) {
	// API keys have format: rbf_<prefix>_<secret>
	// We store sha256(full_key) and the prefix for lookup
	prefix := extractPrefix(key)
	keyHash := hashKey(key)

	var apiKey models.APIKey
	err := pool.QueryRow(ctx,
		`SELECT id, org_id, expires_at FROM api_keys WHERE prefix = $1 AND key_hash = $2`,
		prefix, keyHash,
	).Scan(&apiKey.ID, &apiKey.OrgID, &apiKey.ExpiresAt)
	if err != nil {
		return uuid.Nil, err
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return uuid.Nil, jwt.ErrTokenExpired
	}

	return apiKey.OrgID, nil
}

func extractPrefix(key string) string {
	// key format: rbf_XXXXXXXX_YYYYYYYYYYYY
	parts := strings.SplitN(key, "_", 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return key[:8]
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// HashAPIKey hashes an API key for storage.
func HashAPIKey(key string) string {
	return hashKey(key)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
