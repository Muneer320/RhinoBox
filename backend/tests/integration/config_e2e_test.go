package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
	"log/slog"
)

func TestConfigEndpointE2E(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expectAuth  bool
		description string
	}{
		{
			name:        "auth disabled by default",
			envValue:    "",
			expectAuth:  false,
			description: "When AUTH_ENABLED is not set, should default to false",
		},
		{
			name:        "auth explicitly disabled",
			envValue:    "false",
			expectAuth:  false,
			description: "When AUTH_ENABLED=false, should return false",
		},
		{
			name:        "auth explicitly enabled",
			envValue:    "true",
			expectAuth:  true,
			description: "When AUTH_ENABLED=true, should return true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("RHINOBOX_AUTH_ENABLED", tt.envValue)
				defer os.Unsetenv("RHINOBOX_AUTH_ENABLED")
			} else {
				os.Unsetenv("RHINOBOX_AUTH_ENABLED")
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			if cfg.AuthEnabled != tt.expectAuth {
				t.Errorf("expected AuthEnabled=%v, got %v", tt.expectAuth, cfg.AuthEnabled)
			}

			// Create server
			cfg.DataDir = t.TempDir()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			server, err := api.NewServer(cfg, logger)
			if err != nil {
				t.Fatalf("failed to create server: %v", err)
			}

			// Test endpoint
			req, _ := http.NewRequest("GET", "/api/config", nil)
			rr := httptest.NewRecorder()
			server.Router().ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var response api.AppConfig
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if response.AuthEnabled != tt.expectAuth {
				t.Errorf("expected auth_enabled=%v in response, got %v", tt.expectAuth, response.AuthEnabled)
			}

			// Verify features
			if response.Features["authentication"] != tt.expectAuth {
				t.Errorf("expected features.authentication=%v, got %v", tt.expectAuth, response.Features["authentication"])
			}
		})
	}
}

func TestConfigEndpointPublicAccess(t *testing.T) {
	// Verify that /api/config is accessible without authentication
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        t.TempDir(),
		MaxUploadBytes: 32 * 1024 * 1024,
		AuthEnabled:    true, // Even with auth enabled, config should be public
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/config", nil)
	// No Authorization header
	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 (public access), got %d: %s", rr.Code, rr.Body.String())
	}
}

