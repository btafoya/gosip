# GoSIP Implementation Plan

Detailed implementation plan based on Context7 library documentation and shadcn-vue component patterns.

---

## Technology Reference Summary

| Component | Library | Context7 ID | Key Patterns |
|-----------|---------|-------------|--------------|
| SIP Server | sipgo | `/emiago/sipgo` | UA, Server, Client, DialogServerCache, Digest Auth |
| Twilio API | twilio-go | `/twilio/twilio-go` | Trunks API, Messages API, TwiML |
| Database | go-sqlite3 | `/mattn/go-sqlite3` | database/sql interface, migrations |
| Frontend | Vue 3 | `/vuejs/docs` | Composition API, Pinia, Vue Router |
| UI Components | shadcn-vue | `/llmstxt/shadcn-vue_llms-full_txt` | DataTable, Form, Dialog, Sidebar |
| Styling | Tailwind CSS | `/websites/tailwindcss` | Utility classes, responsive design |

---

## Phase 1: Project Foundation

**Goal**: Establish project structure, build tooling, Docker environment, and P0 configuration constants

### 1.0 P0 Configuration Constants

These values were defined during specification review and MUST be implemented:

```go
// internal/config/constants.go
package config

// Performance SLAs
const (
    SIPRegistrationTimeout = 500 * time.Millisecond  // < 500ms
    CallSetupTimeout       = 2 * time.Second         // < 2 seconds
    APIGetTimeout          = 200 * time.Millisecond  // < 200ms (95th percentile)
    APIPostTimeout         = 500 * time.Millisecond  // < 500ms (95th percentile)
    MaxConcurrentCalls     = 5
    SystemStartupTimeout   = 30 * time.Second
)

// Security
const (
    MaxFailedLoginAttempts = 5
    LoginLockoutDuration   = 15 * time.Minute
    SessionDuration        = 24 * time.Hour
    SessionRefreshOnActivity = true
    SpamScoreThreshold     = 0.7  // Calls > 0.7 blocked
)

// Voicemail
const (
    VoicemailRingTimeout   = 30 * time.Second
    VoicemailMaxLength     = 180 * time.Second  // 3 minutes
    VoicemailMinLength     = 3 * time.Second    // Shorter discarded
    VoicemailSilenceTimeout = 10 * time.Second
)

// API
const (
    DefaultPageSize = 20
    MaxPageSize     = 100
)

// Retry/Recovery
const (
    TwilioMaxRetries       = 3
    TwilioRetryBackoff     = true  // Exponential backoff
    EmailMaxRetries        = 3
    EmailRetryWindow       = 1 * time.Hour
    GotifyMaxRetries       = 3
)
```

### 1.1 Go Backend Scaffolding

```bash
# Initialize Go module
go mod init github.com/btafoya/gosip

# Project structure
mkdir -p cmd/gosip
mkdir -p internal/{api,auth,config,db,models,rules,twilio,webhooks}
mkdir -p pkg/sip
mkdir -p migrations
```

**Tasks:**
- [ ] Create `cmd/gosip/main.go` entry point
- [ ] Set up Go module with dependencies
- [ ] Create `internal/config/constants.go` with P0 values (see 1.0 above)
- [ ] Configure air or similar for hot reload
- [ ] Create Makefile with common commands

**Dependencies (go.mod):**
```go
require (
    github.com/emiago/sipgo v0.21.0
    github.com/twilio/twilio-go v1.20.0
    github.com/mattn/go-sqlite3 v1.14.22
    github.com/go-chi/chi/v5 v5.0.12
    github.com/golang-jwt/jwt/v5 v5.2.0
    golang.org/x/crypto v0.21.0
)
```

### 1.2 Vue Frontend Scaffolding

```bash
cd frontend
pnpm create vite@latest . --template vue-ts
pnpm add -D tailwindcss postcss autoprefixer
pnpm add @tanstack/vue-table pinia vue-router
npx tailwindcss init -p
```

**Tasks:**
- [ ] Initialize Vite + Vue 3 + TypeScript
- [ ] Configure Tailwind CSS
- [ ] Add shadcn-vue via CLI: `npx shadcn-vue@latest init`
- [ ] Set up Pinia store structure
- [ ] Configure Vue Router

**shadcn-vue Components to Install:**
```bash
npx shadcn-vue@latest add button card dialog dropdown-menu form input
npx shadcn-vue@latest add label select sidebar table tabs toast
npx shadcn-vue@latest add badge checkbox command navigation-menu
npx shadcn-vue@latest add sheet scroll-area separator avatar
```

### 1.3 Docker Environment

**Tasks:**
- [ ] Create `Dockerfile` (multi-stage build)
- [ ] Create `docker-compose.yml`
- [ ] Set up volume mounts for data persistence
- [ ] Configure port mappings (5060/UDP, 5060/TCP, 8080)

```yaml
# docker-compose.yml
services:
  gosip:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "5060:5060/udp"
      - "5060:5060/tcp"
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - GOSIP_DATA_DIR=/app/data
    restart: unless-stopped
```

---

## Phase 2: Database Layer

**Goal**: SQLite database with migrations and repository pattern

### 2.1 Schema Migrations

**Tasks:**
- [ ] Create migration system (golang-migrate or custom)
- [ ] Write initial schema migration (see REQUIREMENTS.md)
- [ ] Write performance indexes migration
- [ ] Implement migration runner in main.go

**Migration Files:**
```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_add_indexes.up.sql
└── 002_add_indexes.down.sql
```

