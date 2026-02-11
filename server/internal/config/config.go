package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port        int
	DatabaseURL string
	JWTSecret   string

	// Rebuf WAL settings
	WALDir         string
	WALMaxLogSize  int64
	WALMaxSegments int

	// Delivery worker settings
	DeliveryWorkers    int
	DeliveryTimeout    time.Duration
	MaxRetryAttempts   int
	RetryBaseDelay     time.Duration

	// CORS
	AllowedOrigins []string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	walMaxLogSize, _ := strconv.ParseInt(getEnv("WAL_MAX_LOG_SIZE", "10485760"), 10, 64)
	walMaxSegments, _ := strconv.Atoi(getEnv("WAL_MAX_SEGMENTS", "100"))
	deliveryWorkers, _ := strconv.Atoi(getEnv("DELIVERY_WORKERS", "10"))
	deliveryTimeoutSec, _ := strconv.Atoi(getEnv("DELIVERY_TIMEOUT_SEC", "30"))
	maxRetryAttempts, _ := strconv.Atoi(getEnv("MAX_RETRY_ATTEMPTS", "7"))

	dbURL := getEnv("DATABASE_URL", "")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return &Config{
		Port:               port,
		DatabaseURL:        dbURL,
		JWTSecret:          jwtSecret,
		WALDir:             getEnv("WAL_DIR", "/var/lib/rebuf/wal"),
		WALMaxLogSize:      walMaxLogSize,
		WALMaxSegments:     walMaxSegments,
		DeliveryWorkers:    deliveryWorkers,
		DeliveryTimeout:    time.Duration(deliveryTimeoutSec) * time.Second,
		MaxRetryAttempts:   maxRetryAttempts,
		RetryBaseDelay:     5 * time.Second,
		AllowedOrigins:     []string{"http://localhost:3000", "http://localhost:5173"},
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
