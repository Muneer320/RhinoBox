package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
)

func TestSecurityHeadersMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         config.SecurityConfig
		expectedHeader string
		expectedValue  string
	}{
		{
			name: "Security headers enabled",
			config: config.SecurityConfig{
				SecurityHeadersEnabled: true,
				ContentTypeOptions:     "nosniff",
				FrameOptions:           "DENY",
				XSSProtection:          "1; mode=block",
				ReferrerPolicy:        "strict-origin-when-cross-origin",
				PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
			},
			expectedHeader: "X-Content-Type-Options",
			expectedValue:  "nosniff",
		},
		{
			name: "Security headers disabled",
			config: config.SecurityConfig{
				SecurityHeadersEnabled: false,
			},
			expectedHeader: "X-Content-Type-Options",
			expectedValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := NewSecurityHeadersMiddleware(tt.config, nil)
			server := httptest.NewServer(middleware.Handler(handler))
			defer server.Close()

			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			value := resp.Header.Get(tt.expectedHeader)
			if value != tt.expectedValue {
				t.Errorf("Expected header %q value %q, got %q", tt.expectedHeader, tt.expectedValue, value)
			}

			// Check other headers if enabled
			if tt.config.SecurityHeadersEnabled {
				if resp.Header.Get("X-Frame-Options") != tt.config.FrameOptions {
					t.Errorf("X-Frame-Options not set correctly")
				}
				if resp.Header.Get("X-XSS-Protection") != tt.config.XSSProtection {
					t.Errorf("X-XSS-Protection not set correctly")
				}
				if resp.Header.Get("Referrer-Policy") != tt.config.ReferrerPolicy {
					t.Errorf("Referrer-Policy not set correctly")
				}
			}
		})
	}
}