**001_initial_schema.up.sql** - Core tables from REQUIREMENTS.md:
```sql
-- System configuration (key-value)
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Admin and user accounts
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT CHECK(role IN ('admin', 'user')) NOT NULL DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- Registered SIP devices
CREATE TABLE devices (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    device_type TEXT CHECK(device_type IN ('grandstream', 'softphone', 'webrtc')),
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Active SIP registrations
CREATE TABLE registrations (
    id INTEGER PRIMARY KEY,
    device_id INTEGER REFERENCES devices(id),
    contact TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    transport TEXT CHECK(transport IN ('udp', 'tcp', 'tls', 'ws', 'wss')),
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Phone numbers (DIDs)
CREATE TABLE dids (
    id INTEGER PRIMARY KEY,
    number TEXT UNIQUE NOT NULL,
    twilio_sid TEXT,
    name TEXT,
    sms_enabled BOOLEAN DEFAULT FALSE,
    voice_enabled BOOLEAN DEFAULT TRUE
);

-- Call routing rules
CREATE TABLE routes (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id),
    priority INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    condition_type TEXT CHECK(condition_type IN ('time', 'callerid', 'default')),
    condition_data JSON,
    action_type TEXT CHECK(action_type IN ('ring', 'forward', 'voicemail', 'reject')),
    action_data JSON,
    enabled BOOLEAN DEFAULT TRUE
);

-- Blocked numbers
CREATE TABLE blocklist (
    id INTEGER PRIMARY KEY,
    pattern TEXT NOT NULL,
    pattern_type TEXT CHECK(pattern_type IN ('exact', 'prefix', 'regex')),
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Call detail records
CREATE TABLE cdrs (
    id INTEGER PRIMARY KEY,
    call_sid TEXT UNIQUE,
    direction TEXT CHECK(direction IN ('inbound', 'outbound')),
    from_number TEXT NOT NULL,
    to_number TEXT NOT NULL,
    did_id INTEGER REFERENCES dids(id),
    device_id INTEGER REFERENCES devices(id),
    started_at DATETIME NOT NULL,
    answered_at DATETIME,
    ended_at DATETIME,
    duration INTEGER DEFAULT 0,
    disposition TEXT CHECK(disposition IN ('answered', 'voicemail', 'missed', 'blocked', 'busy', 'failed')),
    recording_url TEXT,
    spam_score REAL
);

-- Voicemails
CREATE TABLE voicemails (
    id INTEGER PRIMARY KEY,
    cdr_id INTEGER REFERENCES cdrs(id),
    user_id INTEGER REFERENCES users(id),
    from_number TEXT NOT NULL,
    audio_url TEXT,
    transcript TEXT,
    duration INTEGER DEFAULT 0,
    is_read BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- SMS/MMS messages
CREATE TABLE messages (
    id INTEGER PRIMARY KEY,
    message_sid TEXT UNIQUE,
    direction TEXT CHECK(direction IN ('inbound', 'outbound')),
    from_number TEXT NOT NULL,
    to_number TEXT NOT NULL,
    did_id INTEGER REFERENCES dids(id),
    body TEXT,
    media_urls JSON,
    status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

-- Auto-reply rules
CREATE TABLE auto_replies (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id),
    trigger_type TEXT CHECK(trigger_type IN ('dnd', 'after_hours', 'keyword')),
    trigger_data JSON,
    reply_text TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE
);
```

**002_add_indexes.up.sql** - Performance indexes (P0 requirement):
```sql
-- CDR indexes for call history queries
CREATE INDEX idx_cdrs_started ON cdrs(started_at DESC);
CREATE INDEX idx_cdrs_disposition ON cdrs(disposition);
CREATE INDEX idx_cdrs_did ON cdrs(did_id);

-- Message indexes for SMS history
CREATE INDEX idx_messages_created ON messages(created_at DESC);
CREATE INDEX idx_messages_did ON messages(did_id);

-- Voicemail indexes
CREATE INDEX idx_voicemails_user ON voicemails(user_id);
CREATE INDEX idx_voicemails_read ON voicemails(is_read);

-- Registration indexes for SIP operations
CREATE INDEX idx_registrations_device ON registrations(device_id);
CREATE INDEX idx_registrations_expires ON registrations(expires_at);

-- Route lookup index
CREATE INDEX idx_routes_did_priority ON routes(did_id, priority);
```

**002_add_indexes.down.sql**:
```sql
DROP INDEX IF EXISTS idx_cdrs_started;
DROP INDEX IF EXISTS idx_cdrs_disposition;
DROP INDEX IF EXISTS idx_cdrs_did;
DROP INDEX IF EXISTS idx_messages_created;
DROP INDEX IF EXISTS idx_messages_did;
DROP INDEX IF EXISTS idx_voicemails_user;
DROP INDEX IF EXISTS idx_voicemails_read;
DROP INDEX IF EXISTS idx_registrations_device;
DROP INDEX IF EXISTS idx_registrations_expires;
DROP INDEX IF EXISTS idx_routes_did_priority;
```

### 2.2 Repository Layer

**Pattern**: Repository per domain with interface abstraction

```go
// internal/db/repository.go
type Repository struct {
    db *sql.DB
}

type UserRepository interface {
    Create(ctx context.Context, user *models.User) error
    GetByID(ctx context.Context, id int64) (*models.User, error)
    GetByEmail(ctx context.Context, email string) (*models.User, error)
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id int64) error
}
```

