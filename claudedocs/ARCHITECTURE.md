# GoSIP Architecture

**Generated**: 2025-12-15

A detailed view of the GoSIP system architecture, component relationships, and data flows.

---

## System Overview

GoSIP is a SIP-to-Twilio bridge PBX designed for small deployments (2-5 phones, 1-3 DIDs).

```
                                    ┌─────────────────┐
                                    │   Twilio Cloud  │
                                    │  (SIP Trunking) │
                                    │   (SMS/Voice)   │
                                    └────────┬────────┘
                                             │
                                             │ HTTPS/Webhooks
                                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                          GoSIP Server                             │
│  ┌────────────────┐    ┌────────────────┐    ┌────────────────┐  │
│  │   HTTP Server  │    │   SIP Server   │    │ Twilio Client  │  │
│  │   (Chi Router) │    │    (sipgo)     │    │ (twilio-go)    │  │
│  │     :8080      │    │  :5060/:5061   │    │                │  │
│  └───────┬────────┘    └───────┬────────┘    └───────┬────────┘  │
│          │                     │                      │           │
│          └─────────────────────┼──────────────────────┘           │
│                                │                                  │
│                    ┌───────────┴───────────┐                     │
│                    │     Rules Engine      │                     │
│                    │  (Call Routing Logic) │                     │
│                    └───────────┬───────────┘                     │
│                                │                                  │
│                    ┌───────────┴───────────┐                     │
│                    │      SQLite DB        │                     │
│                    │   (Repository Layer)  │                     │
│                    └───────────────────────┘                     │
└──────────────────────────────────────────────────────────────────┘
                                 │
                                 │ SIP (UDP/TCP/TLS)
                                 ▼
            ┌────────────────────────────────────────┐
            │           SIP Devices                  │
            │  (Grandstream GXP1760W, Softphones)    │
            └────────────────────────────────────────┘
```

---

## Component Relationships

### 1. Main Entry Point (`cmd/gosip/main.go`)

```
main()
  │
  ├─► config.Load()           → Load environment configuration
  │
  ├─► db.New()                → Initialize SQLite + Repositories
  │      └─► db.Migrate()     → Run schema migrations
  │
  ├─► sip.NewServer()         → Create SIP server with managers
  │      ├─► NewSessionManager()
  │      ├─► NewMOHManager()
  │      ├─► NewMWIManager()
  │      ├─► NewHoldManager()
  │      ├─► NewTransferManager()
  │      ├─► NewCertManager()      (if TLS enabled)
  │      ├─► NewSRTPSessionManager()
  │      └─► NewZRTPManager()      (if ZRTP enabled)
  │
  ├─► twilio.NewClient()      → Create Twilio API client
  │      └─► Start()          → Start queue processor & health checker
  │
  └─► api.NewRouter()         → Create HTTP router with all handlers
         └─► http.ListenAndServe()
```

### 2. HTTP Request Flow

```
HTTP Request
     │
     ▼
┌─────────────────────────────────────────┐
│            Chi Middleware Stack         │
│  ┌───────────────────────────────────┐  │
│  │  RequestID → RealIP → Logger →    │  │
│  │  Recoverer → Compress → CORS      │  │
│  └───────────────────────────────────┘  │
└─────────────────┬───────────────────────┘
                  │
     ┌────────────┼────────────┬───────────────┐
     │            │            │               │
     ▼            ▼            ▼               ▼
 /api/auth    /api/setup   /api/webhooks   /api/*
 (Public)     (Setup Only) (Twilio Sig)    (Auth Required)
                                                │
                                    ┌───────────┴───────────┐
                                    │                       │
                                    ▼                       ▼
                              User Routes            Admin Routes
                              (AuthMiddleware)       (AdminOnlyMiddleware)
```

### 3. SIP Call Flow

