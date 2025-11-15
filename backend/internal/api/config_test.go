package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func TestHandleConfig(t *testing.T) {
	tests := []struct {
		name           string
		authEnabled    bool
		expectedStatus int
		checkAuth      func(*testing.T, *AppConfig)
	}{
		{
			name:           "auth disabled",
			authEnabled:    false,
			expectedStatus: http.StatusOK,
			checkAuth: func(t *testing.T, cfg *AppConfig) {
				if cfg.AuthEnabled {
					t.Error("expected auth_enabled to be false")
				}
				if !cfg.Features["multi_tenant"] {
					t.Error("expected multi_tenant to be true")
				}
				if !cfg.Features["deduplication"] {
					t.Error("expected deduplication to be true")
				}
			},
		},
		{
			name:           "auth enabled",
			authEnabled:    true,
			expectedStatus: http.StatusOK,
			checkAuth: func(t *testing.T, cfg *AppConfig) {
				if !cfg.AuthEnabled {
					t.Error("expected auth_enabled to be true")
				}
				if !cfg.Features["authentication"] {
					t.Error("expected authentication feature to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := config.Config{
				DataDir:        tmpDir,
				MaxUploadBytes: 10 * 1024 * 1024,
				AuthEnabled:    tt.authEnabled,
			}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

			server, err := NewServer(cfg, logger)
			if err != nil {
				t.Fatalf("failed to create server: %v", err)
			}

			req := httptest.NewRequest("GET", "/api/config", nil)
			w := httptest.NewRecorder()
			server.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				var response AppConfig
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if response.Version == "" {
					t.Error("expected version to be set")
				}

				if response.Features == nil {
					t.Error("expected features to be set")
				}

				tt.checkAuth(t, &response)
			}
		})
	}
}

func TestHandleConfigPublicAccess(t *testing.T) {
	// Test that config endpoint is accessible without authentication
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
		AuthEnabled:    true, // Even with auth enabled, config should be public
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config", nil)
	// No Authorization header
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 (public access), got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleConfigResponseFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Config{
		DataDir:        tmpDir,
		MaxUploadBytes: 10 * 1024 * 1024,
		AuthEnabled:    false,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	server, err := NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response AppConfig
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify all required fields are present
	if response.Version == "" {
		t.Error("version field is required")
	}

	if response.Features == nil {
		t.Error("features field is required")
	}

	// Verify feature flags
	requiredFeatures := []string{"authentication", "multi_tenant", "deduplication"}
	for _, feature := range requiredFeatures {
		if _, exists := response.Features[feature]; !exists {
			t.Errorf("required feature '%s' is missing", feature)
		}
	}
}