**Tasks:**
- [ ] Create `internal/db/db.go` - connection pool setup
- [ ] Create `internal/db/users.go` - UserRepository
- [ ] Create `internal/db/devices.go` - DeviceRepository
- [ ] Create `internal/db/dids.go` - DIDRepository
- [ ] Create `internal/db/routes.go` - RouteRepository
- [ ] Create `internal/db/cdrs.go` - CDRRepository
- [ ] Create `internal/db/messages.go` - MessageRepository
- [ ] Create `internal/db/voicemails.go` - VoicemailRepository
- [ ] Create `internal/db/config.go` - ConfigRepository

---

## Phase 3: SIP Server (sipgo Integration)

**Goal**: Functional SIP server with registration and call handling

### 3.1 Core SIP Server

**Reference Pattern (from Context7 sipgo docs):**
```go
// pkg/sip/server.go
package sip

import (
    "context"
    "github.com/emiago/sipgo"
    "github.com/emiago/sipgo/sip"
)

type Server struct {
    ua     *sipgo.UserAgent
    srv    *sipgo.Server
    client *sipgo.Client
    registrar *Registrar
}

func NewServer(cfg Config) (*Server, error) {
    ua, err := sipgo.NewUA(sipgo.WithUserAgent("GoSIP/1.0"))
    if err != nil {
        return nil, err
    }

    srv, err := sipgo.NewServer(ua)
    if err != nil {
        return nil, err
    }

    client, err := sipgo.NewClient(ua)
    if err != nil {
        return nil, err
    }

    return &Server{ua: ua, srv: srv, client: client}, nil
}

func (s *Server) Start(ctx context.Context) error {
    // Register handlers
    s.srv.OnRegister(s.handleRegister)
    s.srv.OnInvite(s.handleInvite)
    s.srv.OnAck(s.handleAck)
    s.srv.OnBye(s.handleBye)
    s.srv.OnCancel(s.handleCancel)
    s.srv.OnOptions(s.handleOptions)

    // Start listeners
    go s.srv.ListenAndServe(ctx, "udp", "0.0.0.0:5060")
    go s.srv.ListenAndServe(ctx, "tcp", "0.0.0.0:5060")

    return nil
}
```

**Tasks:**
- [ ] Create `pkg/sip/server.go` - main SIP server
- [ ] Create `pkg/sip/handlers.go` - request handlers
- [ ] Create `pkg/sip/auth.go` - Digest authentication
- [ ] Create `pkg/sip/registrar.go` - registration management

### 3.2 Digest Authentication

**Reference Pattern:**
```go
// pkg/sip/auth.go
func (s *Server) authenticateRequest(req *sip.Request) (*models.Device, error) {
    authHeader := req.GetHeader("Authorization")
    if authHeader == nil {
        return nil, ErrNoCredentials
    }

    // Parse Digest auth
    username, realm, nonce, uri, response := parseDigestAuth(authHeader.Value())

    // Look up device
    device, err := s.deviceRepo.GetByUsername(ctx, username)
    if err != nil {
        return nil, ErrDeviceNotFound
    }

    // Validate response
    ha1 := md5Hash(fmt.Sprintf("%s:%s:%s", username, realm, device.Password))
    ha2 := md5Hash(fmt.Sprintf("%s:%s", req.Method, uri))
    expected := md5Hash(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))

    if response != expected {
        return nil, ErrInvalidCredentials
    }

    return device, nil
}
```

**Tasks:**
- [ ] Implement MD5 Digest authentication (RFC 2617)
- [ ] Challenge/response flow for 401 Unauthorized
- [ ] Nonce generation and validation
- [ ] Integration with device repository

### 3.3 Registration Management

**Tasks:**
- [ ] Track active registrations with expiry
- [ ] Handle REGISTER requests
- [ ] Update registration on refresh
- [ ] Clean up expired registrations (background goroutine)
- [ ] Emit registration events for UI updates

### 3.4 SIP Failure Handling (P0 Requirement)

**SIP device failure handling per REQUIREMENTS.md:**

```go
// pkg/sip/call_state.go
package sip

import (
    "context"
    "time"
)

// ActiveCall tracks call state for resilience
type ActiveCall struct {
    CallID       string
    DeviceID     int64
    StartedAt    time.Time
    LastActivity time.Time
    State        CallState
}

type CallState int

const (
    CallStateActive CallState = iota
    CallStateOnHold
    CallStateTransferring
)

// CallManager maintains call state during device issues
type CallManager struct {
    calls map[string]*ActiveCall
    mu    sync.RWMutex
}

// HandleDeviceOffline maintains call until BYE or timeout
// Per REQUIREMENTS.md: "Maintain call until BYE/timeout"
func (cm *CallManager) HandleDeviceOffline(deviceID int64) {
    cm.mu.Lock()
    defer cm.mu.Unlock()

    for _, call := range cm.calls {
        if call.DeviceID == deviceID {
            // Don't terminate - wait for explicit BYE or timeout
            // Twilio side maintains the call
            call.LastActivity = time.Now()
        }
    }
}

// CheckCallTimeout monitors for stale calls
func (cm *CallManager) CheckCallTimeout(ctx context.Context, timeout time.Duration) {
    ticker := time.NewTicker(10 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cm.mu.Lock()
            now := time.Now()
            for callID, call := range cm.calls {
                if now.Sub(call.LastActivity) > timeout {
                    // Call timed out, clean up
                    cm.terminateCall(callID)
                }
            }
            cm.mu.Unlock()
        }
    }
}
```

