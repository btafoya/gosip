# GoSIP

A full-featured SIP-to-Twilio bridge PBX with web-based management for home office and small family use.

## Overview

GoSIP connects SIP phones to Twilio's cloud services, providing professional telephony features without complex PBX infrastructure.

**Scale**: 2-5 phones, 1-3 DIDs

## Features

- **SIP Server** - UDP/TCP on port 5060 with Digest authentication (< 500ms registration)
- **Device Support** - Grandstream GXP1760W, softphones (Onesip, Zoiper, Linphone)
- **Call Routing** - Time-based rules, caller ID matching, DND toggle
- **Call Blocking** - Blacklist, anonymous rejection, spam filtering (score > 0.7 blocked)
- **Voicemail** - Twilio recording + transcription, email notification (30s ring, 3min max)
- **Call Recording** - On-demand, local storage with storage monitoring
- **SMS/MMS** - Send/receive, email forwarding, auto-reply
- **Web UI** - Admin dashboard + user portal with setup wizard
- **Security** - 5 failed logins → 15-min lockout, 24-hour sessions
- **Resilience** - Auto-retry with exponential backoff, request queuing

## Tech Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.21+ |
| Frontend | Vue 3 + Tailwind CSS |
| Database | SQLite |
| SIP | [sipgo](https://github.com/emiago/sipgo) |
| Deployment | Docker Compose v2 |
| Cloud | Twilio (SIP, SMS, Transcription) |
| Notifications | Gotify (push), SMTP/Postmarkapp (email) |

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+ with pnpm
- Docker and Docker Compose v2
- Twilio account with SIP Trunking

### Development

```bash
# Clone repository
git clone https://github.com/btafoya/gosip.git
cd gosip

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

## Performance SLAs

| Metric | Target |
|--------|--------|
| SIP Registration | < 500ms |
| Call Setup | < 2 seconds |
| API Response (GET) | < 200ms (95th percentile) |
| API Response (POST) | < 500ms (95th percentile) |
| Concurrent Calls | 5 minimum |
| System Startup | < 30 seconds |

## Documentation

| Document | Description |
|----------|-------------|
| [REQUIREMENTS.md](REQUIREMENTS.md) | Complete functional and technical specification |
| [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) | 12-phase development roadmap with P0 specifications |
| [SCOPE_ALIGNMENT.md](SCOPE_ALIGNMENT.md) | Documentation alignment and scope analysis |
| [SPEC_PANEL_REVIEW.md](SPEC_PANEL_REVIEW.md) | Expert specification review (7.2/10 score) |
| [CLAUDE.md](CLAUDE.md) | AI assistant guidelines |
| [docs/tutorials/](docs/tutorials/) | SIP protocol tutorials and reference |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Vue 3 + Tailwind CSS                     │
├─────────────────────────────────────────────────────────────┤
│                       Go REST API                           │
├──────────┬──────────┬──────────┬──────────┬────────────────┤
│   SIP    │  Rules   │  Twilio  │ Voicemail│   Webhook      │
│  Server  │  Engine  │   API    │  Handler │   API          │
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
│   ├── auth/            # Authentication
│   ├── config/          # Runtime configuration
│   ├── db/              # SQLite repository layer
│   ├── models/          # Domain models
│   ├── rules/           # Call routing engine
│   ├── twilio/          # Twilio API client
│   └── webhooks/        # Twilio webhook handlers
├── pkg/sip/             # SIP integration (sipgo wrapper)
├── frontend/            # Vue 3 SPA
├── migrations/          # SQLite schema migrations
├── docs/                # Documentation and tutorials
├── docker-compose.yml
└── Dockerfile
```

## Failure Handling

| Component | Failure Mode | Response |
|-----------|--------------|----------|
| Twilio API | Unreachable/5xx | Queue requests, retry with backoff, alert after 3 failures |
| Twilio API | Rate limited (429) | Backoff and queue, auto-retry after cooldown |
| SIP Device | Offline mid-call | Maintain call until BYE/timeout |
| Email (SMTP) | Unreachable | Retry 3x over 1 hour, then abandon |
| Push (Gotify) | Unreachable | Silent fail after 3 attempts |
| Recording Storage | Full | Continue call, skip recording, alert admin |

## API Endpoints

- `/api/auth/*` - Authentication
- `/api/devices/*` - SIP device management
- `/api/dids/*` - Phone number management
- `/api/routes/*` - Call routing rules
- `/api/cdrs/*` - Call history
- `/api/voicemails/*` - Voicemail access
- `/api/messages/*` - SMS/MMS
- `/api/webhooks/*` - Twilio callbacks
- `/api/system/*` - Admin configuration

## License

MIT

## Author

Brian Tafoya
