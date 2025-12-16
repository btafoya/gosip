# GoSIP Requirements Specification

## Project Overview

**GoSIP** is a full-featured SIP-to-Twilio bridge PBX with web-based management, designed for small home office/family use with 2-5 phones and 1-3 DIDs.

---

## Core Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Vue 3 + Tailwind CSS                     │
│              (Admin Dashboard + User Portal)                │
├─────────────────────────────────────────────────────────────┤
│                       Go REST API                           │
│            (Authentication, Business Logic)                 │
├──────────┬──────────┬──────────┬──────────┬────────────────┤
│   SIP    │  Rules   │  Twilio  │ Voicemail│   Webhook      │
│  Server  │  Engine  │   API    │  Handler │   API          │
│ (UDP/TCP)│          │          │          │                │
├──────────┴──────────┴──────────┴──────────┴────────────────┤
│                        SQLite                               │
│     (Config, Users, Devices, Routes, CDR, Messages)        │
├─────────────────────────────────────────────────────────────┤
│              File Storage (Recordings, Voicemail)           │
└─────────────────────────────────────────────────────────────┘
              │                           │
    ┌─────────▼─────────┐       ┌─────────▼─────────┐
    │  SIP Endpoints    │       │   Twilio Cloud    │
    │ • Grandstream     │       │ • SIP Trunk       │
    │ • Softphones      │       │ • SMS/MMS         │
    │ • (Future WebRTC) │       │ • Transcription   │
    └───────────────────┘       └───────────────────┘