**Failure Mode Response:**

| Component | Failure Mode | Response | Recovery |
|-----------|--------------|----------|----------|
| SIP Device | Offline mid-call | Maintain call until BYE/timeout | Auto-reregister on reconnect |

**Tasks:**
- [ ] Create `pkg/sip/call_state.go` - active call tracking
- [ ] Implement call maintenance during device offline
- [ ] Add timeout-based call cleanup
- [ ] Handle device re-registration after disconnect

---

## Phase 4: Twilio Integration

**Goal**: Connect to Twilio for SIP trunking and messaging

### 4.1 Twilio Client

**Reference Pattern (from Context7 twilio-go docs):**
```go
// internal/twilio/client.go
package twilio

import (
    "github.com/twilio/twilio-go"
    twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
    trunkingApi "github.com/twilio/twilio-go/rest/trunking/v1"
)

type Client struct {
    api *twilio.RestClient
}

func NewClient(accountSid, authToken string) *Client {
    client := twilio.NewRestClientWithParams(twilio.ClientParams{
        Username: accountSid,
        Password: authToken,
    })
    return &Client{api: client}
}

// SendSMS sends an SMS/MMS message
func (c *Client) SendSMS(from, to, body string, mediaUrls []string) (*twilioApi.ApiV2010Message, error) {
    params := &twilioApi.CreateMessageParams{}
    params.SetFrom(from)
    params.SetTo(to)
    params.SetBody(body)
    if len(mediaUrls) > 0 {
        params.SetMediaUrl(mediaUrls)
    }

    return c.api.Api.CreateMessage(params)
}
```

**Tasks:**
- [ ] Create `internal/twilio/client.go` - base client
- [ ] Create `internal/twilio/sms.go` - SMS/MMS operations
- [ ] Create `internal/twilio/trunking.go` - SIP trunk management
- [ ] Create `internal/twilio/twiml.go` - TwiML generation

### 4.2 Failure Handling & Resilience (P0 Requirement)

**Twilio API failure handling per REQUIREMENTS.md:**

```go
// internal/twilio/retry.go
package twilio

import (
    "context"
    "time"
    "github.com/btafoya/gosip/internal/config"
)

// RetryConfig uses P0 constants from config package
type RetryConfig struct {
    MaxRetries int
    BaseDelay  time.Duration
    MaxDelay   time.Duration
}

var DefaultRetryConfig = RetryConfig{
    MaxRetries: config.TwilioMaxRetries,  // 3
    BaseDelay:  1 * time.Second,
    MaxDelay:   30 * time.Second,
}

// RetryWithBackoff implements exponential backoff for Twilio API calls
func (c *Client) RetryWithBackoff(ctx context.Context, operation func() error) error {
    var lastErr error
    delay := DefaultRetryConfig.BaseDelay

    for attempt := 0; attempt <= DefaultRetryConfig.MaxRetries; attempt++ {
        if err := operation(); err == nil {
            return nil
        } else {
            lastErr = err

            // Check for rate limiting (429)
            if isRateLimited(err) {
                delay = getRetryAfterDelay(err, delay)
            }

            // Alert admin after 3 failures
            if attempt == DefaultRetryConfig.MaxRetries-1 {
                c.alertAdmin("Twilio API failures", lastErr)
            }
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(delay):
            delay = min(delay*2, DefaultRetryConfig.MaxDelay) // Exponential backoff
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// QueuedRequest represents a request to retry later
type QueuedRequest struct {
    ID        string
    Type      string      // "sms", "call", "recording"
    Payload   interface{}
    Attempts  int
    CreatedAt time.Time
    NextRetry time.Time
}

// RequestQueue handles queuing requests during Twilio outages
type RequestQueue struct {
    db    *sql.DB
    mu    sync.Mutex
    queue []QueuedRequest
}

func (q *RequestQueue) Enqueue(req QueuedRequest) error {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.queue = append(q.queue, req)
    return q.persist(req)
}

func (q *RequestQueue) ProcessQueue(ctx context.Context, client *Client) {
    // Background goroutine to retry queued requests
    ticker := time.NewTicker(30 * time.Second)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            q.processReadyRequests(ctx, client)
        }
    }
}
```

**Failure Mode Response Matrix:**

| Component | Failure Mode | Response | Recovery |
|-----------|--------------|----------|----------|
| Twilio API | Unreachable/5xx | Queue requests, alert admin after 3 failures | Auto-retry with exponential backoff |
| Twilio API | Rate limited (429) | Backoff and queue | Auto-retry after cooldown |

**Tasks:**
- [ ] Create `internal/twilio/retry.go` - retry logic with exponential backoff
- [ ] Create `internal/twilio/queue.go` - request queue for outages
- [ ] Implement admin alerting after 3 consecutive failures
- [ ] Add rate limit detection (HTTP 429) with Retry-After parsing

### 4.3 Webhook Handlers

**Tasks:**
- [ ] Create `internal/webhooks/voice.go` - voice webhooks
- [ ] Create `internal/webhooks/sms.go` - SMS webhooks
- [ ] Create `internal/webhooks/recording.go` - recording webhooks
- [ ] Create `internal/webhooks/transcription.go` - transcription webhooks
- [ ] Implement Twilio signature validation

---

## Phase 5: REST API

**Goal**: Complete REST API for frontend consumption

### 5.1 API Structure

**Pattern**: Chi router with middleware chain

