package middleware

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"

	"github.com/Muneer320/RhinoBox/internal/config"
)

// RequestSizeLimitMiddleware enforces maximum request body size limits
type RequestSizeLimitMiddleware struct {
	config config.SecurityConfig
	logger *slog.Logger
}

// NewRequestSizeLimitMiddleware creates a new request size limit middleware instance
func NewRequestSizeLimitMiddleware(cfg config.SecurityConfig, logger *slog.Logger) *RequestSizeLimitMiddleware {
	return &RequestSizeLimitMiddleware{
		config: cfg,
		logger: logger,
	}
}

// Handler limits reads as they happen instead of buffering uploads in memory.
func (r *RequestSizeLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodOptions || r.config.MaxRequestSize <= 0 {
			next.ServeHTTP(w, req)
			return
		}
		if req.ContentLength > r.config.MaxRequestSize {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			fmt.Fprintf(w, `{"error":"Request entity too large","max_bytes":%d}`, r.config.MaxRequestSize)
			return
		}
		if req.Body == nil || req.Body == http.NoBody {
			next.ServeHTTP(w, req)
			return
		}
		body := &limitedRequestBody{ReadCloser: http.MaxBytesReader(w, req.Body, r.config.MaxRequestSize)}
		req.Body = body
		next.ServeHTTP(&limitStatusWriter{ResponseWriter: w, body: body}, req)
	})
}

type limitedRequestBody struct {
	io.ReadCloser
	exceeded bool
}

func (b *limitedRequestBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	var limitError *http.MaxBytesError
	if errors.As(err, &limitError) {
		b.exceeded = true
	}
	return n, err
}

// Preserve the correct status for handlers reporting a bounded-read error.
type limitStatusWriter struct {
	http.ResponseWriter
	body        *limitedRequestBody
	wroteHeader bool
}

func (w *limitStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *limitStatusWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	if w.body.exceeded {
		status = http.StatusRequestEntityTooLarge
	}
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *limitStatusWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// IPFilterMiddleware filters requests based on IP whitelist/blacklist
type IPFilterMiddleware struct {
	config config.SecurityConfig
	logger *slog.Logger
}

// NewIPFilterMiddleware creates a new IP filter middleware instance
func NewIPFilterMiddleware(cfg config.SecurityConfig, logger *slog.Logger) *IPFilterMiddleware {
	return &IPFilterMiddleware{
		config: cfg,
		logger: logger,
	}
}

// Handler returns the middleware handler function
func (i *IPFilterMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		// Check blacklist first (takes precedence)
		if i.config.IPBlacklistEnabled {
			if i.isIPBlocked(clientIP, i.config.IPBlacklist) {
				if i.logger != nil {
					i.logger.Warn("request blocked by IP blacklist",
						"ip", clientIP,
						"path", r.URL.Path,
					)
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		// Check whitelist
		if i.config.IPWhitelistEnabled {
			if !i.isIPAllowed(clientIP, i.config.IPWhitelist) {
				if i.logger != nil {
					i.logger.Warn("request blocked - IP not in whitelist",
						"ip", clientIP,
						"path", r.URL.Path,
					)
				}
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}

		// Continue with the request
		next.ServeHTTP(w, r)
	})
}

// isIPBlocked checks if an IP is in the blacklist
func (i *IPFilterMiddleware) isIPBlocked(ipStr string, blacklist []net.IPNet) bool {
	ip := parseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, ipnet := range blacklist {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// isIPAllowed checks if an IP is in the whitelist
func (i *IPFilterMiddleware) isIPAllowed(ipStr string, whitelist []net.IPNet) bool {
	ip := parseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, ipnet := range whitelist {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}

// parseIP parses an IP address string, handling port numbers
func parseIP(ipStr string) net.IP {
	// Remove port if present
	host, _, err := net.SplitHostPort(ipStr)
	if err == nil {
		ipStr = host
	}

	ip := net.ParseIP(ipStr)
	return ip
}
