# GoSIP Project Index

**Generated**: 2025-12-15
**Version**: 1.0.0

A comprehensive knowledge base and documentation index for the GoSIP SIP-to-Twilio bridge PBX system.

---

## Quick Navigation

| Section | Description |
|---------|-------------|
| [Architecture Overview](#architecture-overview) | System design and component relationships |
| [Package Reference](#package-reference) | All Go packages with descriptions |
| [API Endpoints](#api-endpoints) | Complete REST API reference |
| [Domain Models](#domain-models) | Data structures and entities |
| [Configuration](#configuration) | Constants, settings, and SLAs |
| [Frontend Structure](#frontend-structure) | Vue 3 application layout |
| [Database Schema](#database-schema) | SQLite tables and migrations |
| [Security Features](#security-features) | TLS, SRTP, ZRTP encryption |

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                     Vue 3 + Tailwind CSS                         │
│                    (frontend/src/views/)                         │
├──────────────────────────────────────────────────────────────────┤
│                      Go REST API (Chi)                           │
│                     (internal/api/)                              │
├────────────┬────────────┬────────────┬────────────┬──────────────┤
│    SIP     │   Rules    │   Twilio   │  Webhooks  │     MWI      │
│   Server   │   Engine   │   Client   │  Handlers  │   Manager    │
│ (pkg/sip/) │(rules/)    │ (twilio/)  │ (api/)     │ (pkg/sip/)   │
├────────────┴────────────┴────────────┴────────────┴──────────────┤
│                        SQLite + Repositories                      │
│                          (internal/db/)                          │
└──────────────────────────────────────────────────────────────────┘
```

### Component Flow

1. **Inbound Call** → SIP Server → Rules Engine → Action (Ring/Forward/Voicemail/Reject)
2. **Outbound Call** → API → Twilio Client → SIP Device
3. **SMS/MMS** → Webhook → Message Handler → Database + Notifications
4. **Voicemail** → Twilio Recording → Transcription Webhook → Email/Push Notification

---

## Package Reference

### cmd/gosip/
**Entry point** - Application initialization and lifecycle management.

| File | Purpose |
|------|---------|
| `main.go:21-135` | Initializes DB, SIP server, Twilio client, HTTP server; handles graceful shutdown |

### internal/api/
**REST API handlers** - All HTTP endpoints and middleware.

| File | Purpose | Key Functions |
|------|---------|---------------|
| `router.go:13-271` | Route definitions and handler wiring | `NewRouter()` |
| `auth.go` | Login/logout, user management | `Login()`, `CreateUser()` |
| `devices.go` | SIP device CRUD operations | `List()`, `Create()`, `Update()` |
| `dids.go` | Phone number management | `SyncFromTwilio()` |
| `routes.go` | Call routing rules | `Reorder()` |
| `cdrs.go` | Call detail records | `GetStats()` |
| `voicemails.go` | Voicemail access | `MarkAsRead()` |
| `messages.go` | SMS/MMS handling | `GetConversations()` |
| `calls.go` | Active call control | `HoldCall()`, `TransferCall()` |
| `provisioning.go` | Device auto-provisioning | `GetDeviceConfig()`, `GetTokenQRCode()` |
| `webhooks.go` | Twilio callbacks | `VoiceIncoming()`, `SMSIncoming()` |
| `system.go` | Configuration and admin | `CompleteSetup()`, `CreateBackup()` |
| `tls.go` | TLS/SRTP/ZRTP management | `GetEncryptionStatus()` |
| `mwi.go`, `mwi_handler.go` | Message Waiting Indicator | `TriggerNotification()` |
| `health.go` | Health check endpoints | `Health()`, `Ready()`, `Live()` |
| `middleware.go` | Auth, CORS, logging | `AuthMiddleware()`, `AdminOnlyMiddleware()` |
| `errors.go` | Error response formatting | `WriteError()` |
| `dependencies.go` | Dependency injection container | `Dependencies` struct |

### internal/db/
**Database layer** - SQLite repositories using the repository pattern.

| Repository | Table | Key Methods |
|------------|-------|-------------|
| `users.go` | `users` | `GetByEmail()`, `ValidatePassword()` |
| `devices.go` | `devices` | `GetByUsername()`, `UpdateLastRegistration()` |
| `registrations.go` | `registrations` | `Upsert()`, `DeleteExpired()` |
| `dids.go` | `dids` | `GetByNumber()` |
| `routes.go` | `routes` | `GetEnabledByDID()`, `Reorder()` |
| `blocklist.go` | `blocklist` | `IsBlocked()` |
| `cdrs.go` | `cdrs` | `Create()`, `GetStats()` |
| `voicemails.go` | `voicemails` | `GetUnread()`, `MarkAsRead()` |
| `messages.go` | `messages` | `GetConversation()` |
| `auto_replies.go` | `auto_replies` | Auto-reply rule storage |
| `config.go` | `config` | Key-value system config |
| `sessions.go` | `sessions` | Session token management |
| `provisioning_tokens.go` | `provisioning_tokens` | Token URL management |
| `provisioning_profiles.go` | `provisioning_profiles` | Vendor config templates |
| `device_events.go` | `device_events` | Device audit logging |
| `db.go:46-181` | Connection, migrations | `New()`, `Migrate()` |

### internal/config/
**Configuration** - Runtime settings and P0 constants.

| File | Purpose |
|------|---------|
| `config.go` | Environment loading, `Config` struct |
| `constants.go:1-79` | Performance SLAs, security settings, timeouts |

### internal/models/
**Domain models** - All data structures.

| Model | Description | Location |
|-------|-------------|----------|
| `User` | Admin/user accounts | `models.go:11-18` |
| `Device` | SIP devices with provisioning | `models.go:20-39` |
| `Registration` | Active SIP registrations | `models.go:41-51` |
| `DID` | Phone numbers | `models.go:53-61` |
| `Route` | Call routing rules | `models.go:63-74` |
| `BlocklistEntry` | Blocked numbers | `models.go:103-110` |
| `CDR` | Call detail records | `models.go:112-128` |
| `Voicemail` | Voicemail messages | `models.go:130-141` |
| `Message` | SMS/MMS messages | `models.go:143-156` |
| `AutoReply` | Auto-reply rules | `models.go:158-166` |
| `SystemConfig` | Key-value config | `models.go:168-173` |
| `ProvisioningToken` | Provisioning URLs | `models.go:175-188` |
| `ProvisioningProfile` | Vendor templates | `models.go:190-202` |
| `DeviceEvent` | Device audit events | `models.go:204-213` |

### internal/rules/
**Rules engine** - Call routing logic.

| File | Purpose | Key Functions |
|------|---------|---------------|
| `engine.go:15-31` | Rule evaluation engine | `Evaluate()` |
| `engine.go:60-107` | Blocklist + route matching | Priority-ordered evaluation |
| `engine.go:109-168` | Caller ID conditions | Pattern matching (exact/prefix/regex) |
| `engine.go:170-224` | Time conditions | Business hours, day-of-week |
| `engine.go:276-338` | Rule validation | `ValidateRule()` |
| `engine.go:350-383` | Preset rules | `GetPresetRules()` |

### internal/twilio/
**Twilio integration** - API client with retry logic.

| File | Purpose | Key Functions |
|------|---------|---------------|
| `client.go:14-56` | Client with health monitoring | `NewClient()`, `UpdateCredentials()` |
| `client.go:66-116` | SMS with retry | `SendSMS()`, exponential backoff |
| `client.go:118-146` | Outbound calls | `MakeCall()` |
| `client.go:274-314` | Phone number management | `ListIncomingPhoneNumbers()` |
| `client.go:454-514` | Message fetch/sync | `GetMessage()`, `ListMessages()` |
| `queue.go` | Message queue with retry | Background processing |
| `sip_trunk.go` | SIP trunk TLS configuration | Twilio trunk management |

### internal/notifications/
**Notification system** - Email, push, and alerting.

| File | Purpose |
|------|---------|
| `notifier.go` | Email (SMTP/Postmark), Gotify push |

### pkg/sip/
**SIP server** - Core SIP functionality using sipgo.

| File | Purpose | Key Functions |
|------|---------|---------------|
| `server.go:29-59` | Server struct with managers | TLS, SRTP, ZRTP, Session, Hold, Transfer, MOH, MWI |
| `server.go:62-163` | Server initialization | `NewServer()` |
| `server.go:166-252` | Listener startup | UDP, TCP, TLS, WSS listeners |
| `server.go:286-327` | Registration cleanup | Background goroutines |
| `server.go:406-486` | MWI NOTIFY sending | RFC 3265/3842 compliance |
| `server.go:488-676` | Encryption status methods | TLS, SRTP, ZRTP accessors |
| `handlers.go` | SIP method handlers | REGISTER, INVITE, BYE, CANCEL, OPTIONS |
| `handlers_mwi.go` | MWI SUBSCRIBE handler | Message waiting indicator |
| `auth.go` | Digest authentication | RFC 2617 |
| `registrar.go` | Registration management | Contact/Expires tracking |
| `session.go` | Call session management | Active call state |
| `hold.go` | Call hold functionality | re-INVITE handling |
| `transfer.go` | Call transfer | REFER handling |
| `moh.go` | Music on Hold | Audio streaming |
| `mwi.go` | MWI subscriptions | Subscription state machine |
| `certmanager.go` | TLS certificate management | ACME/Let's Encrypt, manual certs |
| `srtp.go` | SRTP media encryption | Key material generation |
| `zrtp.go` | ZRTP end-to-end encryption | SAS verification |

---

## API Endpoints

### Authentication
| Method | Path | Handler | Auth |
|--------|------|---------|------|
| POST | `/api/auth/login` | `authHandler.Login` | Public |
| POST | `/api/auth/logout` | `authHandler.Logout` | Public |

### Setup (Pre-auth)
| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/api/setup/status` | `systemHandler.GetSetupStatus` | Setup only |
| POST | `/api/setup/complete` | `systemHandler.CompleteSetup` | Setup only |

### Webhooks (Twilio-secured)
| Method | Path | Handler | Purpose |
|--------|------|---------|---------|
| POST | `/api/webhooks/voice/incoming` | `webhookHandler.VoiceIncoming` | Inbound calls |
| POST | `/api/webhooks/voice/status` | `webhookHandler.VoiceStatus` | Call status updates |
| POST | `/api/webhooks/sms/incoming` | `webhookHandler.SMSIncoming` | Inbound SMS |
| POST | `/api/webhooks/sms/status` | `webhookHandler.SMSStatus` | SMS delivery status |
| POST | `/api/webhooks/recording` | `webhookHandler.Recording` | Recording complete |
| POST | `/api/webhooks/transcription` | `webhookHandler.Transcription` | Transcription ready |

### Provisioning (Token-secured)
| Method | Path | Handler | Auth |
|--------|------|---------|------|
| GET | `/api/provision/{token}` | `provisioningHandler.GetDeviceConfig` | Token |

### Devices (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/devices` | `deviceHandler.List` |
| POST | `/api/devices` | `deviceHandler.Create` |
| GET | `/api/devices/registrations` | `deviceHandler.GetRegistrations` |
| GET | `/api/devices/{id}` | `deviceHandler.Get` |
| PUT | `/api/devices/{id}` | `deviceHandler.Update` |
| DELETE | `/api/devices/{id}` | `deviceHandler.Delete` |
| GET | `/api/devices/{id}/events` | `provisioningHandler.GetDeviceEvents` |

### DIDs (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/dids` | `didHandler.List` |
| POST | `/api/dids` | `didHandler.Create` |
| POST | `/api/dids/sync` | `didHandler.SyncFromTwilio` |
| GET | `/api/dids/{id}` | `didHandler.Get` |
| PUT | `/api/dids/{id}` | `didHandler.Update` |
| DELETE | `/api/dids/{id}` | `didHandler.Delete` |

### Routes (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/routes` | `routeHandler.List` |
| POST | `/api/routes` | `routeHandler.Create` |
| GET | `/api/routes/{id}` | `routeHandler.Get` |
| PUT | `/api/routes/{id}` | `routeHandler.Update` |
| DELETE | `/api/routes/{id}` | `routeHandler.Delete` |
| PUT | `/api/routes/reorder` | `routeHandler.Reorder` |

### Call Control (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/calls` | `callHandler.ListActiveCalls` |
| GET | `/api/calls/moh` | `callHandler.GetMOHStatus` |
| PUT | `/api/calls/moh` | `callHandler.UpdateMOH` |
| POST | `/api/calls/moh/upload` | `callHandler.UploadMOHAudio` |
| POST | `/api/calls/moh/validate` | `callHandler.ValidateMOHAudio` |
| GET | `/api/calls/{callID}` | `callHandler.GetCall` |
| POST | `/api/calls/{callID}/hold` | `callHandler.HoldCall` |
| POST | `/api/calls/{callID}/transfer` | `callHandler.TransferCall` |
| DELETE | `/api/calls/{callID}/transfer` | `callHandler.CancelTransferCall` |
| DELETE | `/api/calls/{callID}` | `callHandler.HangupCall` |

### CDRs (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/cdrs` | `cdrHandler.List` |
| GET | `/api/cdrs/stats` | `cdrHandler.GetStats` |
| GET | `/api/cdrs/{id}` | `cdrHandler.Get` |

### Voicemails (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/voicemails` | `voicemailHandler.List` |
| GET | `/api/voicemails/unread` | `voicemailHandler.ListUnread` |
| GET | `/api/voicemails/{id}` | `voicemailHandler.Get` |
| PUT | `/api/voicemails/{id}/read` | `voicemailHandler.MarkAsRead` |
| DELETE | `/api/voicemails/{id}` | `voicemailHandler.Delete` |

### MWI (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/mwi/status` | `mwiHandler.GetStatus` |
| POST | `/api/mwi/notify` | `mwiHandler.TriggerNotification` |

### Messages (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/messages` | `messageHandler.List` |
| POST | `/api/messages` | `messageHandler.Send` |
| GET | `/api/messages/stats` | `messageHandler.GetStats` |
| GET | `/api/messages/unread/count` | `messageHandler.GetUnreadCount` |
| GET | `/api/messages/conversations` | `messageHandler.GetConversations` |
| GET | `/api/messages/conversation/{number}` | `messageHandler.GetConversation` |
| PUT | `/api/messages/conversation/{number}/read` | `messageHandler.MarkConversationAsRead` |
| GET | `/api/messages/{id}` | `messageHandler.Get` |
| PUT | `/api/messages/{id}/read` | `messageHandler.MarkAsRead` |
| POST | `/api/messages/{id}/resend` | `messageHandler.Resend` |
| POST | `/api/messages/{id}/sync` | `messageHandler.SyncFromTwilio` |
| POST | `/api/messages/{id}/cancel` | `messageHandler.Cancel` |
| DELETE | `/api/messages/{id}` | `messageHandler.Delete` |

### Blocklist (Authenticated)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/blocklist` | `routeHandler.ListBlocklist` |
| POST | `/api/blocklist` | `routeHandler.AddToBlocklist` |
| DELETE | `/api/blocklist/{id}` | `routeHandler.RemoveFromBlocklist` |

### Admin-Only Routes
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/users` | `authHandler.ListUsers` |
| POST | `/api/users` | `authHandler.CreateUser` |
| GET/PUT/DELETE | `/api/users/{id}` | User CRUD |
| GET/PUT | `/api/system/config` | System configuration |
| POST | `/api/system/backup` | Create backup |
| POST | `/api/system/restore` | Restore backup |
| GET | `/api/system/status` | System status |
| PUT | `/api/dnd` | Toggle Do Not Disturb |

### TLS/Encryption Admin Routes
| Method | Path | Handler |
|--------|------|---------|
| GET | `/api/system/tls/status` | `tlsHandler.GetStatus` |
| PUT | `/api/system/tls/config` | `tlsHandler.UpdateConfig` |
| POST | `/api/system/tls/renew` | `tlsHandler.ForceRenewal` |
| POST | `/api/system/tls/reload` | `tlsHandler.ReloadCertificates` |
| GET | `/api/system/tls/certificate` | `tlsHandler.GetCertificateInfo` |
| GET | `/api/system/srtp/status` | `tlsHandler.GetSRTPStatus` |
| PUT | `/api/system/srtp/config` | `tlsHandler.UpdateSRTPConfig` |
| GET | `/api/system/zrtp/status` | `tlsHandler.GetZRTPStatus` |
| PUT | `/api/system/zrtp/config` | `tlsHandler.UpdateZRTPConfig` |
| GET | `/api/system/zrtp/sessions` | `tlsHandler.GetZRTPSessions` |
| GET | `/api/system/zrtp/sas` | `tlsHandler.GetZRTPSAS` |
| POST | `/api/system/zrtp/sas/verify` | `tlsHandler.VerifyZRTPSAS` |
| GET | `/api/system/encryption/status` | `tlsHandler.GetEncryptionStatus` |
| GET | `/api/system/trunks/tls/status` | `tlsHandler.GetTrunkTLSStatus` |
| POST | `/api/system/trunks/tls/enable` | `tlsHandler.EnableTrunkTLS` |
| POST | `/api/system/trunks/tls/migrate` | `tlsHandler.MigrateTrunkOrigination` |
| POST | `/api/system/trunks/tls/create` | `tlsHandler.CreateSecureTrunk` |

### Health (Public)
| Method | Path | Handler |
|--------|------|---------|
| GET | `/health`, `/api/health` | `healthHandler.Health` |
| GET | `/api/ready` | `healthHandler.Ready` |
| GET | `/api/live` | `healthHandler.Live` |

---

## Configuration

### P0 Performance Constants (`internal/config/constants.go`)

| Category | Constant | Value |
|----------|----------|-------|
| **Performance** | SIPRegistrationTimeout | 500ms |
| | CallSetupTimeout | 2s |
| | APIGetTimeout | 200ms |
| | APIPostTimeout | 500ms |
| | MaxConcurrentCalls | 5 |
| **Security** | MaxFailedLoginAttempts | 5 |
| | LoginLockoutDuration | 15min |
| | SessionDuration | 24h |
| | SpamScoreThreshold | 0.7 |
| **Voicemail** | VoicemailRingTimeout | 30s |
| | VoicemailMaxLength | 180s |
| | VoicemailMinLength | 3s |
| **Pagination** | DefaultPageSize | 20 |
| | MaxPageSize | 100 |
| **Retry** | TwilioMaxRetries | 3 |
| | EmailMaxRetries | 3 |
| | GotifyMaxRetries | 3 |
| **SIP** | DefaultSIPPort | 5060 |
| | DefaultHTTPPort | 8080 |
| | RegistrationExpires | 3600s |
| **TLS** | DefaultTLSPort | 5061 |
| | DefaultWSSPort | 5081 |
| | DefaultTLSMinVersion | "1.2" |

---

## Frontend Structure

### Views (`frontend/src/views/`)
| View | Purpose |
|------|---------|
| `DashboardView.vue` | Main dashboard |
| `DevicesView.vue` | SIP device management |
| `DIDsView.vue` | Phone number management |
| `RoutesView.vue` | Call routing rules |
| `CallsView.vue` | Call history |
| `CallControlView.vue` | Active call management |
| `VoicemailsView.vue` | Voicemail inbox |
| `MessagesView.vue` | SMS/MMS conversations |
| `SettingsView.vue` | System settings |
| `UsersView.vue` | User management |
| `ProvisioningView.vue` | Device provisioning |
| `SetupWizardView.vue` | Initial setup |
| `LoginView.vue` | Authentication |

### API Client (`frontend/src/api/`)
| File | Purpose |
|------|---------|
| `client.ts` | Axios instance with auth |
| `auth.ts` | Authentication API |
| `devices.ts` | Device API |
| `calls.ts` | Call control API |
| `provisioning.ts` | Provisioning API |

### State Management (`frontend/src/stores/`)
| Store | Purpose |
|-------|---------|
| `auth.ts` | Authentication state (Pinia) |

---

## Database Schema

### Migrations (`internal/db/migrations/`)
| Migration | Purpose |
|-----------|---------|
| `001_initial_schema.up.sql` | Core tables |
| `002_add_indexes.up.sql` | Performance indexes |
| `003_provisioning.up.sql` | Provisioning system |
| `004_linphone_profile.up.sql` | Linphone support |
| `005_add_linphone_device_type.up.sql` | Device type enum |
| `006_add_tls_config.up.sql` | TLS configuration |
| `007_add_disable_unencrypted.up.sql` | Security hardening |
| `008_sessions.up.sql` | Session management |

### Core Tables
- `users` - Admin and user accounts
- `devices` - SIP device registry
- `registrations` - Active SIP registrations
- `dids` - Phone numbers (DIDs)
- `routes` - Call routing rules
- `blocklist` - Blocked numbers
- `cdrs` - Call detail records
- `voicemails` - Voicemail messages
- `messages` - SMS/MMS messages
- `auto_replies` - Auto-reply rules
- `config` - Key-value system config
- `sessions` - User sessions
- `provisioning_tokens` - Device provisioning URLs
- `provisioning_profiles` - Vendor config templates
- `device_events` - Device audit log
- `schema_migrations` - Migration tracking

---

## Security Features

### Signaling Security
- **TLS/SIPS** - Port 5061, configurable certificates (ACME or manual)
- **WSS** - WebSocket Secure on configurable port
- **Option to disable unencrypted SIP** on port 5060

### Media Security
- **SRTP** - AES-128-CM-HMAC-SHA1-80 profile
- **ZRTP** - End-to-end encryption with SAS verification

### Authentication
- **Digest Authentication** - RFC 2617 for SIP
- **Session-based auth** - bcrypt password hashing, 24h sessions
- **Rate limiting** - 5 failed attempts = 15min lockout

### Twilio Integration
- **Webhook signature validation**
- **SIP trunk TLS** - Secure origination

---

## Key Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/emiago/sipgo` | v0.21.0 | SIP server |
| `github.com/go-chi/chi/v5` | v5.0.12 | HTTP router |
| `github.com/mattn/go-sqlite3` | v1.14.22 | SQLite driver |
| `github.com/twilio/twilio-go` | v1.20.0 | Twilio API |
| `github.com/pion/srtp/v2` | v2.0.20 | SRTP encryption |
| `github.com/caddyserver/certmagic` | v0.20.0 | ACME certificates |
| `github.com/yeqown/go-qrcode/v2` | v2.2.5 | QR code generation |
| `golang.org/x/crypto` | v0.21.0 | Cryptographic operations |

---

## Development Quick Reference

```bash
# Development
make dev-all          # Run both backend and frontend with hot reload

# Testing
make test             # Run all tests
make test-coverage    # With coverage report

# Linting
make lint-all         # Run all linters

# Build
make build            # Build frontend + backend
make release          # Build release artifacts

# Docker
make docker-up        # Start production environment
make docker-down      # Stop
```

---

## File Locations Quick Reference

| Need | Location |
|------|----------|
| Add API endpoint | `internal/api/router.go` |
| Add domain model | `internal/models/models.go` |
| Add database table | `internal/db/migrations/` |
| Add SIP handler | `pkg/sip/handlers.go` |
| Add routing rule | `internal/rules/engine.go` |
| Add Twilio feature | `internal/twilio/client.go` |
| Add frontend view | `frontend/src/views/` |
| Change constants | `internal/config/constants.go` |
