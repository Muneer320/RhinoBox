package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	config     config.SecurityConfig
	logger     *slog.Logger
	clients    map[string]*clientLimiter
	mu         sync.RWMutex
	cleanupTicker *time.Ticker
	stopCleanup    chan bool
}

// clientLimiter tracks rate limit state for a single client
type clientLimiter struct {
	tokens     int
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(cfg config.SecurityConfig, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		config:  cfg,
		logger:  logger,
		clients: make(map[string]*clientLimiter),
		stopCleanup: make(chan bool),
	}

	// Start cleanup goroutine to remove old entries
	rl.cleanupTicker = time.NewTicker(5 * time.Minute)
	go rl.cleanup()

	return rl
}

// cleanup removes old client entries periodically
func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, client := range rl.clients {
				client.mu.Lock()
				// Remove entries that haven't been used in 2x the rate limit window
				if now.Sub(client.lastUpdate) > 2*rl.config.RateLimitWindow {
					delete(rl.clients, key)
				}
				client.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopCleanup:
			rl.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the rate limiter cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// Handler returns the middleware handler function
func (rl *RateLimiter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.config.RateLimitEnabled {
			next.ServeHTTP(w, r)
			return
		}

		// Get client identifier
		clientKey := rl.getClientKey(r)

		// Check rate limit
		allowed, remaining, resetTime := rl.allow(clientKey, r.URL.Path)
		if !allowed {
			w.Header().Set("X-RateLimit-Limit", formatInt(rl.config.RateLimitRequests))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", formatInt64(resetTime.Unix()))
			w.Header().Set("Retry-After", formatInt64(int64(time.Until(resetTime).Seconds())))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", formatInt(rl.config.RateLimitRequests))
		w.Header().Set("X-RateLimit-Remaining", formatInt(remaining))
		w.Header().Set("X-RateLimit-Reset", formatInt64(resetTime.Unix()))

		// Continue with the request
		next.ServeHTTP(w, r)
	})
}

// getClientKey returns a unique key for the client based on configuration
func (rl *RateLimiter) getClientKey(r *http.Request) string {
	parts := make([]string, 0, 2)

	if rl.config.RateLimitByIP {
		ip := getClientIP(r)
		parts = append(parts, ip)
	}

	if rl.config.RateLimitByEndpoint {
		parts = append(parts, r.URL.Path)
	}

	if len(parts) == 0 {
		// Default to IP if nothing is configured
		return getClientIP(r)
	}

	return joinStrings(parts, ":")
}

// allow checks if a request is allowed and updates the token bucket
func (rl *RateLimiter) allow(clientKey, endpoint string) (allowed bool, remaining int, resetTime time.Time) {
	rl.mu.Lock()
	client, exists := rl.clients[clientKey]
	if !exists {
		client = &clientLimiter{
			tokens:     rl.config.RateLimitRequests + rl.config.RateLimitBurst,
			lastUpdate: time.Now(),
		}
		rl.clients[clientKey] = client
	}
	rl.mu.Unlock()

	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(client.lastUpdate)

	// Calculate how many tokens to add based on elapsed time
	tokensToAdd := int(elapsed.Seconds() / rl.config.RateLimitWindow.Seconds() * float64(rl.config.RateLimitRequests))
	if tokensToAdd > 0 {
		// Refill tokens, but don't exceed the limit + burst
		maxTokens := rl.config.RateLimitRequests + rl.config.RateLimitBurst
		client.tokens = min(client.tokens+tokensToAdd, maxTokens)
		client.lastUpdate = now
	}

	// Check if we have tokens available
	if client.tokens <= 0 {
		// Calculate reset time
		resetTime = client.lastUpdate.Add(rl.config.RateLimitWindow)
		return false, 0, resetTime
	}

	// Consume a token
	client.tokens--
	remaining = client.tokens

	// Calculate reset time (when tokens will be fully refilled)
	if client.tokens < rl.config.RateLimitRequests {
		tokensNeeded := rl.config.RateLimitRequests - client.tokens
		timeNeeded := time.Duration(float64(tokensNeeded) / float64(rl.config.RateLimitRequests) * float64(rl.config.RateLimitWindow))
		resetTime = now.Add(timeNeeded)
	} else {
		resetTime = now.Add(rl.config.RateLimitWindow)
	}

	return true, remaining, resetTime
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := splitString(forwarded, ",")
		if len(ips) > 0 {
			return trimString(ips[0])
		}
	}

	// Check X-Real-IP header
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// Helper functions
func formatInt(i int) string {
	return strconv.Itoa(i)
}

func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

func splitString(s, sep string) []string {
	return strings.Split(s, sep)
}

func trimString(s string) string {
	return strings.TrimSpace(s)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

