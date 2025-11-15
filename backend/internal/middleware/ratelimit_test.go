package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
)

func TestRateLimiter_Handler(t *testing.T) {
	tests := []struct {
		name           string
		config         config.SecurityConfig
		requestCount   int
		expectedStatus int
		expectRateLimit bool
	}{
		{
			name: "Rate limit enabled - within limit",
			config: config.SecurityConfig{
				RateLimitEnabled:    true,
				RateLimitRequests:   10,
				RateLimitWindow:     1 * time.Minute,
				RateLimitBurst:      5,
				RateLimitByIP:       true,
				RateLimitByEndpoint: false,
			},
			requestCount:    5,
			expectedStatus: http.StatusOK,
			expectRateLimit: false,
		},
		{
			name: "Rate limit enabled - exceeded",
			config: config.SecurityConfig{
				RateLimitEnabled:    true,
				RateLimitRequests:   5,
				RateLimitWindow:     1 * time.Minute,
				RateLimitBurst:      2,
				RateLimitByIP:       true,
				RateLimitByEndpoint: false,
			},
			requestCount:    10,
			expectedStatus:  http.StatusTooManyRequests,
			expectRateLimit:  true,
		},
		{
			name: "Rate limit disabled",
			config: config.SecurityConfig{
				RateLimitEnabled: false,
			},
			requestCount:    100,
			expectedStatus:  http.StatusOK,
			expectRateLimit:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			rateLimiter := NewRateLimiter(tt.config, nil)
			defer rateLimiter.Stop()

			middleware := rateLimiter.Handler(handler)
			server := httptest.NewServer(middleware)
			defer server.Close()

			rateLimited := false
			for i := 0; i < tt.requestCount; i++ {
				req, _ := http.NewRequest("GET", server.URL, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("Request %d failed: %v", i, err)
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusTooManyRequests {
					rateLimited = true
					// Check for rate limit headers
					if resp.Header.Get("X-RateLimit-Limit") == "" {
						t.Error("X-RateLimit-Limit header not set")
					}
					if resp.Header.Get("Retry-After") == "" {
						t.Error("Retry-After header not set")
					}
					break
				}
			}

			if rateLimited != tt.expectRateLimit {
				t.Errorf("Expected rate limit %v, got %v", tt.expectRateLimit, rateLimited)
			}
		})
	}
}