```go
// internal/api/router.go
func NewRouter(deps *Dependencies) chi.Router {
    r := chi.NewRouter()

    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(cors.Handler(cors.Options{...}))

    // Public routes
    r.Post("/api/auth/login", deps.AuthHandler.Login)
    r.Post("/api/webhooks/voice/incoming", deps.WebhookHandler.VoiceIncoming)
    r.Post("/api/webhooks/sms/incoming", deps.WebhookHandler.SMSIncoming)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(deps.AuthMiddleware)

        r.Route("/api/devices", func(r chi.Router) {
            r.Get("/", deps.DeviceHandler.List)
            r.Post("/", deps.DeviceHandler.Create)
            r.Get("/{id}", deps.DeviceHandler.Get)
            r.Put("/{id}", deps.DeviceHandler.Update)
            r.Delete("/{id}", deps.DeviceHandler.Delete)
        })

        // ... more routes
    })

    return r
}
```

**Tasks:**
- [ ] Create `internal/api/router.go` - main router
- [ ] Create `internal/api/middleware.go` - auth middleware
- [ ] Create `internal/api/auth.go` - auth handlers
- [ ] Create `internal/api/devices.go` - device handlers
- [ ] Create `internal/api/dids.go` - DID handlers
- [ ] Create `internal/api/routes.go` - route handlers
- [ ] Create `internal/api/cdrs.go` - CDR handlers
- [ ] Create `internal/api/voicemails.go` - voicemail handlers
- [ ] Create `internal/api/messages.go` - message handlers
- [ ] Create `internal/api/system.go` - system handlers

### 5.2 Authentication & Security (P0 Requirements)

**Authentication with fail-safe handling per REQUIREMENTS.md:**

```go
// internal/auth/session.go
package auth

import (
    "time"
    "github.com/btafoya/gosip/internal/config"
)

// Session configuration using P0 constants
type SessionConfig struct {
    Duration          time.Duration // 24 hours
    RefreshOnActivity bool          // true
}

var DefaultSessionConfig = SessionConfig{
    Duration:          config.SessionDuration,         // 24 * time.Hour
    RefreshOnActivity: config.SessionRefreshOnActivity, // true
}

// LoginAttemptTracker prevents brute force attacks
type LoginAttemptTracker struct {
    attempts map[string][]time.Time
    mu       sync.RWMutex
}

// CheckAndRecord returns true if login should be allowed
func (t *LoginAttemptTracker) CheckAndRecord(ip string) (bool, time.Duration) {
    t.mu.Lock()
    defer t.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-1 * time.Minute)

    // Clean old attempts
    var recent []time.Time
    for _, attempt := range t.attempts[ip] {
        if attempt.After(cutoff) {
            recent = append(recent, attempt)
        }
    }
    t.attempts[ip] = recent

    // Check if locked out
    if len(recent) >= config.MaxFailedLoginAttempts { // 5
        lockoutEnd := recent[0].Add(config.LoginLockoutDuration) // 15 min
        if now.Before(lockoutEnd) {
            return false, lockoutEnd.Sub(now)
        }
        // Lockout expired, reset
        t.attempts[ip] = nil
    }

    // Record this attempt
    t.attempts[ip] = append(t.attempts[ip], now)
    return true, 0
}
```

**API Standard Error Response (per REQUIREMENTS.md):**

```go
// internal/api/errors.go
package api

import (
    "encoding/json"
    "net/http"
)

// ErrorResponse follows REQUIREMENTS.md standard format
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details []FieldError   `json:"details,omitempty"`
}

type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

// Standard error codes
const (
    ErrCodeValidation     = "VALIDATION_ERROR"
    ErrCodeAuthentication = "AUTHENTICATION_ERROR"
    ErrCodeAuthorization  = "AUTHORIZATION_ERROR"
    ErrCodeNotFound       = "NOT_FOUND"
    ErrCodeRateLimited    = "RATE_LIMITED"
    ErrCodeInternal       = "INTERNAL_ERROR"
)

func WriteError(w http.ResponseWriter, statusCode int, code, message string, details []FieldError) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(ErrorResponse{
        Error: ErrorDetail{
            Code:    code,
            Message: message,
            Details: details,
        },
    })
}

// Example usage:
// WriteError(w, 400, ErrCodeValidation, "Invalid input", []FieldError{{Field: "email", Message: "Invalid format"}})
```

**Failure Mode Response Matrix:**

| Component | Failure Mode | Response | Recovery |
|-----------|--------------|----------|----------|
| Auth | 5 failed attempts/min | 15-minute IP lockout | Automatic after cooldown |
| Session | Expired | 401 with clear message | Re-login required |
| Validation | Invalid input | 400 with field details | Client correction |

**Tasks:**
- [ ] Session-based auth with secure cookies
- [ ] Bcrypt password hashing
- [ ] Rate limiting on auth endpoints (5 attempts/min → 15-min lockout)
- [ ] Session timeout handling (24-hour duration, refresh on activity)
- [ ] Standard error response format implementation
- [ ] Login attempt tracking with IP-based lockout

---

## Phase 6: Frontend - Core Structure

**Goal**: Vue 3 application foundation with routing and state management

### 6.1 Application Structure

