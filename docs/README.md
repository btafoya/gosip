# GoSIP Documentation

Welcome to the GoSIP documentation. GoSIP is a SIP-to-Twilio bridge PBX designed for small home office and family use.

---

## Documentation Index

### Getting Started

| Document | Description |
|----------|-------------|
| [Installation Guide](INSTALLATION.md) | System requirements, Docker and manual installation, initial setup |
| [User Guide](USER_GUIDE.md) | End-user documentation for daily use |

### Administration

| Document | Description |
|----------|-------------|
| [Administration Guide](ADMINISTRATION.md) | System administration, user/device management, call routing |
| [Backup & Recovery](BACKUP.md) | Backup strategies, automated backups, disaster recovery |

### Reference

| Document | Description |
|----------|-------------|
| [API Reference](API.md) | REST API documentation for all endpoints |
| [SIP Basics](tutorials/SIP_BASICS.md) | Introduction to SIP protocol concepts |

---

## Quick Start

### 1. Install GoSIP

**Docker (recommended):**
```bash
mkdir -p /opt/gosip && cd /opt/gosip
curl -O https://raw.githubusercontent.com/btafoya/gosip/main/docker-compose.yml
docker compose up -d
```

### 2. Run Setup Wizard

Open `http://your-server:8080` and complete the setup wizard with:
- Twilio Account SID and Auth Token
- Admin email and password

### 3. Configure Twilio

1. Create a SIP trunk pointing to your GoSIP server
2. Configure your DID webhooks to point to GoSIP
3. Sync DIDs in GoSIP

### 4. Add SIP Devices

1. Create devices in Admin → Devices
2. Configure your SIP phones with the provided credentials

---

## Architecture Overview

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

---

## Key Features

- **SIP Server**: UDP/TCP on port 5060 with Digest authentication
- **Device Support**: Grandstream phones, softphones (Zoiper, etc.)
- **Twilio Integration**: SIP trunking, SMS/MMS, voicemail transcription
- **Call Routing**: Time-based, caller ID matching, DND toggle
- **Call Blocking**: Blacklist, anonymous rejection, spam filtering
- **Voicemail**: Recording + transcription, email notification
- **SMS/MMS**: Send/receive, email forwarding
- **Web UI**: Admin dashboard + user portal

---

## System Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 1 core | 2 cores |
| RAM | 512 MB | 1 GB |
| Storage | 1 GB | 10 GB SSD |
| Network | Any | Static IP |

---

## Support

- **Documentation**: This documentation
- **Issues**: [GitHub Issues](https://github.com/btafoya/gosip/issues)
- **Source Code**: [GitHub Repository](https://github.com/btafoya/gosip)

---

**Version**: 1.0
**Last Updated**: December 2025
