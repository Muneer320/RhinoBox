package middleware

import (
	"log/slog"
	"net/http"

	"github.com/Muneer320/RhinoBox/internal/config"
)

// SecurityHeadersMiddleware sets security-related HTTP headers
type SecurityHeadersMiddleware struct {
	config config.SecurityConfig
	logger *slog.Logger
}

// NewSecurityHeadersMiddleware creates a new security headers middleware instance
func NewSecurityHeadersMiddleware(cfg config.SecurityConfig, logger *slog.Logger) *SecurityHeadersMiddleware {
	return &SecurityHeadersMiddleware{
		config: cfg,
		logger: logger,
	}
}

// Handler returns the middleware handler function
func (s *SecurityHeadersMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.SecurityHeadersEnabled {
			next.ServeHTTP(w, r)
			return
		}

		// Set X-Content-Type-Options
		if s.config.ContentTypeOptions != "" {
			w.Header().Set("X-Content-Type-Options", s.config.ContentTypeOptions)
		}

		// Set X-Frame-Options
		if s.config.FrameOptions != "" {
			w.Header().Set("X-Frame-Options", s.config.FrameOptions)
		}

		// Set X-XSS-Protection
		if s.config.XSSProtection != "" {
			w.Header().Set("X-XSS-Protection", s.config.XSSProtection)
		}

		// Set Referrer-Policy
		if s.config.ReferrerPolicy != "" {
			w.Header().Set("Referrer-Policy", s.config.ReferrerPolicy)
		}

		// Set Permissions-Policy
		if s.config.PermissionsPolicy != "" {
			w.Header().Set("Permissions-Policy", s.config.PermissionsPolicy)
		}

		// Set Strict-Transport-Security (HSTS) - only for HTTPS
		if s.config.StrictTransportSecurity != "" && r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", s.config.StrictTransportSecurity)
		}

		// Continue with the request
		next.ServeHTTP(w, r)
	})
}