```
frontend/src/
├── api/                 # API client
│   ├── client.ts
│   ├── auth.ts
│   ├── devices.ts
│   └── ...
├── components/
│   ├── ui/              # shadcn-vue components
│   ├── layout/          # Layout components
│   │   ├── AppSidebar.vue
│   │   ├── AppHeader.vue
│   │   └── AppLayout.vue
│   └── shared/          # Shared components
├── composables/         # Vue composables
├── stores/              # Pinia stores
│   ├── auth.ts
│   ├── devices.ts
│   └── ...
├── views/               # Page components
│   ├── auth/
│   ├── admin/
│   └── user/
├── router/              # Vue Router
└── lib/                 # Utilities
```

**Tasks:**
- [ ] Set up API client with axios
- [ ] Configure Vue Router with guards
- [ ] Create Pinia stores
- [ ] Build layout components

### 6.2 Pinia Stores

**Pattern (from Context7 Vue docs):**
```typescript
// stores/auth.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '@/api/auth'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(email: string, password: string) {
    const response = await authApi.login(email, password)
    user.value = response.user
  }

  async function logout() {
    await authApi.logout()
    user.value = null
  }

  return { user, isAuthenticated, isAdmin, login, logout }
})
```

**Tasks:**
- [ ] Create `stores/auth.ts`
- [ ] Create `stores/devices.ts`
- [ ] Create `stores/dids.ts`
- [ ] Create `stores/cdrs.ts`
- [ ] Create `stores/messages.ts`
- [ ] Create `stores/voicemails.ts`
- [ ] Create `stores/system.ts`

---

## Phase 7: Frontend - Admin Dashboard

**Goal**: Complete admin interface with shadcn-vue components

### 7.1 Setup Wizard

**Tasks:**
- [ ] Multi-step form for first-run setup
- [ ] Twilio credentials input
- [ ] Admin account creation
- [ ] Initial DID configuration
- [ ] Validation and error handling

### 7.2 Device Management

**Pattern (from Context7 shadcn-vue DataTable):**
```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useVueTable, getCoreRowModel, getPaginationRowModel, getSortedRowModel } from '@tanstack/vue-table'
import { columns } from './columns'
import DataTable from '@/components/ui/data-table/DataTable.vue'
import { useDevicesStore } from '@/stores/devices'

const store = useDevicesStore()
const data = computed(() => store.devices)

const table = useVueTable({
  get data() { return data.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
  getPaginationRowModel: getPaginationRowModel(),
  getSortedRowModel: getSortedRowModel(),
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <h2 class="text-2xl font-bold">Devices</h2>
      <Button @click="openAddDialog">Add Device</Button>
    </div>
    <DataTable :table="table" />
  </div>
</template>
```

**Tasks:**
- [ ] Device list with DataTable
- [ ] Add/Edit device dialog
- [ ] Delete confirmation
- [ ] Registration status indicators
- [ ] QR code generation for softphone provisioning

### 7.3 DID Management

**Tasks:**
- [ ] DID list with status
- [ ] Link Twilio numbers
- [ ] Per-DID routing configuration
- [ ] SMS settings per DID

### 7.4 Routing Rules

**Tasks:**
- [ ] Visual rule builder
- [ ] Drag-and-drop priority ordering
- [ ] Time-based schedule editor
- [ ] Caller ID list management (VIP, blocklist)
- [ ] DND toggle

### 7.5 System Settings

**Tasks:**
- [ ] Twilio configuration panel
- [ ] Email/SMTP settings
- [ ] Gotify push configuration
- [ ] Postmarkapp configuration
- [ ] Backup/restore functionality

---

## Phase 8: Frontend - User Portal

**Goal**: User-facing interface for calls, messages, and settings

### 8.1 Dashboard

**Tasks:**
- [ ] Recent calls summary cards
- [ ] Unread voicemails count
- [ ] Unread messages count
- [ ] DND quick toggle

### 8.2 Call History

**Tasks:**
- [ ] CDR list with filtering
- [ ] Date range picker
- [ ] Recording playback
- [ ] Export to CSV

### 8.3 Voicemail

**Tasks:**
- [ ] Voicemail list with read/unread
- [ ] Audio player with waveform
- [ ] Transcript display
- [ ] Delete/archive actions

### 8.4 Messages (SMS/MMS)

**Tasks:**
- [ ] Conversation view (threaded)
- [ ] Compose new message
- [ ] Media preview for MMS
- [ ] Mark as read

---

## Phase 9: Call Routing Engine

**Goal**: Intelligent call routing based on rules

### 9.1 Rules Engine

```go
// internal/rules/engine.go
type Engine struct {
    routeRepo RouteRepository
    deviceRepo DeviceRepository
}

type RouteResult struct {
    Action     ActionType
    Targets    []string
    Timeout    int
    FallbackAction ActionType
}

func (e *Engine) Evaluate(ctx context.Context, call *IncomingCall) (*RouteResult, error) {
    // Get routes for DID, ordered by priority
    routes, err := e.routeRepo.GetByDID(ctx, call.DIDID)
    if err != nil {
        return nil, err
    }

    for _, route := range routes {
        if !route.Enabled {
            continue
        }

        if e.matchesCondition(route, call) {
            return e.buildResult(route, call), nil
        }
    }

    // Default action
    return &RouteResult{Action: ActionVoicemail}, nil
}
```

**Tasks:**
- [ ] Create `internal/rules/engine.go`
- [ ] Create `internal/rules/conditions.go` - condition matchers
- [ ] Create `internal/rules/actions.go` - action executors
- [ ] Time-based condition matching
- [ ] Caller ID matching
- [ ] DND check

### 9.2 Call Blocking

**Tasks:**
- [ ] Blacklist check (exact, prefix, regex)
- [ ] Anonymous call rejection
- [ ] Twilio spam score threshold
- [ ] Block action execution

