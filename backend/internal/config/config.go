package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config captures the runtime configuration for RhinoBox.
type Config struct {
	Addr           string
	DataDir        string
	MaxUploadBytes int64
	
	// Database connection strings (optional - if empty, only NDJSON storage is used)
	PostgresURL    string
	MongoURL       string
	DBMaxConns     int
	
	// Authentication configuration
	AuthEnabled bool
	
	// Security configuration
	Security SecurityConfig
}

// Load reads environment variables and falls back to sane defaults for hackathon usage.
func Load() (Config, error) {
	addr := getEnv("RHINOBOX_ADDR", ":8090")
	dataDir := getEnv("RHINOBOX_DATA_DIR", filepath.Join(".", "data"))

	maxUploadBytes := int64(25 * 1024 * 1024 * 1024) // 25 GiB default
	if raw := os.Getenv("RHINOBOX_MAX_UPLOAD_MB"); raw != "" {
		if mb, err := strconv.ParseInt(raw, 10, 64); err == nil && mb > 0 {
			maxUploadBytes = mb * 1024 * 1024
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}

	// Database configuration (optional)
	postgresURL := getEnv("RHINOBOX_POSTGRES_URL", "")
	mongoURL := getEnv("RHINOBOX_MONGO_URL", "")
	
	dbMaxConns := 100
	if raw := os.Getenv("RHINOBOX_DB_MAX_CONNS"); raw != "" {
		if conns, err := strconv.Atoi(raw); err == nil && conns > 0 {
			dbMaxConns = conns
		}
	}

	// Authentication configuration (defaults to false for security)
	authEnabled := getBoolEnvFromEnv("RHINOBOX_AUTH_ENABLED", false)

	return Config{
		Addr:           addr,
		DataDir:        dataDir,
		MaxUploadBytes: maxUploadBytes,
		PostgresURL:    postgresURL,
		MongoURL:       mongoURL,
		DBMaxConns:     dbMaxConns,
		AuthEnabled:    authEnabled,
		Security:       LoadSecurityConfig(),
	}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// getBoolEnvFromEnv reads a boolean environment variable (separate from security.go's getBoolEnv)
func getBoolEnvFromEnv(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}