```
┌────────────┐         ┌────────────┐         ┌────────────┐
│ SIP Device │         │ GoSIP SIP  │         │   Twilio   │
│            │         │   Server   │         │            │
└─────┬──────┘         └─────┬──────┘         └─────┬──────┘
      │                      │                      │
      │ REGISTER (UDP/TLS)   │                      │
      │─────────────────────►│                      │
      │                      │                      │
      │     Authenticator    │                      │
      │   (Digest Auth)      │                      │
      │◄─────────────────────│                      │
      │                      │                      │
      │ REGISTER (w/auth)    │                      │
      │─────────────────────►│                      │
      │                      │                      │
      │  200 OK              │  Registrar.Register()│
      │◄─────────────────────│  → db.Registrations  │
      │                      │                      │
      │                      │                      │
═══════════════════════════════════════════════════════════
      │                      │                      │
      │                      │  INVITE (from Twilio)│
      │                      │◄─────────────────────│
      │                      │                      │
      │                      │  Rules Engine        │
      │                      │  └─► Blocklist check │
      │                      │  └─► Route matching  │
      │                      │  └─► Action: ring    │
      │                      │                      │
      │ INVITE               │                      │
      │◄─────────────────────│                      │
      │                      │                      │
      │ 180 Ringing          │                      │
      │─────────────────────►│                      │
      │                      │  180 Ringing         │
      │                      │─────────────────────►│
      │                      │                      │
      │ 200 OK               │                      │
      │─────────────────────►│                      │
      │                      │  200 OK              │
      │                      │─────────────────────►│
      │                      │                      │
      │  ACK                 │                      │
      │◄─────────────────────│◄─────────────────────│
      │                      │                      │
      │ ══════════ RTP/SRTP Media Stream ══════════│
      │                      │                      │
      │ BYE                  │                      │
      │─────────────────────►│                      │
      │                      │  BYE                 │
      │                      │─────────────────────►│
      │ 200 OK               │                      │
      │◄─────────────────────│◄─────────────────────│
      │                      │                      │
      │                      │  CDR.Create()        │
      │                      │  → db.CDRs           │
```

### 4. Database Layer Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                          DB struct                              │
├─────────────────────────────────────────────────────────────────┤
│  conn *sql.DB                                                   │
├─────────────────────────────────────────────────────────────────┤
│  Repositories:                                                  │
│  ┌─────────────────┬─────────────────┬─────────────────┐       │
│  │ Users           │ Devices         │ Registrations   │       │
│  │ GetByEmail()    │ GetByUsername() │ Upsert()        │       │
│  │ ValidatePass()  │ UpdateLastReg() │ DeleteExpired() │       │
│  └─────────────────┴─────────────────┴─────────────────┘       │
│  ┌─────────────────┬─────────────────┬─────────────────┐       │
│  │ DIDs            │ Routes          │ Blocklist       │       │
│  │ GetByNumber()   │ GetEnabledBy()  │ IsBlocked()     │       │
│  │ List()          │ Reorder()       │ Add/Remove()    │       │
│  └─────────────────┴─────────────────┴─────────────────┘       │
│  ┌─────────────────┬─────────────────┬─────────────────┐       │
│  │ CDRs            │ Voicemails      │ Messages        │       │
│  │ Create()        │ GetUnread()     │ GetConversation │       │
│  │ GetStats()      │ MarkAsRead()    │ List()          │       │
│  └─────────────────┴─────────────────┴─────────────────┘       │
│  ┌─────────────────┬─────────────────┬─────────────────┐       │
│  │ Config          │ Sessions        │ Provisioning    │       │
│  │ Get/Set()       │ Create/Delete() │ Tokens/Profiles │       │
│  └─────────────────┴─────────────────┴─────────────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

### 5. SIP Server Internal Structure

```
┌─────────────────────────────────────────────────────────────────┐
│                        SIP Server                               │
├─────────────────────────────────────────────────────────────────┤
│  cfg       Config                                               │
│  ua        *sipgo.UserAgent                                     │
│  srv       *sipgo.Server                                        │
│  client    *sipgo.Client                                        │
│  db        *db.DB                                               │
├─────────────────────────────────────────────────────────────────┤
│  Core Managers:                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │  registrar   │  │     auth     │  │   sessions   │          │
│  │  *Registrar  │  │*Authenticator│  │*SessionMgr   │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
├─────────────────────────────────────────────────────────────────┤
│  Call Control Managers:                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   holdMgr    │  │ transferMgr  │  │    mohMgr    │          │
│  │  *HoldMgr    │  │*TransferMgr  │  │  *MOHMgr     │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│  ┌──────────────┐                                               │
│  │    mwiMgr    │  (Message Waiting Indicator)                  │
│  │  *MWIMgr     │                                               │
│  └──────────────┘                                               │
├─────────────────────────────────────────────────────────────────┤
│  Security Managers:                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   certMgr    │  │   srtpMgr    │  │   zrtpMgr    │          │
│  │ *CertManager │  │*SRTPSessMgr  │  │*ZRTPManager  │          │
│  │ (TLS certs)  │  │ (media enc)  │  │ (E2E enc)    │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
└─────────────────────────────────────────────────────────────────┘
```

