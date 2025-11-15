package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Muneer320/RhinoBox/internal/config"
)

// CORSMiddleware handles Cross-Origin Resource Sharing (CORS) requests
type CORSMiddleware struct {
	config config.SecurityConfig
	logger *slog.Logger
}

// NewCORSMiddleware creates a new CORS middleware instance
func NewCORSMiddleware(cfg config.SecurityConfig, logger *slog.Logger) *CORSMiddleware {
	return &CORSMiddleware{
		config: cfg,
		logger: logger,
	}
}

// Handler returns the middleware handler function
func (c *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !c.config.CORSEnabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Handle preflight OPTIONS requests
		if r.Method == "OPTIONS" {
			c.handlePreflight(w, r, origin)
			return
		}

		// Handle actual requests - set CORS headers
		allowedOrigin := c.getAllowedOrigin(origin)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			if c.config.CORSAllowCreds {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// Set exposed headers on actual responses (not just preflight)
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Request-ID")
		}

		// Continue with the request
		next.ServeHTTP(w, r)
	})
}

// handlePreflight handles CORS preflight OPTIONS requests
func (c *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request, origin string) {
	allowedOrigin := c.getAllowedOrigin(origin)
	if allowedOrigin == "" {
		// Origin not allowed, reject preflight
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Set CORS headers for preflight
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	if c.config.CORSAllowCreds {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	// Set allowed methods
	if len(c.config.CORSAllowMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(c.config.CORSAllowMethods, ", "))
	}

	// Set allowed headers
	requestedHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestedHeaders != "" {
		// Use requested headers if they match our allowed headers, otherwise use configured
		if c.isHeaderAllowed(requestedHeaders) {
			w.Header().Set("Access-Control-Allow-Headers", requestedHeaders)
		} else if len(c.config.CORSAllowHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.CORSAllowHeaders, ", "))
		}
	} else if len(c.config.CORSAllowHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(c.config.CORSAllowHeaders, ", "))
	}

	// Set max age
	if c.config.CORSMaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", formatDuration(c.config.CORSMaxAge))
	}

	// Set exposed headers if needed
	w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Request-ID")

	w.WriteHeader(http.StatusNoContent)
}

// getAllowedOrigin determines if the origin is allowed and returns the allowed origin value
func (c *CORSMiddleware) getAllowedOrigin(origin string) string {
	if len(c.config.CORSOrigins) == 0 {
		return ""
	}

	// Check for wildcard
	for _, allowed := range c.config.CORSOrigins {
		if allowed == "*" {
			// Wildcard allows all origins, but credentials can't be used with wildcard
			// Never reflect arbitrary origins when credentials are enabled (security issue)
			if !c.config.CORSAllowCreds {
				return "*"
			}
			// If credentials are enabled, we should never reach here due to config validation,
			// but as a safety measure, return empty string to reject the request
			return ""
		}
		if allowed == origin {
			return origin
		}
	}

	// No match found
	return ""
}

// isHeaderAllowed checks if a requested header is in the allowed list
func (c *CORSMiddleware) isHeaderAllowed(requestedHeaders string) bool {
	requested := strings.Split(requestedHeaders, ",")
	allowedMap := make(map[string]bool)
	for _, h := range c.config.CORSAllowHeaders {
		allowedMap[strings.ToLower(strings.TrimSpace(h))] = true
	}

	for _, req := range requested {
		req = strings.ToLower(strings.TrimSpace(req))
		if !allowedMap[req] {
			return false
		}
	}
	return true
}

// formatDuration formats a duration as seconds for HTTP headers
func formatDuration(d time.Duration) string {
	seconds := int64(d.Seconds())
	if seconds <= 0 {
		return "0"
	}
	return strconv.FormatInt(seconds, 10)
}

