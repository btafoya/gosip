# Security Fix: CORS Configuration

## Summary

Fixed **MEDIUM** severity security vulnerability where CORS was configured with `AllowedOrigins: "*"` combined with `AllowCredentials: true`, allowing any website to make authenticated requests to the GoSIP API.

## Changes Made

### 1. Configuration Layer (`/home/btafoya/projects/gosip/internal/config/config.go`)

**Added:**
- `CORSOrigins []string` field to `Config` struct
- `getEnvStringSlice()` helper function to parse comma-separated environment variables
- Supporting string parsing functions: `splitAndTrim()`, `splitString()`, `trimSpace()`, `isSpace()`

**Default Values:**
```go
CORSOrigins: []string{
    "http://localhost:3000",
    "http://localhost:8080",
    "http://127.0.0.1:3000",
    "http://127.0.0.1:8080",
}
```

**Environment Variable:**
- `GOSIP_CORS_ORIGINS` - Comma-separated list of allowed origins
- Example: `GOSIP_CORS_ORIGINS="https://gosip.example.com,https://admin.example.com"`

### 2. Router Layer (`/home/btafoya/projects/gosip/internal/api/router.go`)

**Changed:**
```go
// Before (VULNERABLE)
AllowedOrigins: []string{"*"},

// After (SECURE)
AllowedOrigins: deps.Config.CORSOrigins,
```

## Security Impact

### Before Fix
- **Risk**: Any website could make authenticated API requests on behalf of logged-in users
- **Attack Vector**: CSRF attacks via malicious websites
- **Severity**: MEDIUM
- **Exploitability**: HIGH (simple JavaScript fetch from any domain)

### After Fix
- **Risk Mitigation**: Only explicitly whitelisted origins can make authenticated requests
- **Default Security**: Localhost-only access by default (safe for development)
- **Production Ready**: Configurable via environment variable for production deployments

## Usage

### Development (Default)
No configuration needed. The following origins are allowed by default:
- `http://localhost:3000`
- `http://localhost:8080`
- `http://127.0.0.1:3000`
- `http://127.0.0.1:8080`

### Production Deployment

Set the `GOSIP_CORS_ORIGINS` environment variable to your production frontend URLs:

**Docker Compose:**
```yaml
environment:
  - GOSIP_CORS_ORIGINS=https://gosip.example.com,https://admin.example.com
```

**Docker Run:**
```bash
docker run -e GOSIP_CORS_ORIGINS="https://gosip.example.com" gosip:latest
```

**Systemd Service:**
```ini
[Service]
Environment="GOSIP_CORS_ORIGINS=https://gosip.example.com,https://admin.example.com"
```

### Multiple Origins
Origins should be comma-separated with optional whitespace:
```bash
GOSIP_CORS_ORIGINS="https://example.com, https://www.example.com, https://admin.example.com"
```

## Testing

### Build Verification
```bash
go build ./...
```
✅ Compiles successfully

### Runtime Verification
Check CORS headers in browser DevTools Network tab:
- Authenticated requests from allowed origins: ✅ Succeed
- Authenticated requests from unauthorized origins: ❌ Blocked by browser

### Manual Testing
```bash
# Test allowed origin
curl -H "Origin: http://localhost:3000" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: X-Requested-With" \
     -X OPTIONS http://localhost:8080/api/auth/login

# Should return: Access-Control-Allow-Origin: http://localhost:3000

# Test disallowed origin
curl -H "Origin: https://evil.com" \
     -H "Access-Control-Request-Method: POST" \
     -H "Access-Control-Request-Headers: X-Requested-With" \
     -X OPTIONS http://localhost:8080/api/auth/login

# Should NOT return Access-Control-Allow-Origin header
```

## Implementation Notes

### Why Custom String Parsing?
The implementation includes custom string parsing functions (`splitString`, `trimSpace`, etc.) rather than using standard library `strings.Split()` and `strings.TrimSpace()`. This appears to be a design choice to avoid additional imports, though using the standard library would be equally valid and more idiomatic.

### Alternative Implementation
If desired, the parsing could be simplified using standard library functions:

```go
import "strings"

func getEnvStringSlice(key string, defaultValue []string) []string {
    if value := os.Getenv(key); value != "" {
        parts := make([]string, 0)
        for _, part := range strings.Split(value, ",") {
            trimmed := strings.TrimSpace(part)
            if trimmed != "" {
                parts = append(parts, trimmed)
            }
        }
        if len(parts) > 0 {
            return parts
        }
    }
    return defaultValue
}
```

## References

- [OWASP CORS Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Origin_Resource_Sharing_Security_Cheat_Sheet.html)
- [MDN: CORS and Credentials](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#requests_with_credentials)
- CWE-942: Permissive Cross-domain Policy with Untrusted Domains

## Files Modified

1. `/home/btafoya/projects/gosip/internal/config/config.go`
   - Added `CORSOrigins` field to `Config` struct
   - Added `getEnvStringSlice()` and supporting string parsing functions
   - Set secure default values for development

2. `/home/btafoya/projects/gosip/internal/api/router.go`
   - Changed `AllowedOrigins` from `[]string{"*"}` to `deps.Config.CORSOrigins`
   - Updated comment to reflect secure configuration
