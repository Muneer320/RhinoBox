package integration

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/api"
	"github.com/Muneer320/RhinoBox/internal/config"
)

// setupSecurityTestServer creates a test server with security configuration
func setupSecurityTestServer(t *testing.T, securityCfg config.SecurityConfig) (*api.Server, string) {
	tempDir, err := os.MkdirTemp("", "rhinobox_security_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := config.Config{
		Addr:           ":0",
		DataDir:        tempDir,
		MaxUploadBytes: 10 * 1024 * 1024, // 10MB for testing
		Security:       securityCfg,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	server, err := api.NewServer(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return server, tempDir
}

// TestCORS_EndToEnd tests CORS functionality end-to-end
func TestCORS_EndToEnd(t *testing.T) {
	securityCfg := config.SecurityConfig{
		CORSEnabled:      true,
		CORSOrigins:      []string{"https://example.com", "https://app.example.com"},
		CORSAllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		CORSAllowHeaders: []string{"Content-Type", "Authorization"},
		CORSMaxAge:       3600 * time.Second,
		CORSAllowCreds:   true,
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	tests := []struct {
		name           string
		origin         string
		method         string
		expectedStatus int
		expectedHeader string
		expectedValue  string
	}{
		{
			name:           "Allowed origin - GET request",
			origin:         "https://example.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeader: "Access-Control-Allow-Origin",
			expectedValue:  "https://example.com",
		},
		{
			name:           "Allowed origin - OPTIONS preflight",
			origin:         "https://example.com",
			method:         "OPTIONS",
			expectedStatus: http.StatusNoContent,
			expectedHeader: "Access-Control-Allow-Origin",
			expectedValue:  "https://example.com",
		},
		{
			name:           "Disallowed origin",
			origin:         "https://evil.com",
			method:         "GET",
			expectedStatus: http.StatusOK, // Request succeeds but no CORS header
			expectedHeader: "Access-Control-Allow-Origin",
			expectedValue:  "", // No CORS header for disallowed origin
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/healthz", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.method == "OPTIONS" {
				req.Header.Set("Access-Control-Request-Method", "GET")
			}

			w := httptest.NewRecorder()
			server.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			headerValue := w.Header().Get(tt.expectedHeader)
			if headerValue != tt.expectedValue {
				t.Errorf("Expected header %q value %q, got %q", tt.expectedHeader, tt.expectedValue, headerValue)
			}

			// Check additional CORS headers for preflight
			if tt.method == "OPTIONS" {
				if w.Header().Get("Access-Control-Allow-Methods") == "" {
					t.Error("Access-Control-Allow-Methods header not set for preflight")
				}
				if w.Header().Get("Access-Control-Max-Age") == "" {
					t.Error("Access-Control-Max-Age header not set for preflight")
				}
			}
		})
	}
}

// TestSecurityHeaders_EndToEnd tests security headers end-to-end
func TestSecurityHeaders_EndToEnd(t *testing.T) {
	securityCfg := config.SecurityConfig{
		SecurityHeadersEnabled: true,
		ContentTypeOptions:     "nosniff",
		FrameOptions:           "DENY",
		XSSProtection:          "1; mode=block",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
	}

	for header, expectedValue := range expectedHeaders {
		actualValue := w.Header().Get(header)
		if actualValue != expectedValue {
			t.Errorf("Expected header %q value %q, got %q", header, expectedValue, actualValue)
		}
	}
}

// TestRateLimiting_EndToEnd tests rate limiting end-to-end
func TestRateLimiting_EndToEnd(t *testing.T) {
	securityCfg := config.SecurityConfig{
		RateLimitEnabled:    true,
		RateLimitRequests:   5,              // 5 requests
		RateLimitWindow:     1 * time.Second, // per second
		RateLimitBurst:      2,              // burst of 2
		RateLimitByIP:       true,
		RateLimitByEndpoint: false,
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	// Make requests within limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}

		// Check rate limit headers
		limit := w.Header().Get("X-RateLimit-Limit")
		if limit == "" {
			t.Error("X-RateLimit-Limit header not set")
		}
		remaining := w.Header().Get("X-RateLimit-Remaining")
		if remaining == "" {
			t.Error("X-RateLimit-Remaining header not set")
		}
	}

	// Make request that should be rate limited
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set for rate limited request")
	}
}

