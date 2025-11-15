package config

import (
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	// CORS configuration
	CORSEnabled      bool
	CORSOrigins      []string
	CORSAllowMethods []string
	CORSAllowHeaders []string
	CORSMaxAge       time.Duration
	CORSAllowCreds   bool

	// Security headers
	SecurityHeadersEnabled bool
	ContentTypeOptions     string
	FrameOptions           string
	XSSProtection          string
	ReferrerPolicy         string
	PermissionsPolicy      string
	StrictTransportSecurity string

	// Rate limiting
	RateLimitEnabled    bool
	RateLimitRequests   int           // requests per window
	RateLimitWindow     time.Duration // time window
	RateLimitBurst      int           // burst allowance
	RateLimitByIP       bool
	RateLimitByEndpoint bool

	// Request size limits
	MaxRequestSize int64 // max request body size in bytes

	// IP filtering
	IPWhitelistEnabled bool
	IPWhitelist        []net.IPNet
	IPBlacklistEnabled bool
	IPBlacklist        []net.IPNet
}

// LoadSecurityConfig loads security configuration from environment variables
func LoadSecurityConfig() SecurityConfig {
	cfg := SecurityConfig{
		// CORS defaults
		CORSEnabled:      getBoolEnv("RHINOBOX_CORS_ENABLED", true),
		CORSOrigins:      getStringSliceEnv("RHINOBOX_CORS_ORIGINS", []string{"*"}),
		CORSAllowMethods: getStringSliceEnv("RHINOBOX_CORS_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		CORSAllowHeaders: getStringSliceEnv("RHINOBOX_CORS_HEADERS", []string{"Content-Type", "Authorization", "X-Requested-With"}),
		CORSMaxAge:       getDurationEnv("RHINOBOX_CORS_MAX_AGE", 3600*time.Second),
		CORSAllowCreds:   getBoolEnv("RHINOBOX_CORS_CREDENTIALS", true),

		// Security headers defaults
		SecurityHeadersEnabled:     getBoolEnv("RHINOBOX_SECURITY_HEADERS_ENABLED", true),
		ContentTypeOptions:          getEnv("RHINOBOX_HEADER_CONTENT_TYPE_OPTIONS", "nosniff"),
		FrameOptions:                getEnv("RHINOBOX_HEADER_FRAME_OPTIONS", "DENY"),
		XSSProtection:               getEnv("RHINOBOX_HEADER_XSS_PROTECTION", "1; mode=block"),
		ReferrerPolicy:              getEnv("RHINOBOX_HEADER_REFERRER_POLICY", "strict-origin-when-cross-origin"),
		PermissionsPolicy:           getEnv("RHINOBOX_HEADER_PERMISSIONS_POLICY", "geolocation=(), microphone=(), camera=()"),
		StrictTransportSecurity:     getEnv("RHINOBOX_HEADER_HSTS", ""), // empty = disabled by default

		// Rate limiting defaults
		RateLimitEnabled:    getBoolEnv("RHINOBOX_RATE_LIMIT_ENABLED", true),
		RateLimitRequests:   getIntEnv("RHINOBOX_RATE_LIMIT_REQUESTS", 100),
		RateLimitWindow:     getDurationEnv("RHINOBOX_RATE_LIMIT_WINDOW", 1*time.Minute),
		RateLimitBurst:      getIntEnv("RHINOBOX_RATE_LIMIT_BURST", 10),
		RateLimitByIP:       getBoolEnv("RHINOBOX_RATE_LIMIT_BY_IP", true),
		RateLimitByEndpoint: getBoolEnv("RHINOBOX_RATE_LIMIT_BY_ENDPOINT", false),

		// Request size limits (default 10MB, separate from upload size)
		MaxRequestSize: getInt64Env("RHINOBOX_MAX_REQUEST_SIZE", 10*1024*1024),

		// IP filtering defaults
		IPWhitelistEnabled: getBoolEnv("RHINOBOX_IP_WHITELIST_ENABLED", false),
		IPBlacklistEnabled: getBoolEnv("RHINOBOX_IP_BLACKLIST_ENABLED", false),
	}

	// Parse IP whitelist
	if cfg.IPWhitelistEnabled {
		cfg.IPWhitelist = parseIPList(getEnv("RHINOBOX_IP_WHITELIST", ""))
	}

	// Parse IP blacklist
	if cfg.IPBlacklistEnabled {
		cfg.IPBlacklist = parseIPList(getEnv("RHINOBOX_IP_BLACKLIST", ""))
	}

	return cfg
}

// getBoolEnv reads a boolean environment variable
func getBoolEnv(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}

// getIntEnv reads an integer environment variable
func getIntEnv(key string, defaultValue int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// getInt64Env reads an int64 environment variable
func getInt64Env(key string, defaultValue int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return i
}

// getDurationEnv reads a duration environment variable (in seconds)
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	seconds, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return time.Duration(seconds) * time.Second
}

// getStringSliceEnv reads a comma-separated string slice environment variable
func getStringSliceEnv(key string, defaultValue []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}

// parseIPList parses a comma-separated list of IP addresses or CIDR ranges
func parseIPList(list string) []net.IPNet {
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