---

## Phase 10: Notifications & Integrations

**Goal**: Email, push notifications, and external webhooks

### 10.1 Email Notifications

**Tasks:**
- [ ] Create `internal/notify/email.go`
- [ ] SMTP configuration
- [ ] Postmarkapp integration
- [ ] Email templates (voicemail, missed call, SMS forward)

### 10.2 Push Notifications (Gotify)

**Tasks:**
- [ ] Create `internal/notify/gotify.go`
- [ ] Push notification for incoming calls
- [ ] Push notification for voicemails
- [ ] Push notification for SMS

### 10.3 External Webhooks

**Tasks:**
- [ ] Create `internal/notify/webhook.go`
- [ ] Configurable webhook endpoints
- [ ] Event payload formatting
- [ ] Retry logic with exponential backoff

### 10.4 Notification Failure Handling (P0 Requirement)

**Notification failure handling per REQUIREMENTS.md:**

```go
// internal/notify/retry.go
package notify

import (
    "context"
    "time"
    "github.com/btafoya/gosip/internal/config"
)

// EmailSender with retry logic
type EmailSender struct {
    client     SMTPClient
    maxRetries int
    retryWindow time.Duration
}

func NewEmailSender(client SMTPClient) *EmailSender {
    return &EmailSender{
        client:      client,
        maxRetries:  config.EmailMaxRetries,  // 3
        retryWindow: config.EmailRetryWindow, // 1 hour
    }
}

// SendWithRetry attempts to send email with automatic retries
// Per REQUIREMENTS.md: "Retry 3x over 1 hour, then abandon"
func (e *EmailSender) SendWithRetry(ctx context.Context, msg *Email) error {
    delays := []time.Duration{0, 5 * time.Minute, 20 * time.Minute, 35 * time.Minute}
    var lastErr error

    for attempt := 0; attempt <= e.maxRetries; attempt++ {
        if attempt > 0 {
            select {
            case <-ctx.Done():
                return ctx.Err()
            case <-time.After(delays[attempt]):
            }
        }

        if err := e.client.Send(msg); err == nil {
            return nil
        } else {
            lastErr = err
        }
    }

    // Abandoned after 3 retries over ~1 hour
    return fmt.Errorf("email send abandoned after %d retries: %w", e.maxRetries, lastErr)
}

// GotifySender with silent fail
type GotifySender struct {
    client     GotifyClient
    maxRetries int
}

func NewGotifySender(client GotifyClient) *GotifySender {
    return &GotifySender{
        client:     client,
        maxRetries: config.GotifyMaxRetries, // 3
    }
}

// Send with silent failure per REQUIREMENTS.md
// Per REQUIREMENTS.md: "Silent fail after 3 attempts"
func (g *GotifySender) Send(ctx context.Context, msg *PushNotification) {
    for attempt := 0; attempt <= g.maxRetries; attempt++ {
        if err := g.client.Send(msg); err == nil {
            return
        }
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    // Silent fail - no error returned, no user impact
}
```

**Storage full handling for recordings:**

```go
// internal/recording/storage.go
package recording

import (
    "os"
    "syscall"
)

// StorageChecker monitors disk space
type StorageChecker struct {
    dataDir    string
    minFreeGB  uint64
    notifier   AdminNotifier
}

// CheckBeforeRecording returns true if storage is available
// Per REQUIREMENTS.md: "Continue call, skip recording, alert admin"
func (s *StorageChecker) CheckBeforeRecording() (bool, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(s.dataDir, &stat); err != nil {
        return false, err
    }

    freeBytes := stat.Bavail * uint64(stat.Bsize)
    freeGB := freeBytes / (1024 * 1024 * 1024)

    if freeGB < s.minFreeGB {
        // Alert admin but don't fail the call
        s.notifier.AlertAdmin("Recording storage full",
            fmt.Sprintf("Only %d GB free, recordings disabled", freeGB))
        return false, nil
    }
    return true, nil
}

// RecordingHandler decides whether to record
func (h *RecordingHandler) ShouldRecord(deviceID int64) bool {
    // Check if device has recording enabled
    device, err := h.deviceRepo.GetByID(context.Background(), deviceID)
    if err != nil || !device.RecordingEnabled {
        return false
    }

    // Check storage availability
    canRecord, _ := h.storage.CheckBeforeRecording()
    return canRecord
}
```

**Failure Mode Response Matrix:**

| Component | Failure Mode | Response | Recovery |
|-----------|--------------|----------|----------|
| Email (SMTP) | Unreachable | Retry 3x over 1 hour, then abandon | Automatic |
| Push (Gotify) | Unreachable | Silent fail after 3 attempts | Automatic |
| Recording Storage | Full | Continue call, skip recording, alert admin | Admin cleanup required |

**Tasks:**
- [ ] Create `internal/notify/retry.go` - retry logic for notifications
- [ ] Implement email retry with 3x over 1 hour schedule
- [ ] Implement Gotify silent failure after 3 attempts
- [ ] Create `internal/recording/storage.go` - storage monitoring
- [ ] Implement storage check before recording
- [ ] Add admin alerting for storage full condition

---

## Phase 11: Testing

**Goal**: Comprehensive test coverage

### 11.1 Backend Tests

**Tasks:**
- [ ] Unit tests for repositories
- [ ] Unit tests for SIP handlers
- [ ] Unit tests for rules engine
- [ ] Integration tests for API endpoints
- [ ] E2E tests for SIP registration flow

