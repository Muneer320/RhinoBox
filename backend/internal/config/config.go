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
}

// Load reads environment variables and falls back to sane defaults for hackathon usage.
func Load() (Config, error) {
	addr := getEnv("RHINOBOX_ADDR", ":8090")
	dataDir := getEnv("RHINOBOX_DATA_DIR", filepath.Join(".", "data"))

	maxUploadBytes := int64(512 * 1024 * 1024) // 512 MiB default
	if raw := os.Getenv("RHINOBOX_MAX_UPLOAD_MB"); raw != "" {
		if mb, err := strconv.ParseInt(raw, 10, 64); err == nil && mb > 0 {
			maxUploadBytes = mb * 1024 * 1024
		}
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create data dir: %w", err)
	}

	return Config{Addr: addr, DataDir: dataDir, MaxUploadBytes: maxUploadBytes}, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