### 6. Twilio Integration Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      Twilio Client                              │
├─────────────────────────────────────────────────────────────────┤
│  Outbound Operations:                                           │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐        │
│  │   SendSMS()  │   │  MakeCall()  │   │ GetMessage() │        │
│  │  + retry     │   │              │   │              │        │
│  │  + backoff   │   │              │   │              │        │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘        │
│         │                  │                  │                 │
│         └──────────────────┼──────────────────┘                 │
│                            │                                    │
│                            ▼                                    │
│                   ┌──────────────────┐                         │
│                   │   MessageQueue   │                         │
│                   │  (Background)    │                         │
│                   └──────────────────┘                         │
├─────────────────────────────────────────────────────────────────┤
│  Health Monitoring:                                             │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  healthChecker() → CheckHealth() every 5 minutes        │  │
│  │  recordSuccess() / recordFailure() → healthy bool        │  │
│  │  failureCount >= 3 → mark unhealthy                      │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                            │
                            │ Webhooks
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Webhook Handlers                            │
├─────────────────────────────────────────────────────────────────┤
│  /webhooks/voice/incoming  → Route call via Rules Engine       │
│  /webhooks/voice/status    → Update CDR                        │
│  /webhooks/sms/incoming    → Store message, send notification  │
│  /webhooks/sms/status      → Update message status             │
│  /webhooks/recording       → Store recording metadata          │
│  /webhooks/transcription   → Store voicemail transcript        │
└─────────────────────────────────────────────────────────────────┘
```

### 7. Rules Engine Decision Flow

```
Evaluate(CallContext)
        │
        ▼
┌───────────────────┐
│  Check Blocklist  │
│  db.Blocklist.    │
│  IsBlocked()      │
└────────┬──────────┘
         │
    Blocked?
    ├── Yes ─► Return Action{Type: "reject", Name: "Blocklist"}
    │
    No
    │
    ▼
┌───────────────────┐
│  Get DID Routes   │
│  + Global Routes  │
│  Sort by Priority │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│  For each Route:  │◄─────────────────────────────┐
│  evaluateCondition│                              │
└────────┬──────────┘                              │
         │                                         │
    Match?                                         │
    ├── Yes ─► Return Action{Type, Data, Name}     │
    │                                              │
    No ─► Next Route ──────────────────────────────┘
    │
    All routes exhausted
    │
    ▼
Return Action{Type: "voicemail", Name: "Default"}


Condition Evaluation:
┌─────────────────────────────────────────────────────────────────┐
│  ConditionType    │  Logic                                     │
├───────────────────┼─────────────────────────────────────────────┤
│  "default"        │  Always true                               │
│  "callerid"       │  Match pattern (exact/prefix/contains/regex)│
│                   │  OR match anonymous callers                 │
│  "time"           │  Check hour range, day-of-week             │
│                   │  OR business_hours / after_hours shortcut  │
└─────────────────────────────────────────────────────────────────┘
```

### 8. Security Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Security Layers                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Layer 1: Transport Security                                    │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  SIP TLS (SIPS) ─ Port 5061                             │  │
│  │  │                                                       │  │
│  │  ├─► CertManager                                         │  │
│  │  │   ├─► ACME Mode (Let's Encrypt)                      │  │
│  │  │   │   └─► certmagic + libdns/cloudflare              │  │
│  │  │   └─► Manual Mode (custom certs)                     │  │
│  │  │                                                       │  │
│  │  └─► Option: DisableUnencrypted = true                   │  │
│  │      └─► Blocks UDP/TCP on port 5060                    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Layer 2: Media Security                                        │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  SRTP ─ AES-128-CM-HMAC-SHA1-80                         │  │
│  │  │                                                       │  │
│  │  └─► SRTPSessionManager                                  │  │
│  │      ├─► GenerateKeyMaterial()                          │  │
│  │      └─► pion/srtp for encryption                       │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Layer 3: End-to-End Encryption                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  ZRTP ─ Opportunistic/Required                          │  │
│  │  │                                                       │  │
│  │  └─► ZRTPManager                                         │  │
│  │      ├─► Key agreement (DH)                             │  │
│  │      ├─► SAS verification                               │  │
│  │      └─► DeriveKeys() → SRTP keys                       │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Layer 4: Application Security                                  │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  SIP: Digest Authentication (RFC 2617)                  │  │
│  │  HTTP: Session-based auth (bcrypt, 24h expiry)          │  │
│  │  Rate limiting: 5 failures = 15min lockout              │  │
│  │  Twilio: Webhook signature validation                   │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Data Flow Diagrams

### Inbound Voice Call

```
Twilio ──────► /webhooks/voice/incoming
                        │
                        ▼
               ┌────────────────┐
               │ Extract caller │
               │ DIDID, time    │
               └───────┬────────┘
                       │
                       ▼
               ┌────────────────┐
               │ Rules Engine   │
               │ Evaluate()     │
               └───────┬────────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
     "ring"       "voicemail"    "reject"
        │              │              │
        ▼              ▼              ▼
  TwiML <Dial>   TwiML <Record>  TwiML <Reject>
  SIP devices    + transcribe