```

---

## Functional Requirements

### 1. SIP Server

| Feature | Description |
|---------|-------------|
| Protocol | UDP (primary), TCP (optional) on port 5060 |
| Authentication | Digest auth (MD5) per RFC 2617 |
| Device Support | Grandstream GXP1760W, standard SIP softphones |
| Registration | Track registrations with expiry/refresh |
| NAT Handling | Symmetric RTP, rport support |

**Supported Devices:**
- Grandstream GXP1760W (primary hardware phone)
- Softphones: Onesip, Zoiper, Linphone, Onesip/Phone, etc.
- Future: WebRTC via Twilio Client SDK

### 2. Twilio Integration

| Feature | Description |
|---------|-------------|
| SIP Trunking | Elastic SIP Trunk for inbound/outbound |
| SMS/MMS | Programmable Messaging API |
| Voicemail | Twilio recording + transcription |
| Caller ID | STIR/SHAKEN validation, spam scoring |

**Required Twilio Products:**
- Elastic SIP Trunking
- Programmable SMS/MMS
- Add-ons: Voicemail transcription, Spam filtering

### 3. Call Routing & Forwarding

| Rule Type | Description |
|-----------|-------------|
| Time-based | Business hours, after hours, weekends, holidays |
| Caller ID matching | VIP list, known callers, blacklist |
| DND mode | Manual toggle via UI |
| DID routing | Route specific DIDs to specific phones/destinations |

**Forwarding Actions:**
- Ring registered device(s)
- Forward to external number
- Send to voicemail
- Play announcement + disconnect
- Reject (busy signal)

### 4. Call Blocking

| Feature | Description |
|---------|-------------|
| Blacklist | Block specific phone numbers |
| Anonymous rejection | Block calls with no caller ID |
| Spam filtering | Twilio spam score threshold |

**Blocking Actions:**
- Reject immediately
- Play "number disconnected" tone
- Send to voicemail silently

### 5. Voicemail

| Feature | Description |
|---------|-------------|
| Recording | Twilio-hosted recording |
| Transcription | Twilio transcription service |
| Storage | Download and store locally |
| Notification | Email with audio + transcript |
| Playback | In-app audio player |

### 6. Call Recording

| Feature | Description |
|---------|-------------|
| Mode | On-demand (per-call or per-device setting) |
| Storage | Local filesystem (Docker volume) |
| Playback | In-app audio player with download |
| Retention | Configurable retention period |

### 7. SMS/MMS

| Feature | Description |
|---------|-------------|
| Inbound | Receive and display in UI |
| Outbound | Compose and send from UI |
| Email forwarding | Forward incoming SMS to email |
| Auto-reply | Configurable auto-responses (DND, after hours) |
| MMS | Image/media support |

### 8. CDR (Call Detail Records)

| Field | Description |
|-------|-------------|
| Timestamp | Call start/end times |
| Direction | Inbound/Outbound |
| From/To | Caller ID and destination |
| Duration | Call length in seconds |
| Disposition | Answered, voicemail, missed, blocked |
| Recording | Link to recording if enabled |

---

## Non-Functional Requirements

### Performance Requirements

| Metric | Requirement | Measurement |
|--------|-------------|-------------|
| SIP Registration Time | < 500ms | Time from REGISTER to 200 OK |
| Call Setup Latency | < 2 seconds | Time from INVITE to media flow |
| API Response (GET) | < 200ms | 95th percentile |
| API Response (POST) | < 500ms | 95th percentile |
| Concurrent Calls | 5 minimum | Simultaneous active calls |
| System Startup | < 30 seconds | Time to accept first registration |

### Authentication & Authorization

| Requirement | Description |
|-------------|-------------|
| User Roles | Admin (full access), User (own data only) |
| Auth Method | Session-based with secure cookies |
| Password | Bcrypt hashing, minimum 8 characters |
| Sessions | 24-hour timeout, refresh on activity |
| Failed Login Lockout | 5 attempts per minute, 15-minute lockout |
| Spam Score Threshold | Calls with Twilio spam_score > 0.7 are blocked |

### Deployment

| Requirement | Description |
|-------------|-------------|
| Container | Docker Compose orchestration |
| Persistence | SQLite + volume mounts for media |
| Ports | 5060/UDP (SIP), 8080/TCP (Web UI) |
| TLS Termination | Caddy reverse proxy (external) |
| First Run | Setup wizard for initial configuration |

### Configuration

| Requirement | Description |
|-------------|-------------|
| No config files | All settings via Web UI |
| Setup wizard | First-run Twilio credentials, admin account |
| Runtime changes | Hot-reload where possible |
| Backup/Restore | Export/import configuration |

### Failure Modes & Recovery

| Component | Failure Mode | Response | Recovery |
|-----------|--------------|----------|----------|
| Twilio API | Unreachable/5xx | Queue requests, alert admin after 3 failures | Auto-retry with exponential backoff |
| Twilio API | Rate limited (429) | Backoff and queue | Auto-retry after cooldown |
| SIP Device | Offline mid-call | Maintain call until BYE/timeout | Auto-reregister on reconnect |
| SQLite | Corruption | Switch to read-only mode | Manual restore from backup |
| Recording Storage | Full | Continue call, skip recording, alert admin | Admin cleanup required |
| Email (SMTP) | Unreachable | Retry 3x over 1 hour, then abandon | Automatic |
| Push (Gotify) | Unreachable | Silent fail after 3 attempts | Automatic |

### API Conventions

| Convention | Specification |
|------------|---------------|
| Base Path | `/api/` (no version prefix) |
| Pagination Default | 20 items per page |
| Pagination Max | 100 items per page |
| Pagination Params | `?page=1&per_page=20&sort=created_at&order=desc` |
| Date Format | ISO 8601 (e.g., `2025-01-15T14:30:00Z`) |
| Phone Format | E.164 (e.g., `+15551234567`) |

**Standard Error Response:**
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Human readable message",
    "details": [{"field": "email", "message": "Invalid format"}]
  }
}
```

### Voicemail Settings

| Parameter | Value |
|-----------|-------|
| Ring timeout before voicemail | 30 seconds |
| Maximum recording length | 3 minutes (180 seconds) |
| Minimum recording length | 3 seconds (shorter discarded) |
| Silence timeout | 10 seconds |

---

## Data Model (SQLite)

### Core Tables

