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

		// Log CORS requests for debugging
		if origin != "" {
			c.logger.Debug("CORS request",
				"method", r.Method,
				"origin", origin,
				"path", r.URL.Path,
			)
		}

		// Handle preflight OPTIONS requests
		if r.Method == "OPTIONS" {
			c.handlePreflight(w, r, origin)
			return
		}

		// Handle actual requests - set CORS headers
		allowedOrigin := c.getAllowedOrigin(origin)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			// Only set credentials header if credentials are enabled AND not using wildcard
			// (wildcard with credentials is insecure and should be disabled by config validation)
			if c.config.CORSAllowCreds && allowedOrigin != "*" {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			// Set exposed headers on actual responses (not just preflight)
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type, X-Request-ID")
			c.logger.Debug("CORS: Allowed origin",
				"origin", origin,
				"allowed_origin", allowedOrigin,
				"credentials", c.config.CORSAllowCreds,
			)
		} else if origin != "" {
			// Log when origin is provided but not allowed (for debugging)
			c.logger.Warn("CORS: Origin not allowed",
				"origin", origin,
				"allowed_origins", c.config.CORSOrigins,
				"path", r.URL.Path,
			)
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
		// Log for debugging
		c.logger.Debug("CORS: Preflight rejected - origin not allowed",
			"origin", origin,
			"allowed_origins", c.config.CORSOrigins,
		)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Set CORS headers for preflight
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	// Only set credentials header if credentials are enabled AND not using wildcard
	// (wildcard with credentials is insecure and should be disabled by config validation)
	if c.config.CORSAllowCreds && allowedOrigin != "*" {
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
			// Echo back the requested headers (normalized)
			normalizedHeaders := c.normalizeHeaders(requestedHeaders)
			w.Header().Set("Access-Control-Allow-Headers", normalizedHeaders)
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

	// Handle null/empty origin (file:// protocol or same-origin requests)
	// For same-origin requests, CORS doesn't apply, but we still need to handle them gracefully
	// For file:// protocol, we allow it if wildcard is enabled
	if origin == "" || origin == "null" {
		// Check if wildcard is enabled and credentials are disabled (required for wildcard)
		for _, allowed := range c.config.CORSOrigins {
			if allowed == "*" && !c.config.CORSAllowCreds {
				return "*"
			}
		}
		// If no wildcard or credentials enabled, reject null origin
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

// normalizeHeaders normalizes header names (lowercase, trimmed) and returns them as a comma-separated string
func (c *CORSMiddleware) normalizeHeaders(headers string) string {
	parts := strings.Split(headers, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	return strings.Join(normalized, ", ")
}

// formatDuration formats a duration as seconds for HTTP headers
func formatDuration(d time.Duration) string {
	seconds := int64(d.Seconds())
	if seconds <= 0 {
		return "0"
	}
	return strconv.FormatInt(seconds, 10)
}