### 11.2 Frontend Tests

**Tasks:**
- [ ] Component tests with Vitest
- [ ] Store tests
- [ ] E2E tests with Playwright

---

## Phase 12: Production Readiness

**Goal**: Production-ready deployment

### 12.1 Security Hardening

**Tasks:**
- [ ] HTTPS configuration (reverse proxy)
- [ ] Rate limiting
- [ ] Input validation
- [ ] SQL injection prevention
- [ ] XSS prevention
- [ ] CSRF protection
- [ ] Twilio webhook signature validation

### 12.2 Observability

**Tasks:**
- [ ] Structured logging (slog)
- [ ] Health check endpoint
- [ ] Metrics (optional Prometheus)
- [ ] Error tracking

### 12.3 Documentation

**Tasks:**
- [ ] API documentation
- [ ] Deployment guide
- [ ] Configuration reference
- [ ] Troubleshooting guide

---

## Implementation Order Summary

| Phase | Name | Dependencies | Estimated Complexity |
|-------|------|--------------|---------------------|
| 1 | Project Foundation | None | Low |
| 2 | Database Layer | Phase 1 | Medium |
| 3 | SIP Server | Phase 2 | High |
| 4 | Twilio Integration | Phase 2 | Medium |
| 5 | REST API | Phase 2, 3, 4 | Medium |
| 6 | Frontend Core | Phase 1 | Medium |
| 7 | Admin Dashboard | Phase 5, 6 | High |
| 8 | User Portal | Phase 5, 6 | Medium |
| 9 | Routing Engine | Phase 3, 4 | Medium |
| 10 | Notifications | Phase 5 | Low |
| 11 | Testing | All phases | Medium |
| 12 | Production | All phases | Medium |

---

## Key sipgo Patterns Reference

### SIP Server Setup
```go
ua, _ := sipgo.NewUA(sipgo.WithUserAgent("GoSIP"))
srv, _ := sipgo.NewServer(ua)
client, _ := sipgo.NewClient(ua)

srv.OnInvite(handleInvite)
srv.OnRegister(handleRegister)
srv.OnBye(handleBye)

go srv.ListenAndServe(ctx, "udp", "0.0.0.0:5060")
```

### Handling INVITE with Dialog
```go
dialogServer := sipgo.NewDialogServerCache(client, contactHDR)

srv.OnInvite(func(req *sip.Request, tx sip.ServerTransaction) {
    dlg, _ := dialogServer.ReadInvite(req, tx)
    defer dlg.Close()

    dlg.Respond(sip.StatusTrying, "Trying", nil)
    dlg.Respond(sip.StatusRinging, "Ringing", nil)
    dlg.Respond(sip.StatusOK, "OK", nil)

    <-dlg.Context().Done()
})
```

### Digest Authentication
```go
res, err := client.Do(ctx, req, sipgo.ClientRequestRegisterBuild)
if res.StatusCode == 401 {
    res, err = client.DoDigestAuth(ctx, req, res, sipgo.DigestAuth{
        Username: username,
        Password: password,
    })
}
```

---

## Key shadcn-vue Patterns Reference

### DataTable with Actions
```vue
<script setup>
import { useVueTable, getCoreRowModel } from '@tanstack/vue-table'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem } from '@/components/ui/dropdown-menu'
</script>
```

### Form with Validation
```vue
<script setup>
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'
import * as z from 'zod'

const formSchema = toTypedSchema(z.object({
  email: z.string().email(),
  password: z.string().min(8),
}))

const { handleSubmit } = useForm({ validationSchema: formSchema })
</script>
```

### Dialog Pattern
```vue
<Dialog>
  <DialogTrigger as-child>
    <Button>Open</Button>
  </DialogTrigger>
  <DialogContent>
    <DialogHeader>
      <DialogTitle>Title</DialogTitle>
      <DialogDescription>Description</DialogDescription>
    </DialogHeader>
    <!-- Content -->
    <DialogFooter>
      <Button>Save</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

---

## P0 Requirements Alignment Summary

The following P0 requirements from REQUIREMENTS.md are implemented in this plan:

| P0 Requirement | Implementation Location |
|---------------|------------------------|
| Performance SLAs (< 500ms SIP, < 2s call setup, < 200ms API GET) | Phase 1.0 - `constants.go` |
| Security (5 attempts → 15-min lockout, 24h sessions) | Phase 1.0 - `constants.go`, Phase 5.2 |
| Voicemail Settings (30s ring, 3min max, 3s min) | Phase 1.0 - `constants.go` |
| Database Indexes (10 performance indexes) | Phase 2.1 - `002_add_indexes.up.sql` |
| Twilio Failure Handling (queue, retry, alert) | Phase 4.2 |
| API Error Format (standard JSON structure) | Phase 5.2 |
| Login Attempt Tracking (IP-based lockout) | Phase 5.2 |
| SIP Device Offline Handling (maintain call until BYE) | Phase 3.4 |
| Email Retry (3x over 1 hour) | Phase 10.4 |
| Gotify Silent Fail (3 attempts) | Phase 10.4 |
| Recording Storage Full (skip, alert admin) | Phase 10.4 |

---

## Next Steps

1. **Start with Phase 1** - Get the project structure in place
2. **Parallel work**: Database schema (Phase 2) can start while SIP research continues
3. **Incremental delivery**: Each phase produces testable functionality
4. **Continuous integration**: Set up CI/CD early for automated testing