```

### SMS Message Flow

```
Inbound SMS:
Twilio ──────► /webhooks/sms/incoming
                        │
                        ▼
               ┌────────────────┐
               │ db.Messages.   │
               │ Create()       │
               └───────┬────────┘
                       │
                       ▼
               ┌────────────────┐
               │ Check auto-    │
               │ reply rules    │
               └───────┬────────┘
                       │
        ┌──────────────┴──────────────┐
        │                             │
   Match found                   No match
        │                             │
        ▼                             ▼
  Twilio.SendSMS()              Notify user
  (auto-reply)                  (push/email)


Outbound SMS:
Frontend ──────► POST /api/messages
                        │
                        ▼
               ┌────────────────┐
               │ Twilio.SendSMS │
               │ (with retry)   │
               └───────┬────────┘
                       │
                       ▼
               ┌────────────────┐
               │ db.Messages.   │
               │ Create()       │
               └────────────────┘
```

### Device Provisioning Flow

```
Admin creates device:
POST /api/provisioning/device
        │
        ▼
┌───────────────────┐
│ Create Device     │
│ Create Token      │
│ Generate QR Code  │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Return URL:       │
│ /api/provision/   │
│ {token}           │
└───────────────────┘

Device fetches config:
GET /api/provision/{token}
        │
        ▼
┌───────────────────┐
│ Validate token    │
│ (expiry, revoked) │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Get profile       │
│ template          │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Render config     │
│ with device creds │
└────────┬──────────┘
         │
         ▼
┌───────────────────┐
│ Log DeviceEvent   │
│ Return config XML │
└───────────────────┘
```

---

## Cross-Reference Index

| Component | Depends On | Used By |
|-----------|------------|---------|
| `api.NewRouter` | `Dependencies`, all handlers | `main.go` |
| `sip.Server` | `db.DB`, `sipgo` | `main.go`, `api` handlers |
| `twilio.Client` | `config.Config` | `main.go`, `api` handlers |
| `rules.Engine` | `db.DB` | Webhook handlers |
| `db.DB` | SQLite | All packages |
| `models.*` | - | `db`, `api`, `rules` |
| `CertManager` | `certmagic`, `libdns` | `sip.Server` |
| `SRTPSessionManager` | `pion/srtp` | `sip.Server` |
| `ZRTPManager` | - | `sip.Server` |

---

## Background Goroutines

| Goroutine | Package | Trigger | Purpose |
|-----------|---------|---------|---------|
| `cleanupExpiredRegistrations` | `pkg/sip` | 60s ticker | Remove stale SIP registrations |
| `cleanupTerminatedSessions` | `pkg/sip` | 5min ticker | Clean up call sessions |
| `cleanupExpiredMWISubscriptions` | `pkg/sip` | 60s ticker | Remove expired MWI subs |
| `healthChecker` | `internal/twilio` | 5min ticker | Check Twilio API health |
| `MessageQueue.Start` | `internal/twilio` | On start | Process queued messages |
