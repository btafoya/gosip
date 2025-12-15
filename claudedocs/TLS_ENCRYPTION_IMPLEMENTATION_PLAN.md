# GoSIP TLS/Encryption Implementation Plan

## Executive Summary

This plan details adding encrypted communications to GoSIP, covering:
1. **TLS for SIP signaling** (SIPS on port 5061)
2. **Automatic certificate management** via Let's Encrypt with Cloudflare DNS
3. **Optional SRTP** for media encryption

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         GoSIP Server                            │
├──────────────┬──────────────┬──────────────┬───────────────────┤
│  UDP:5060    │  TCP:5060    │  TLS:5061    │  WSS:5081         │
│  (SIP)       │  (SIP)       │  (SIPS)      │  (WebSocket/TLS)  │
├──────────────┴──────────────┴──────────────┴───────────────────┤
│                     Certificate Manager                         │
│              (CertMagic + Cloudflare DNS-01)                    │
├─────────────────────────────────────────────────────────────────┤
│                   Media Encryption (Optional)                   │
│                        pion/srtp                                │
└─────────────────────────────────────────────────────────────────┘
```

---

## Phase 1: TLS Configuration Options

### 1.1 Configuration Structure Updates

**File**: `internal/config/config.go`

```go
// TLSConfig holds TLS-specific configuration
type TLSConfig struct {
    // Enabled enables TLS/SIPS support
    Enabled bool

    // Port for SIPS (default: 5061)
    Port int

    // WSSPort for WebSocket Secure (default: 5081)
    WSSPort int

    // CertMode: "manual" | "acme"
    CertMode string

    // Manual certificate paths (when CertMode = "manual")
    CertFile string
    KeyFile  string
    CAFile   string // Optional CA certificate for client verification

    // ACME/Let's Encrypt settings (when CertMode = "acme")
    ACMEEmail   string
    ACMEDomain  string   // Primary domain for certificate
    ACMEDomains []string // Additional SANs
    ACMECA      string   // "production" | "staging"

    // Cloudflare DNS challenge settings
    CloudflareAPIToken string

    // Client certificate verification
    ClientAuth string // "none" | "request" | "require"

    // Minimum TLS version: "1.2" | "1.3"
    MinVersion string
}

// SRTPConfig holds SRTP-specific configuration (optional)
type SRTPConfig struct {
    Enabled bool
    Profile string // "AES_CM_128_HMAC_SHA1_80" | "AEAD_AES_128_GCM"
}
```

### 1.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GOSIP_TLS_ENABLED` | Enable TLS support | `false` |
| `GOSIP_TLS_PORT` | SIPS port | `5061` |
| `GOSIP_TLS_WSS_PORT` | WSS port | `5081` |
| `GOSIP_TLS_CERT_MODE` | `manual` or `acme` | `acme` |
| `GOSIP_TLS_CERT_FILE` | Path to certificate (manual mode) | - |
| `GOSIP_TLS_KEY_FILE` | Path to private key (manual mode) | - |
| `GOSIP_TLS_CA_FILE` | Path to CA cert (optional) | - |
| `GOSIP_ACME_EMAIL` | Email for Let's Encrypt | - |
| `GOSIP_ACME_DOMAIN` | Primary domain | - |
| `GOSIP_ACME_CA` | `production` or `staging` | `staging` |
| `CLOUDFLARE_DNS_API_TOKEN` | Cloudflare API token for DNS-01 | - |
| `GOSIP_TLS_MIN_VERSION` | Minimum TLS version | `1.2` |
| `GOSIP_SRTP_ENABLED` | Enable SRTP | `false` |

### 1.3 Database Schema Addition

**File**: `migrations/XXX_add_tls_config.sql`

```sql
-- Add TLS configuration to config table
ALTER TABLE config ADD COLUMN tls_enabled INTEGER DEFAULT 0;
ALTER TABLE config ADD COLUMN tls_cert_mode TEXT DEFAULT 'acme';
ALTER TABLE config ADD COLUMN tls_cert_file TEXT;
ALTER TABLE config ADD COLUMN tls_key_file TEXT;
ALTER TABLE config ADD COLUMN acme_email TEXT;
ALTER TABLE config ADD COLUMN acme_domain TEXT;
ALTER TABLE config ADD COLUMN cloudflare_api_token TEXT;
ALTER TABLE config ADD COLUMN srtp_enabled INTEGER DEFAULT 0;
```

---

## Phase 2: TLS Listener Implementation

### 2.1 Dependencies

Add to `go.mod`:

