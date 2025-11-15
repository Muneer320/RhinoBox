package middleware

import (
	"bytes"
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

// Handler returns the middleware handler function
func (r *RequestSizeLimitMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if r.config.MaxRequestSize <= 0 {
			next.ServeHTTP(w, req)
			return
		}

		// Check Content-Length header first
		if req.ContentLength > r.config.MaxRequestSize {
			http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Limit request body size
		limitedReader := io.LimitReader(req.Body, r.config.MaxRequestSize+1)
		bodyBytes := make([]byte, 0, 1024)
		buf := make([]byte, 32*1024) // 32KB buffer

		for {
			n, err := limitedReader.Read(buf)
			if n > 0 {
				bodyBytes = append(bodyBytes, buf[:n]...)
				if int64(len(bodyBytes)) > r.config.MaxRequestSize {
					http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
					return
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusBadRequest)
				return
			}
		}

		// Replace body with the read bytes
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))

		next.ServeHTTP(w, req)
	})
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