```sql
-- System configuration (key-value)
config (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME
)

-- Admin and user accounts
users (
    id INTEGER PRIMARY KEY,
    email TEXT UNIQUE,
    password_hash TEXT,
    role TEXT CHECK(role IN ('admin', 'user')),
    created_at DATETIME,
    last_login DATETIME
)

-- Registered SIP devices
devices (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT,
    username TEXT UNIQUE,
    password_hash TEXT,
    device_type TEXT,  -- 'grandstream', 'softphone', 'webrtc'
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME
)

-- Active SIP registrations
registrations (
    id INTEGER PRIMARY KEY,
    device_id INTEGER REFERENCES devices(id),
    contact TEXT,
    expires_at DATETIME,
    user_agent TEXT,
    ip_address TEXT,
    transport TEXT,
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
)

-- Phone numbers (DIDs)
dids (
    id INTEGER PRIMARY KEY,
    number TEXT UNIQUE,
    twilio_sid TEXT,
    name TEXT,
    sms_enabled BOOLEAN,
    voice_enabled BOOLEAN
)

-- Call routing rules
routes (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id),
    priority INTEGER,
    name TEXT,
    condition_type TEXT,  -- 'time', 'callerid', 'default'
    condition_data JSON,
    action_type TEXT,     -- 'ring', 'forward', 'voicemail', 'reject'
    action_data JSON,
    enabled BOOLEAN
)

-- Blocked numbers
blocklist (
    id INTEGER PRIMARY KEY,
    pattern TEXT,
    pattern_type TEXT,  -- 'exact', 'prefix', 'regex'
    reason TEXT,
    created_at DATETIME
)

-- Call detail records
cdrs (
    id INTEGER PRIMARY KEY,
    call_sid TEXT UNIQUE,
    direction TEXT,
    from_number TEXT,
    to_number TEXT,
    did_id INTEGER REFERENCES dids(id),
    device_id INTEGER REFERENCES devices(id),
    started_at DATETIME,
    answered_at DATETIME,
    ended_at DATETIME,
    duration INTEGER,
    disposition TEXT,
    recording_url TEXT,
    spam_score REAL
)

-- Voicemails
voicemails (
    id INTEGER PRIMARY KEY,
    cdr_id INTEGER REFERENCES cdrs(id),
    user_id INTEGER REFERENCES users(id),
    from_number TEXT,
    audio_url TEXT,
    transcript TEXT,
    duration INTEGER,
    is_read BOOLEAN DEFAULT FALSE,
    created_at DATETIME
)

-- SMS/MMS messages
messages (
    id INTEGER PRIMARY KEY,
    message_sid TEXT UNIQUE,
    direction TEXT,
    from_number TEXT,
    to_number TEXT,
    did_id INTEGER REFERENCES dids(id),
    body TEXT,
    media_urls JSON,
    status TEXT,
    created_at DATETIME,
    is_read BOOLEAN DEFAULT FALSE
)

-- Auto-reply rules
auto_replies (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id),
    trigger_type TEXT,  -- 'dnd', 'after_hours', 'keyword'
    trigger_data JSON,
    reply_text TEXT,
    enabled BOOLEAN
)

-- Performance indexes
CREATE INDEX idx_cdrs_started ON cdrs(started_at DESC);
CREATE INDEX idx_cdrs_disposition ON cdrs(disposition);
CREATE INDEX idx_cdrs_did ON cdrs(did_id);
CREATE INDEX idx_messages_created ON messages(created_at DESC);
CREATE INDEX idx_messages_did ON messages(did_id);
CREATE INDEX idx_voicemails_user ON voicemails(user_id);
CREATE INDEX idx_voicemails_read ON voicemails(is_read);
CREATE INDEX idx_registrations_device ON registrations(device_id);
CREATE INDEX idx_registrations_expires ON registrations(expires_at);
CREATE INDEX idx_routes_did_priority ON routes(did_id, priority);
```

---

## UI Components (Vue 3 + Tailwind)

### Admin Dashboard

- **Setup Wizard** (first run)
  - Twilio credentials
  - Admin account creation
  - First DID configuration

- **System Settings**
  - Twilio configuration
  - Email/SMTP settings
  - Webhook endpoints
  - Backup/restore

- **Device Management**
  - Add/edit/delete devices
  - View registration status
  - Generate SIP credentials
  - QR code for softphone provisioning

- **DID Management**
  - Link Twilio numbers
  - Configure routing per DID
  - SMS settings per DID

- **Routing Rules**
  - Visual rule builder
  - Time-based schedule editor
  - Caller ID lists (VIP, blocklist)
  - DND toggle

- **User Management**
  - Add/edit/delete users
  - Role assignment
  - Password reset

