package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
)

func TestCORSMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         config.SecurityConfig
		requestOrigin  string
		requestMethod  string
		expectedStatus int
		expectedOrigin string
	}{
		{
			name: "CORS enabled with wildcard origin",
			config: config.SecurityConfig{
				CORSEnabled:      true,
				CORSOrigins:      []string{"*"},
				CORSAllowMethods: []string{"GET", "POST"},
				CORSAllowHeaders: []string{"Content-Type"},
				CORSMaxAge:       3600 * time.Second,
				CORSAllowCreds:   false,
			},
			requestOrigin:  "https://example.com",
			requestMethod:  "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "*",
		},
		{
			name: "CORS enabled with specific origin",
			config: config.SecurityConfig{
				CORSEnabled:      true,
				CORSOrigins:      []string{"https://example.com"},
				CORSAllowMethods: []string{"GET", "POST"},
				CORSAllowHeaders: []string{"Content-Type"},
				CORSMaxAge:       3600 * time.Second,
				CORSAllowCreds:   true,
			},
			requestOrigin:  "https://example.com",
			requestMethod:  "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "https://example.com",
		},
		{
			name: "CORS preflight request",
			config: config.SecurityConfig{
				CORSEnabled:      true,
				CORSOrigins:      []string{"https://example.com"},
				CORSAllowMethods: []string{"GET", "POST"},
				CORSAllowHeaders: []string{"Content-Type"},
				CORSMaxAge:       3600 * time.Second,
				CORSAllowCreds:   true,
			},
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedStatus: http.StatusNoContent,
			expectedOrigin: "https://example.com",
		},
		{
			name: "CORS disabled",
			config: config.SecurityConfig{
				CORSEnabled: false,
			},
			requestOrigin:  "https://example.com",
			requestMethod:  "GET",
			expectedStatus: http.StatusOK,
			expectedOrigin: "",
		},
		{
			name: "Origin not allowed",
			config: config.SecurityConfig{
				CORSEnabled:      true,
				CORSOrigins:      []string{"https://allowed.com"},
				CORSAllowMethods: []string{"GET", "POST"},
				CORSAllowHeaders: []string{"Content-Type"},
				CORSMaxAge:       3600 * time.Second,
				CORSAllowCreds:   false,
			},
			requestOrigin:  "https://example.com",
			requestMethod:  "OPTIONS",
			expectedStatus: http.StatusForbidden,
			expectedOrigin: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := NewCORSMiddleware(tt.config, nil)
			server := httptest.NewServer(middleware.Handler(handler))
			defer server.Close()

			req, _ := http.NewRequest(tt.requestMethod, server.URL, nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			origin := resp.Header.Get("Access-Control-Allow-Origin")
			if origin != tt.expectedOrigin {
				t.Errorf("Expected origin %q, got %q", tt.expectedOrigin, origin)
			}
		})
	}
}

