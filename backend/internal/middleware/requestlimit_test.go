package middleware

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Muneer320/RhinoBox/internal/config"
)

func TestRequestSizeLimitMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         config.SecurityConfig
		bodySize       int
		expectedStatus int
	}{
		{
			name: "Request within size limit",
			config: config.SecurityConfig{
				MaxRequestSize: 1024, // 1KB
			},
			bodySize:       512,
			expectedStatus: http.StatusOK,
		},
		{
			name: "Request exceeds size limit",
			config: config.SecurityConfig{
				MaxRequestSize: 1024, // 1KB
			},
			bodySize:       2048,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "Request size limit disabled",
			config: config.SecurityConfig{
				MaxRequestSize: 0,
			},
			bodySize:       10000,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := NewRequestSizeLimitMiddleware(tt.config, nil)
			server := httptest.NewServer(middleware.Handler(handler))
			defer server.Close()

			body := strings.Repeat("a", tt.bodySize)
			req, _ := http.NewRequest("POST", server.URL, bytes.NewBufferString(body))
			req.ContentLength = int64(tt.bodySize)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestIPFilterMiddleware_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         config.SecurityConfig
		clientIP       string
		expectedStatus int
	}{
		{
			name: "IP whitelist enabled - IP allowed",
			config: config.SecurityConfig{
				IPWhitelistEnabled: true,
				IPWhitelist:        parseIPListForTest("127.0.0.1/32"),
			},
			clientIP:       "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "IP whitelist enabled - IP not allowed",
			config: config.SecurityConfig{
				IPWhitelistEnabled: true,
				IPWhitelist:        parseIPListForTest("127.0.0.1/32"),
			},
			clientIP:       "192.168.1.1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "IP blacklist enabled - IP blocked",
			config: config.SecurityConfig{
				IPBlacklistEnabled: true,
				IPBlacklist:        parseIPListForTest("192.168.1.1/32"),
			},
			clientIP:       "192.168.1.1",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "IP blacklist enabled - IP not blocked",
			config: config.SecurityConfig{
				IPBlacklistEnabled: true,
				IPBlacklist:        parseIPListForTest("192.168.1.1/32"),
			},
			clientIP:       "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
		{
			name: "IP filtering disabled",
			config: config.SecurityConfig{
				IPWhitelistEnabled: false,
				IPBlacklistEnabled: false,
			},
			clientIP:       "127.0.0.1",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := NewIPFilterMiddleware(tt.config, nil)
			
			// Create a custom handler that sets the RemoteAddr
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Override RemoteAddr for testing
				r.RemoteAddr = tt.clientIP + ":12345"
				middleware.Handler(handler).ServeHTTP(w, r)
			})
			
			server := httptest.NewServer(testHandler)
			defer server.Close()

			req, _ := http.NewRequest("GET", server.URL, nil)
			// Also set X-Forwarded-For to test IP extraction
			req.Header.Set("X-Forwarded-For", tt.clientIP)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

// Helper function for testing - parse IP list
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