```go
require (
    github.com/caddyserver/certmagic v0.21.0
    github.com/libdns/cloudflare v0.1.1
    github.com/pion/srtp/v2 v2.0.18  // Optional, for SRTP
)
```

### 2.2 Certificate Manager

**New File**: `pkg/sip/certmanager.go`

```go
package sip

import (
    "context"
    "crypto/tls"
    "fmt"
    "sync"

    "github.com/caddyserver/certmagic"
    "github.com/libdns/cloudflare"
)

// CertManager handles TLS certificate lifecycle
type CertManager struct {
    config    *TLSConfig
    tlsConfig *tls.Config
    magic     *certmagic.Config
    mu        sync.RWMutex
}

// NewCertManager creates a certificate manager
func NewCertManager(cfg *TLSConfig) (*CertManager, error) {
    cm := &CertManager{config: cfg}

    if cfg.CertMode == "manual" {
        return cm.initManual()
    }
    return cm.initACME()
}

// initManual loads certificates from files
func (cm *CertManager) initManual() (*CertManager, error) {
    cert, err := tls.LoadX509KeyPair(cm.config.CertFile, cm.config.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("load certificate: %w", err)
    }

    cm.tlsConfig = &tls.Config{
        Certificates: []tls.Certificate{cert},
        MinVersion:   tls.VersionTLS12,
    }

    return cm, nil
}

// initACME sets up automatic certificate management
func (cm *CertManager) initACME() (*CertManager, error) {
    // Configure Cloudflare DNS provider
    cfProvider := &cloudflare.Provider{
        APIToken: cm.config.CloudflareAPIToken,
    }

    // Configure ACME settings
    certmagic.DefaultACME.Agreed = true
    certmagic.DefaultACME.Email = cm.config.ACMEEmail
    certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
        DNSManager: certmagic.DNSManager{
            DNSProvider: cfProvider,
        },
    }

    // Set CA based on configuration
    if cm.config.ACMECA == "production" {
        certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
    } else {
        certmagic.DefaultACME.CA = certmagic.LetsEncryptStagingCA
    }

    // Create CertMagic config
    cm.magic = certmagic.NewDefault()

    // Build domain list
    domains := []string{cm.config.ACMEDomain}
    domains = append(domains, cm.config.ACMEDomains...)

    // Obtain certificates (async to not block startup)
    ctx := context.Background()
    if err := cm.magic.ManageAsync(ctx, domains); err != nil {
        return nil, fmt.Errorf("certmagic manage: %w", err)
    }

    // Get TLS config from CertMagic
    cm.tlsConfig = cm.magic.TLSConfig()
    cm.tlsConfig.MinVersion = tls.VersionTLS12

    return cm, nil
}

// GetTLSConfig returns the current TLS configuration
func (cm *CertManager) GetTLSConfig() *tls.Config {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    return cm.tlsConfig
}

// Close cleans up resources
func (cm *CertManager) Close() error {
    // CertMagic handles cleanup automatically
    return nil
}
```

### 2.3 Server Updates

**File**: `pkg/sip/server.go` (modifications)

```go
// Config holds SIP server configuration
type Config struct {
    Port       int
    UserAgent  string
    MOHEnabled bool
    MOHPath    string

    // TLS configuration
    TLS *TLSConfig
}

// Server wraps sipgo server with GoSIP-specific functionality
type Server struct {
    // ... existing fields ...

    certMgr *CertManager // Certificate manager for TLS
}

// NewServer creates a new SIP server
func NewServer(cfg Config, database *db.DB) (*Server, error) {
    // ... existing initialization ...

    // Initialize certificate manager if TLS enabled
    if cfg.TLS != nil && cfg.TLS.Enabled {
        certMgr, err := NewCertManager(cfg.TLS)
        if err != nil {
            return nil, fmt.Errorf("init cert manager: %w", err)
        }
        server.certMgr = certMgr
    }

    return server, nil
}

// Start begins listening for SIP messages
func (s *Server) Start(ctx context.Context) error {
    // ... existing mutex check ...

    // Register handlers
    s.srv.OnRegister(s.handleRegister)
    s.srv.OnInvite(s.handleInvite)
    // ... other handlers ...

    // Start UDP listener (unencrypted)
    addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)
    go func() {
        slog.Info("Starting SIP UDP listener", "addr", addr)
        if err := s.srv.ListenAndServe(ctx, "udp", addr); err != nil {
            slog.Error("SIP UDP listener error", "error", err)
        }
    }()

    // Start TCP listener (unencrypted)
    go func() {
        slog.Info("Starting SIP TCP listener", "addr", addr)
        if err := s.srv.ListenAndServe(ctx, "tcp", addr); err != nil {
            slog.Error("SIP TCP listener error", "error", err)
        }
    }()

    // Start TLS listener if enabled
    if s.certMgr != nil {
        tlsAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.TLS.Port)
        tlsConfig := s.certMgr.GetTLSConfig()

        go func() {
            slog.Info("Starting SIP TLS listener", "addr", tlsAddr)
            if err := s.srv.ListenAndServeTLS(ctx, "tcp", tlsAddr, tlsConfig); err != nil {
                slog.Error("SIP TLS listener error", "error", err)
            }
        }()

        // Start WSS listener if configured
        if s.cfg.TLS.WSSPort > 0 {
            wssAddr := fmt.Sprintf("0.0.0.0:%d", s.cfg.TLS.WSSPort)
            go func() {
                slog.Info("Starting SIP WSS listener", "addr", wssAddr)
                if err := s.srv.ListenAndServeTLS(ctx, "ws", wssAddr, tlsConfig); err != nil {
                    slog.Error("SIP WSS listener error", "error", err)
                }
            }()
        }
    }

    // ... existing cleanup goroutines ...

    return nil
}
```

