package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// ResponseWriter wraps http.ResponseWriter to capture response details
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
	startTime  time.Time
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		startTime:      time.Now(),
	}
}

// WriteHeader captures the status code
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response body
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return rw.ResponseWriter.Write(b)
}

// StatusCode returns the captured status code
func (rw *ResponseWriter) StatusCode() int {
	return rw.statusCode
}

// Body returns the captured response body
func (rw *ResponseWriter) Body() []byte {
	return rw.body
}

// Duration returns the request duration
func (rw *ResponseWriter) Duration() time.Duration {
	return time.Since(rw.startTime)
}

// ResponseConfig holds configuration for response middleware
type ResponseConfig struct {
	Logger           *slog.Logger
	EnableLogging    bool
	EnableCORS       bool
	CORSOrigins      []string
	DefaultPageSize  int
	MaxPageSize      int
}

// DefaultResponseConfig returns a default configuration
func DefaultResponseConfig(logger *slog.Logger) *ResponseConfig {
	return &ResponseConfig{
		Logger:          logger,
		EnableLogging:   true,
		EnableCORS:      true,
		CORSOrigins:     []string{"*"},
		DefaultPageSize: 20,
		MaxPageSize:     100,
	}
}

// ResponseMiddleware provides standardized response handling
type ResponseMiddleware struct {
	config *ResponseConfig
}

// NewResponseMiddleware creates a new response middleware
func NewResponseMiddleware(config *ResponseConfig) *ResponseMiddleware {
	if config == nil {
		config = DefaultResponseConfig(slog.Default())
	}
	return &ResponseMiddleware{
		config: config,
	}
}

// Handler wraps an http.Handler with response transformation
func (rm *ResponseMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := NewResponseWriter(w)
		
		// Set common headers
		rm.setCommonHeaders(rw, r)
		
		// Execute the next handler
		next.ServeHTTP(rw, r)
		
		// Log response if enabled
		if rm.config.EnableLogging {
			rm.logResponse(r, rw)
		}
	})
}

// setCommonHeaders sets standard HTTP headers
func (rm *ResponseMiddleware) setCommonHeaders(w http.ResponseWriter, r *http.Request) {
	// Set Content-Type for JSON responses (can be overridden by handlers)
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	
	// Set CORS headers if enabled
	if rm.config.EnableCORS {
		origin := r.Header.Get("Origin")
		if origin != "" || len(rm.config.CORSOrigins) > 0 {
			allowedOrigin := rm.getAllowedOrigin(origin)
			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}
		}
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	
	// Set security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	
	// Set cache control for API responses (can be overridden)
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}
}

// getAllowedOrigin determines the allowed origin for CORS
func (rm *ResponseMiddleware) getAllowedOrigin(origin string) string {
	if len(rm.config.CORSOrigins) == 0 {
		return ""
	}
	
	// Allow all origins if "*" is specified
	for _, allowed := range rm.config.CORSOrigins {
		if allowed == "*" {
			return "*"
		}
		if allowed == origin {
			return origin
		}
	}
	
	// Return first origin as fallback if no match
	if len(rm.config.CORSOrigins) > 0 {
		return rm.config.CORSOrigins[0]
	}
	
	return ""
}

// logResponse logs the response details
func (rm *ResponseMiddleware) logResponse(r *http.Request, rw *ResponseWriter) {
	duration := rw.Duration()
	statusCode := rw.StatusCode()
	
	// Determine log level based on status code
	level := slog.LevelInfo
	if statusCode >= 400 && statusCode < 500 {
		level = slog.LevelWarn
	} else if statusCode >= 500 {
		level = slog.LevelError
	}
	
	// Log response details
	rm.config.Logger.Log(r.Context(), level,
		"response",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.Int("status", statusCode),
		slog.Duration("duration", duration),
		slog.String("remote_addr", r.RemoteAddr),
		slog.String("user_agent", r.UserAgent()),
	)
}

// WrapHandler is a convenience function to wrap a handler with response middleware
func WrapHandler(handler http.Handler, config *ResponseConfig) http.Handler {
	mw := NewResponseMiddleware(config)
	return mw.Handler(handler)
}