### User Portal

- **Dashboard**
  - Recent calls summary
  - Unread voicemails/messages count
  - DND quick toggle

- **Call History**
  - Filterable CDR list
  - Playback for recordings
  - Export to CSV

- **Voicemail**
  - List with read/unread status
  - Audio player + transcript
  - Delete/archive

- **Messages**
  - Conversation view (SMS/MMS)
  - Compose new message
  - Media preview

- **Settings**
  - Personal device preferences
  - Notification preferences
  - Password change

---

## API Endpoints

### Authentication
```
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
POST   /api/auth/password
```

### Devices
```
GET    /api/devices
POST   /api/devices
GET    /api/devices/:id
PUT    /api/devices/:id
DELETE /api/devices/:id
GET    /api/devices/:id/qr
```

### DIDs
```
GET    /api/dids
POST   /api/dids
GET    /api/dids/:id
PUT    /api/dids/:id
DELETE /api/dids/:id
```

### Routes
```
GET    /api/routes
POST   /api/routes
GET    /api/routes/:id
PUT    /api/routes/:id
DELETE /api/routes/:id
PUT    /api/routes/reorder
```

### Blocklist
```
GET    /api/blocklist
POST   /api/blocklist
DELETE /api/blocklist/:id
```

### CDRs
```
GET    /api/cdrs
GET    /api/cdrs/:id
GET    /api/cdrs/:id/recording
```

### Voicemail
```
GET    /api/voicemails
GET    /api/voicemails/:id
PUT    /api/voicemails/:id/read
DELETE /api/voicemails/:id
GET    /api/voicemails/:id/audio
```

### Messages
```
GET    /api/messages
POST   /api/messages
GET    /api/messages/:id
PUT    /api/messages/:id/read
```

### Webhooks (Twilio callbacks)
```
POST   /api/webhooks/voice/incoming
POST   /api/webhooks/voice/status
POST   /api/webhooks/sms/incoming
POST   /api/webhooks/sms/status
POST   /api/webhooks/recording
POST   /api/webhooks/transcription
```

### External API (webhook consumers)
```
GET    /api/external/calls
GET    /api/external/messages
POST   /api/external/call
POST   /api/external/sms
```

### System (Admin only)
```
GET    /api/system/config
PUT    /api/system/config
POST   /api/system/backup
POST   /api/system/restore
GET    /api/system/health
```

---

## Docker Compose v2 Structure

```yaml
services:
  gosip:
    build: .
    ports:
      - "5060:5060/udp"    # SIP
      - "5060:5060/tcp"    # SIP over TCP
      - "8080:8080"        # Web UI
    volumes:
      - ./data:/app/data           # SQLite + media storage
    environment:
      - GOSIP_DATA_DIR=/app/data
      - GOSIP_HTTP_PORT=8080
      - GOSIP_SIP_PORT=5060
    restart: unless-stopped
```

**Volume Structure:**
```
data/
├── gosip.db              # SQLite database
├── recordings/           # Call recordings
├── voicemails/           # Voicemail audio files
└── backups/              # Configuration backups
```

---

## Future Considerations

### WebRTC Support Widget
- Twilio Client SDK integration
- Embeddable widget for websites
- Click-to-call functionality
- Browser-based softphone
- Support queue integration

### Additional Features (Backlog)
- Conference calling
- Call transfer
- Music on hold
- IVR/Auto-attendant
- Multiple tenants
- Mobile app (React Native)

---

## Technology Stack Summary

| Layer | Technology |
|-------|------------|
| Backend | Go 1.21+ |
| Frontend | Vue 3 + Tailwind CSS |
| Database | SQLite |
| SIP | Go implementation https://github.com/emiago/sipgo |
| Deployment | Docker Compose |
| Cloud | Twilio (SIP, SMS, Transcription), Gotify Push Notifications |
| Email | SMTP (configurable), Postmarkapp |

---

## Security Considerations

- All passwords bcrypt hashed
- SIP credentials separate from UI credentials
- Rate limiting on auth endpoints
- HTTPS recommended for production (reverse proxy)
- Twilio webhook signature validation
- Session timeout and secure cookies
- Input validation on all API endpoints