---

## Phase 3: Certificate Management

### 3.1 Cloudflare API Token Requirements

Create a Cloudflare API Token with these permissions:
- **Zone:DNS:Edit** - For creating/deleting DNS-01 challenge records
- **Zone:Zone:Read** - For listing zones (optional, for auto zone detection)

### 3.2 Certificate Storage

CertMagic stores certificates in `~/.local/share/certmagic` by default. For GoSIP, configure:

```go
// In initACME()
certmagic.Default.Storage = &certmagic.FileStorage{
    Path: filepath.Join(dataDir, "certs"),
}
```

This stores certificates in `data/certs/` alongside the database.

### 3.3 Web UI for Certificate Management

**New API Endpoints**:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/system/tls` | Get TLS configuration status |
| PUT | `/api/system/tls` | Update TLS configuration |
| POST | `/api/system/tls/test` | Test TLS configuration |
| GET | `/api/system/tls/certificate` | Get certificate details (expiry, issuer, SANs) |
| POST | `/api/system/tls/certificate/renew` | Force certificate renewal |

**File**: `internal/api/tls.go`

```go
package api

import (
    "encoding/json"
    "net/http"
)

type TLSStatus struct {
    Enabled       bool     `json:"enabled"`
    CertMode      string   `json:"certMode"`
    Domain        string   `json:"domain"`
    Domains       []string `json:"domains"`
    CertExpiry    string   `json:"certExpiry,omitempty"`
    CertIssuer    string   `json:"certIssuer,omitempty"`
    AutoRenewal   bool     `json:"autoRenewal"`
    LastRenewal   string   `json:"lastRenewal,omitempty"`
    NextRenewal   string   `json:"nextRenewal,omitempty"`
}

func (h *Handler) GetTLSStatus(w http.ResponseWriter, r *http.Request) {
    // Retrieve TLS status from cert manager
    // Return certificate details, expiry dates, etc.
}

func (h *Handler) UpdateTLSConfig(w http.ResponseWriter, r *http.Request) {
    // Update TLS configuration
    // Trigger certificate reissuance if domain changes
}

func (h *Handler) RenewCertificate(w http.ResponseWriter, r *http.Request) {
    // Force certificate renewal
}
```

---

## Phase 4: SRTP Support (Optional)

### 4.1 SRTP Integration with pion/srtp

SRTP encrypts the media stream (voice data), separate from TLS which encrypts signaling.

**New File**: `pkg/sip/srtp.go`

```go
package sip

import (
    "github.com/pion/srtp/v2"
)

// SRTPConfig holds SRTP configuration
type SRTPConfig struct {
    Enabled         bool
    Profile         srtp.ProtectionProfile
    MasterKey       []byte
    MasterSalt      []byte
}

// SRTPContext wraps pion/srtp for media encryption
type SRTPContext struct {
    encryptCtx *srtp.Context
    decryptCtx *srtp.Context
}

// NewSRTPContext creates SRTP encryption/decryption contexts
func NewSRTPContext(cfg *SRTPConfig) (*SRTPContext, error) {
    profile := srtp.ProtectionProfileAes128CmHmacSha1_80
    if cfg.Profile == srtp.ProtectionProfileAeadAes128Gcm {
        profile = srtp.ProtectionProfileAeadAes128Gcm
    }

    encryptCtx, err := srtp.CreateContext(cfg.MasterKey, cfg.MasterSalt, profile)
    if err != nil {
        return nil, err
    }

    decryptCtx, err := srtp.CreateContext(cfg.MasterKey, cfg.MasterSalt, profile)
    if err != nil {
        return nil, err
    }

    return &SRTPContext{
        encryptCtx: encryptCtx,
        decryptCtx: decryptCtx,
    }, nil
}