// TestRequestSizeLimit_EndToEnd tests request size limiting end-to-end
func TestRequestSizeLimit_EndToEnd(t *testing.T) {
	securityCfg := config.SecurityConfig{
		MaxRequestSize: 1024, // 1KB limit
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	tests := []struct {
		name           string
		bodySize       int
		expectedStatus int
	}{
		{
			name:           "Request within size limit",
			bodySize:       512,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Request exceeds size limit",
			bodySize:       2048,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.Repeat("a", tt.bodySize)
			req := httptest.NewRequest("POST", "/ingest/json", bytes.NewBufferString(body))
			req.Header.Set("Content-Type", "application/json")
			req.ContentLength = int64(tt.bodySize)

			w := httptest.NewRecorder()
			server.Router().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

// TestIPFiltering_EndToEnd tests IP whitelist/blacklist end-to-end
func TestIPFiltering_EndToEnd(t *testing.T) {
	// Parse IP lists
	whitelist := parseIPListForTest("127.0.0.1/32")
	blacklist := parseIPListForTest("192.168.1.100/32")

	securityCfg := config.SecurityConfig{
		IPWhitelistEnabled: true,
		IPWhitelist:        whitelist,
		IPBlacklistEnabled: false,
		IPBlacklist:        nil,
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	t.Run("IP in whitelist", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "127.0.0.1")
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("IP not in whitelist", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		req.Header.Set("X-Forwarded-For", "192.168.1.1")
		w := httptest.NewRecorder()
		server.Router().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})

	// Test blacklist
	securityCfg2 := config.SecurityConfig{
		IPWhitelistEnabled: false,
		IPWhitelist:        nil,
		IPBlacklistEnabled: true,
		IPBlacklist:        blacklist,
	}

	server2, tempDir2 := setupSecurityTestServer(t, securityCfg2)
	defer os.RemoveAll(tempDir2)
	defer server2.Stop()

	t.Run("IP in blacklist", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		req.Header.Set("X-Forwarded-For", "192.168.1.100")
		w := httptest.NewRecorder()
		server2.Router().ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403, got %d", w.Code)
		}
	})
}

// TestSecurityIntegration_EndToEnd tests all security features working together
func TestSecurityIntegration_EndToEnd(t *testing.T) {
	securityCfg := config.SecurityConfig{
		CORSEnabled:          true,
		CORSOrigins:          []string{"https://example.com"},
		CORSAllowMethods:     []string{"GET", "POST"},
		CORSAllowHeaders:     []string{"Content-Type"},
		CORSMaxAge:           3600 * time.Second,
		CORSAllowCreds:       true,
		SecurityHeadersEnabled: true,
		ContentTypeOptions:     "nosniff",
		FrameOptions:           "DENY",
		XSSProtection:          "1; mode=block",
		RateLimitEnabled:      true,
		RateLimitRequests:      10,
		RateLimitWindow:        1 * time.Minute,
		RateLimitBurst:         5,
		RateLimitByIP:           true,
		MaxRequestSize:         1024 * 1024, // 1MB
	}

	server, tempDir := setupSecurityTestServer(t, securityCfg)
	defer os.RemoveAll(tempDir)
	defer server.Stop()

	// Test that all security features work together
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("Origin", "https://example.com")
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	// Check CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Error("CORS header not set correctly")
	}

	// Check security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Security header not set correctly")
	}

	// Check rate limit headers
	if w.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("Rate limit header not set")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Helper function to parse IP list for testing
func parseIPListForTest(list string) []net.IPNet {
	if list == "" {
		return nil
	}

	parts := strings.Split(list, ",")
	result := make([]net.IPNet, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		// Try parsing as CIDR
		_, ipnet, err := net.ParseCIDR(trimmed)
		if err == nil {
			result = append(result, *ipnet)
			continue
		}

		// Try parsing as single IP (convert to /32 or /128)
		ip := net.ParseIP(trimmed)
		if ip != nil {
			if ip.To4() != nil {
				// IPv4
				_, ipnet, _ := net.ParseCIDR(trimmed + "/32")
				if ipnet != nil {
					result = append(result, *ipnet)
				}
			} else {
				// IPv6
				_, ipnet, _ := net.ParseCIDR(trimmed + "/128")
				if ipnet != nil {
					result = append(result, *ipnet)
				}
			}
		}
	}

	return result
}

