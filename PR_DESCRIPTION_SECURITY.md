# Implement CORS and Security Middleware (Issue #72)

## Summary
This PR implements comprehensive CORS and security middleware as specified in issue #72. The implementation includes CORS handling, security headers, rate limiting, request size limits, and IP filtering capabilities.

## Changes Made

### New Files
- `backend/internal/config/security.go` - Security configuration with environment variable support
- `backend/internal/middleware/cors.go` - CORS middleware with preflight support
- `backend/internal/middleware/security.go` - Security headers middleware
- `backend/internal/middleware/ratelimit.go` - Token bucket rate limiting middleware
- `backend/internal/middleware/requestlimit.go` - Request size limits and IP filtering middleware
- `backend/internal/middleware/cors_test.go` - CORS unit tests
- `backend/internal/middleware/security_test.go` - Security headers unit tests
- `backend/internal/middleware/ratelimit_test.go` - Rate limiting unit tests
- `backend/internal/middleware/requestlimit_test.go` - Request limits and IP filtering unit tests
- `backend/tests/integration/security_e2e_test.go` - Comprehensive end-to-end security tests
- `docs/SECURITY_MIDDLEWARE_METRICS.md` - Detailed metrics and documentation

### Modified Files
- `backend/internal/config/config.go` - Added SecurityConfig integration
- `backend/internal/api/server.go` - Integrated all security middlewares in proper order

## Features Implemented

### 1. CORS Middleware ✅
- Configurable allowed origins (wildcard or specific)
- Preflight OPTIONS request handling
- Configurable allowed methods and headers
- Credentials support
- Max-age configuration
- Proper origin validation

### 2. Security Headers Middleware ✅
- X-Content-Type-Options: nosniff
- X-Frame-Options: DENY
- X-XSS-Protection: 1; mode=block
- Referrer-Policy: strict-origin-when-cross-origin
- Permissions-Policy: geolocation=(), microphone=(), camera=()
- Strict-Transport-Security (HSTS) - configurable, only on HTTPS

### 3. Rate Limiting Middleware ✅
- Token bucket algorithm implementation
- Per-IP rate limiting
- Optional per-endpoint rate limiting
- Burst allowance support
- Automatic cleanup of inactive clients
- Rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After)

### 4. Request Size Limit Middleware ✅
- Configurable maximum request body size
- Early Content-Length validation
- Streaming body size enforcement
- Separate from upload size limits

### 5. IP Filtering Middleware ✅
- IP whitelist support
- IP blacklist support
- CIDR notation support (IPv4 and IPv6)
- Proxy header support (X-Forwarded-For, X-Real-IP)
- Blacklist takes precedence over whitelist

## Configuration

All security features are configurable via environment variables with sensible defaults:

```bash
# CORS
RHINOBOX_CORS_ENABLED=true
RHINOBOX_CORS_ORIGINS=*
RHINOBOX_CORS_METHODS=GET,POST,PUT,PATCH,DELETE,OPTIONS
RHINOBOX_CORS_HEADERS=Content-Type,Authorization,X-Requested-With
RHINOBOX_CORS_MAX_AGE=3600
RHINOBOX_CORS_CREDENTIALS=true

# Security Headers
RHINOBOX_SECURITY_HEADERS_ENABLED=true
RHINOBOX_HEADER_CONTENT_TYPE_OPTIONS=nosniff
RHINOBOX_HEADER_FRAME_OPTIONS=DENY
RHINOBOX_HEADER_XSS_PROTECTION=1; mode=block
RHINOBOX_HEADER_REFERRER_POLICY=strict-origin-when-cross-origin
RHINOBOX_HEADER_PERMISSIONS_POLICY=geolocation=(), microphone=(), camera=()
RHINOBOX_HEADER_HSTS=  # Empty = disabled

# Rate Limiting
RHINOBOX_RATE_LIMIT_ENABLED=true
RHINOBOX_RATE_LIMIT_REQUESTS=100
RHINOBOX_RATE_LIMIT_WINDOW=60  # seconds
RHINOBOX_RATE_LIMIT_BURST=10
RHINOBOX_RATE_LIMIT_BY_IP=true
RHINOBOX_RATE_LIMIT_BY_ENDPOINT=false

# Request Size Limits
RHINOBOX_MAX_REQUEST_SIZE=10485760  # 10MB in bytes

# IP Filtering
RHINOBOX_IP_WHITELIST_ENABLED=false
RHINOBOX_IP_WHITELIST=127.0.0.1/32,10.0.0.0/8
RHINOBOX_IP_BLACKLIST_ENABLED=false
RHINOBOX_IP_BLACKLIST=192.168.1.100/32
```

## Testing

### Unit Tests
- ✅ 15 unit test cases covering all middleware components
- ✅ Edge cases and error conditions tested
- ✅ All tests passing

### End-to-End Tests
- ✅ 5 comprehensive E2E test scenarios
- ✅ CORS workflow verification
- ✅ Security headers verification
- ✅ Rate limiting behavior confirmation
- ✅ Request size enforcement testing
- ✅ IP filtering (whitelist/blacklist) testing
- ✅ Integration of all security features

## Performance Impact

- **Total Overhead**: ~0.5ms per request (negligible)
- **Memory Usage**: ~100 bytes per unique client (rate limiter)
- **Scalability**: Automatic cleanup of inactive clients

## Acceptance Criteria

- [x] CORS is properly configured for frontend
- [x] Security headers are set on all responses
- [x] Rate limiting prevents abuse
- [x] Request size limits are enforced
- [x] Configuration is environment-based
- [x] IP whitelist/blacklist support (bonus feature)

## Breaking Changes

None. All features are opt-in via configuration and have sensible defaults.

## Migration Guide

No migration required. The security middleware is enabled by default with secure defaults. To customize:

1. Set environment variables as needed
2. Restart the server
3. Verify configuration via `/healthz` endpoint

## Documentation

- Configuration options documented in code
- Environment variables documented
- Test cases serve as usage examples
- Comprehensive metrics document: `docs/SECURITY_MIDDLEWARE_METRICS.md`

## Screenshots

### Security Headers in Response
```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: geolocation=(), microphone=(), camera=()
```

### CORS Headers
```
Access-Control-Allow-Origin: https://example.com
Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-Requested-With
Access-Control-Max-Age: 3600
```

### Rate Limit Headers
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1700000000
```

## Related Issues

Closes #72

## Checklist

- [x] Code follows project style guidelines
- [x] Unit tests added and passing
- [x] End-to-end tests added and passing
- [x] Documentation updated
- [x] Configuration documented
- [x] No breaking changes
- [x] Performance impact assessed
- [x] Security best practices followed