// EncryptRTP encrypts an RTP packet
func (s *SRTPContext) EncryptRTP(dst, src []byte) ([]byte, error) {
    return s.encryptCtx.EncryptRTP(dst, src, nil)
}

// DecryptRTP decrypts an SRTP packet
func (s *SRTPContext) DecryptRTP(dst, src []byte) ([]byte, error) {
    return s.decryptCtx.DecryptRTP(dst, src, nil)
}
```

### 4.2 SDP Negotiation for SRTP

SRTP requires SDP negotiation with crypto attributes:

```
m=audio 49170 RTP/SAVP 0
a=crypto:1 AES_CM_128_HMAC_SHA1_80 inline:WVNfX19zZW1jdGwgKCkgewkyMjA7fQp9
```

This requires modifying call setup to:
1. Generate SRTP keys
2. Include crypto attribute in SDP offer
3. Parse crypto attribute from SDP answer
4. Initialize SRTP contexts for the call

---

## Implementation Timeline

### Sprint 1: Foundation (Week 1-2)
- [ ] Add TLS configuration to `internal/config/`
- [ ] Add database migration for TLS settings
- [ ] Implement `CertManager` for manual certificates
- [ ] Add TLS listener to SIP server

### Sprint 2: ACME Integration (Week 3-4)
- [ ] Integrate CertMagic with Cloudflare DNS provider
- [ ] Implement certificate storage in data directory
- [ ] Add certificate status API endpoints
- [ ] Add certificate renewal functionality

### Sprint 3: Web UI (Week 5)
- [ ] Create TLS configuration UI in Vue
- [ ] Add certificate status display
- [ ] Add setup wizard step for TLS

### Sprint 4: SRTP (Week 6, Optional)
- [ ] Integrate pion/srtp
- [ ] Implement SDP crypto negotiation
- [ ] Add SRTP toggle in configuration

### Sprint 5: Testing & Documentation (Week 7)
- [ ] Test with Grandstream GXP1760W
- [ ] Test with various softphones (Onesip, Zoiper)
- [ ] Update documentation
- [ ] Performance testing

---

## Testing Checklist

### TLS Signaling Tests
- [ ] TLS handshake with valid certificate
- [ ] TLS handshake with self-signed certificate
- [ ] Certificate expiry handling
- [ ] Auto-renewal 30 days before expiry
- [ ] Client certificate verification (optional)
- [ ] TLS 1.2 minimum enforcement

### Device Compatibility
- [ ] Grandstream GXP1760W SIPS registration
- [ ] Onesip iOS with TLS
- [ ] Zoiper with TLS
- [ ] Twilio SIP trunk TLS

### Certificate Management
- [ ] Let's Encrypt staging issuance
- [ ] Let's Encrypt production issuance
- [ ] Cloudflare DNS-01 challenge
- [ ] Certificate renewal
- [ ] Manual certificate upload

---

## Security Considerations

1. **API Token Security**: Store Cloudflare API token encrypted in database
2. **Key Storage**: Private keys stored with restrictive permissions (0600)
3. **TLS Version**: Enforce TLS 1.2 minimum, prefer TLS 1.3
4. **Cipher Suites**: Use modern, secure cipher suites only
5. **Certificate Validation**: Validate Let's Encrypt certificates on renewal

---

## File Changes Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/config/config.go` | Modify | Add TLSConfig struct |
| `internal/config/constants.go` | Modify | Add TLS defaults |
| `pkg/sip/certmanager.go` | Create | Certificate lifecycle management |
| `pkg/sip/server.go` | Modify | Add TLS listener support |
| `pkg/sip/srtp.go` | Create | SRTP encryption (optional) |
| `internal/api/tls.go` | Create | TLS API endpoints |
| `internal/api/routes.go` | Modify | Register TLS routes |
| `migrations/XXX_add_tls.sql` | Create | Database schema |
| `frontend/src/views/TLSSettings.vue` | Create | TLS configuration UI |
| `go.mod` | Modify | Add certmagic, libdns/cloudflare |

---

## References

- [sipgo TLS Documentation](https://github.com/emiago/sipgo)
- [CertMagic Documentation](https://github.com/caddyserver/certmagic)
- [libdns/cloudflare](https://github.com/libdns/cloudflare)
- [pion/srtp](https://github.com/pion/srtp)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
- [Cloudflare API Tokens](https://developers.cloudflare.com/api/tokens/)
