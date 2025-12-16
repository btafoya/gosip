# GoSIP

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-btafoya%2Fgosip-181717?style=flat&logo=github)](https://github.com/btafoya/gosip)

A full-featured SIP-to-Twilio bridge PBX with web-based management for home office and small family use.

**Repository**: [https://github.com/btafoya/gosip](https://github.com/btafoya/gosip)

## Overview

GoSIP connects SIP phones to Twilio's cloud services, providing professional telephony features without complex PBX infrastructure. It acts as a bridge between your local SIP devices and Twilio's cloud platform for voice, SMS, and MMS services.

**Target Scale**: 2-5 phones, 1-3 DIDs

## Features

### Core Telephony
- **SIP Server** - UDP/TCP on port 5060 and TLS on port 5061 with Digest authentication (< 500ms registration)
- **Encrypted Mode** - Option to disable unencrypted traffic (TLS-only mode)
- **Call Control** - Hold/resume, attended/blind transfer, music on hold
- **Session Management** - Active call tracking, session state persistence
- **Message Waiting Indicator (MWI)** - Voicemail notification via SIP SUBSCRIBE/NOTIFY

### Device Support
- **Hardware Phones** - Grandstream GXP1760W and similar
- **Softphones** - Onesip, Zoiper, Linphone
- **Auto-Provisioning** - QR code provisioning, vendor-specific config templates
- **Device Events** - Registration tracking, config fetch logging

### Call Routing
- **Time-Based Rules** - Route calls based on day/time schedules
- **Caller ID Matching** - VIP routing, pattern-based rules
- **Priority System** - Configurable route priorities with reordering
- **DND Toggle** - System-wide Do Not Disturb mode

### Call Blocking & Spam
- **Blocklist Management** - Exact, prefix, and regex patterns
- **Anonymous Rejection** - Block calls without caller ID
- **Spam Filtering** - Score-based blocking (threshold > 0.7)

### Voicemail
- **Twilio Recording** - Cloud-based voicemail capture
- **Transcription** - Automatic speech-to-text via Twilio
- **Email Notification** - Voicemail alerts with transcripts
- **Push Notifications** - Gotify integration for mobile alerts
- **MWI Support** - Light up voicemail indicators on SIP phones

### Messaging
- **SMS/MMS** - Send and receive through Twilio
- **Conversation View** - Grouped message threads
- **Email Forwarding** - Forward incoming messages to email
- **Auto-Reply** - Automatic responses (DND, after-hours, keyword)
- **Message Sync** - Sync with Twilio for status updates

### Call Recording
- **On-Demand Recording** - Per-device recording settings
- **Local Storage** - Recordings stored on your server
- **Storage Monitoring** - Alerts when storage is low

### Web UI
- **Admin Dashboard** - Full system configuration
- **User Portal** - Personal voicemail, messages, call history
- **Setup Wizard** - Guided initial configuration
- **Device Provisioning** - QR codes for easy phone setup
- **Real-Time Updates** - Live call and registration status

### Security & Encryption
- **Authentication** - Admin and user roles
- **Session Management** - 24-hour sessions with secure tokens
- **Login Protection** - 5 failed attempts → 15-min lockout
- **Webhook Validation** - Twilio signature verification
- **TLS/SIPS** - Encrypted SIP signaling on port 5061
- **SRTP** - Encrypted media with AES-CM-128/256 and HMAC-SHA1
- **ZRTP** - End-to-end key exchange with SAS verification
- **ACME Certificates** - Automatic Let's Encrypt certificate management
- **Twilio TLS** - Secure trunk connections with sips: scheme

### Resilience
- **Auto-Retry** - Exponential backoff for Twilio API calls
- **Request Queuing** - Queue messages during outages
- **Health Monitoring** - Service health endpoints
- **Graceful Degradation** - Continue operation during partial failures

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.21+ |
| Frontend | Vue 3 + Tailwind CSS |
| Database | SQLite |
| SIP | [sipgo](https://github.com/emiago/sipgo) |
| HTTP Router | [chi](https://github.com/go-chi/chi) |
| Deployment | Docker Compose v2 |
| Cloud | Twilio (SIP, SMS, Transcription) |
| Push Notifications | Gotify |
| Email | SMTP or Postmarkapp |

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+ with pnpm
- Docker and Docker Compose v2 (for production)
- Twilio account with SIP Trunking

### Development

```bash
# Clone repository
git clone https://github.com/btafoya/gosip.git
cd gosip

# Setup development environment
make setup

# Run both backend and frontend with hot reload
make dev-all
```

Or manually:

```bash
# Backend
go mod tidy
go run cmd/gosip/main.go

# Frontend (separate terminal)
cd frontend
pnpm install
pnpm dev
```

### Production (Docker)

```bash
docker-compose up --build
```

Access the web UI at `http://localhost:8080`

## Configuration

All configuration is done via the web UI. First run presents a setup wizard for:
- Twilio credentials (Account SID, Auth Token)
- Admin account creation
- Initial DID configuration

## Make Targets

```bash
make              # Build everything (frontend + backend)
make dev          # Run backend with hot reload (air)
make dev-frontend # Run frontend dev server
make dev-all      # Run both backend and frontend dev servers
make test         # Run all tests with race detection
make lint         # Run Go linters
make docker       # Build Docker image
make help         # Show all available targets
```

See `make help` for the complete list of targets.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Vue 3 + Tailwind CSS                     │
├─────────────────────────────────────────────────────────────┤
│                       Go REST API (chi)                     │
├──────────┬──────────┬──────────┬──────────┬────────────────┤
│   SIP    │  Rules   │  Twilio  │ Voicemail│   Webhook      │
│  Server  │  Engine  │   API    │  Handler │   Handler      │
├──────────┴──────────┴──────────┴──────────┴────────────────┤
│                        SQLite                               │
└─────────────────────────────────────────────────────────────┘
```

## Project Structure

```
gosip/
├── cmd/gosip/           # Application entry point
├── internal/
│   ├── api/             # REST API handlers
│   ├── audio/           # Audio file processing (WAV validation)
│   ├── config/          # Runtime configuration & constants
│   ├── db/              # SQLite repository layer
│   │   └── migrations/  # Embedded SQL migrations
│   ├── models/          # Domain models
│   ├── notifications/   # Email & push notifications
│   ├── rules/           # Call routing engine
│   └── twilio/          # Twilio API client with retry logic
├── pkg/sip/             # SIP integration (sipgo wrapper)
│   ├── server.go        # sipgo server setup
│   ├── handlers.go      # REGISTER, INVITE, BYE, REFER handlers
│   ├── auth.go          # Digest authentication
│   ├── registrar.go     # Registration management
│   ├── session.go       # Call session tracking
│   ├── hold.go          # Call hold/resume
│   ├── transfer.go      # Call transfer (attended/blind)
│   ├── moh.go           # Music on hold
│   ├── mwi.go           # Message waiting indicator
│   ├── certmanager.go   # TLS certificate management (ACME/manual)
│   ├── srtp.go          # SRTP media encryption
│   └── zrtp.go          # ZRTP end-to-end encryption
├── frontend/            # Vue 3 SPA
│   ├── src/
│   │   ├── views/       # Page components
│   │   ├── stores/      # Pinia state management
│   │   └── api/         # API client
│   └── tailwind.config.js
├── claudedocs/          # Generated documentation
│   ├── PROJECT_INDEX.md # Complete knowledge base
│   └── ARCHITECTURE.md  # System architecture
├── docs/                # Reference documentation
│   ├── API.md           # API documentation
│   └── tutorials/       # SIP protocol tutorials
├── docker-compose.yml
└── Dockerfile
```

## API Endpoints

### Public
- `GET /health` - Health check
- `POST /api/auth/login` - User login
- `POST /api/auth/logout` - User logout
- `GET /api/setup/status` - Setup wizard status
- `GET /api/provision/{token}` - Device provisioning

### Protected (Authenticated)
- `/api/devices/*` - SIP device management
- `/api/dids/*` - Phone number management
- `/api/routes/*` - Call routing rules
- `/api/calls/*` - Active call control (hold, transfer, hangup)
- `/api/cdrs/*` - Call history
- `/api/voicemails/*` - Voicemail access
- `/api/messages/*` - SMS/MMS messaging
- `/api/mwi/*` - Message waiting indicator status
- `/api/blocklist/*` - Call blocking rules
- `/api/provisioning/*` - Device provisioning management

### Admin Only
- `/api/users/*` - User management
- `/api/system/*` - System configuration
- `/api/system/tls/*` - TLS/encryption configuration
- `/api/system/srtp/*` - SRTP media encryption settings
- `/api/system/zrtp/*` - ZRTP key exchange and SAS verification
- `/api/system/trunks/tls/*` - Twilio trunk TLS management
- `/api/system/encryption/status` - Comprehensive encryption status

### Webhooks
- `POST /api/webhooks/voice/incoming` - Incoming call handler
- `POST /api/webhooks/voice/status` - Call status updates
- `POST /api/webhooks/sms/incoming` - Incoming SMS/MMS
- `POST /api/webhooks/sms/status` - Message status updates
- `POST /api/webhooks/recording` - Recording completion
- `POST /api/webhooks/transcription` - Transcription completion

## Performance SLAs

| Metric | Target |
|--------|--------|
| SIP Registration | < 500ms |
| Call Setup | < 2 seconds |
| API Response (GET) | < 200ms (95th percentile) |
| API Response (POST) | < 500ms (95th percentile) |
| Concurrent Calls | 5 minimum |
| System Startup | < 30 seconds |

## Failure Handling

| Component | Failure Mode | Response |
|-----------|--------------|----------|
| Twilio API | Unreachable/5xx | Queue requests, retry with backoff, alert after 3 failures |
| Twilio API | Rate limited (429) | Backoff and queue, auto-retry after cooldown |
| SIP Device | Offline mid-call | Maintain call until BYE/timeout |
| Email (SMTP) | Unreachable | Retry 3x over 1 hour, then abandon |
| Push (Gotify) | Unreachable | Silent fail after 3 attempts |
| Recording Storage | Full | Continue call, skip recording, alert admin |

## Documentation

### Project Documentation
| Document | Description |
|----------|-------------|
| [claudedocs/PROJECT_INDEX.md](claudedocs/PROJECT_INDEX.md) | Complete knowledge base with package reference and API docs |
| [claudedocs/ARCHITECTURE.md](claudedocs/ARCHITECTURE.md) | System architecture diagrams and component relationships |
| [REQUIREMENTS.md](REQUIREMENTS.md) | Complete functional and technical specification |
| [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) | Development roadmap with P0 specifications |

### Security Documentation
| Document | Description |
|----------|-------------|
| [claudedocs/TLS_ENCRYPTION_IMPLEMENTATION_PLAN.md](claudedocs/TLS_ENCRYPTION_IMPLEMENTATION_PLAN.md) | TLS/SRTP/ZRTP encryption implementation |
| [claudedocs/SECURITY_FIX_SESSION_TOKENS.md](claudedocs/SECURITY_FIX_SESSION_TOKENS.md) | Session token security hardening |

### Reference
| Document | Description |
|----------|-------------|
| [docs/API.md](docs/API.md) | API endpoint documentation |
| [docs/tutorials/](docs/tutorials/) | SIP protocol tutorials and reference |

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

Brian Tafoya ([@btafoya](https://github.com/btafoya))
