# GoSIP REST API Documentation

## Overview

GoSIP provides a RESTful API for managing SIP devices, DIDs, call routing, voicemails, and messages.

**Base URL**: `http://localhost:8080/api`

## Authentication

Most endpoints require authentication via session cookie or Bearer token.

### Login
```http
POST /api/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "your-password"
}
```

**Response**: Sets session cookie and returns user info.

### Logout
```http
POST /api/auth/logout
```

---

## Health Endpoints

### Health Check
```http
GET /health
GET /api/health
```
Returns service health status.

### Readiness Check
```http
GET /api/ready
```
Returns readiness status for load balancer probes.

### Liveness Check
```http
GET /api/live
```
Returns liveness status for container orchestration.

---

## Current User

### Get Current User
```http
GET /api/me
```
Returns the authenticated user's profile.

### Change Password
```http
PUT /api/me/password
Content-Type: application/json

{
  "current_password": "old-password",
  "new_password": "new-password"
}
```

---

## Devices

### List Devices
```http
GET /api/devices
GET /api/devices?limit=20&offset=0
```

### Create Device
```http
POST /api/devices
Content-Type: application/json

{
  "name": "Office Phone",
  "username": "1001",
  "password": "secure-password",
  "type": "grandstream"
}
```

### Get Device
```http
GET /api/devices/{id}
```

### Update Device
```http
PUT /api/devices/{id}
Content-Type: application/json

{
  "name": "Updated Name"
}
```

### Delete Device
```http
DELETE /api/devices/{id}
```

### Get Device Registrations
```http
GET /api/devices/registrations
```
Returns active SIP registrations.

### Get Device Events
```http
GET /api/devices/{id}/events
```
Returns provisioning events for a device.

---

## Provisioning

### Provision Device
```http
POST /api/provisioning/device
Content-Type: application/json

{
  "device_id": 1,
  "profile_id": 1
}
```

### List Vendors
```http
GET /api/provisioning/vendors
```
Returns supported device vendors.

### List Provisioning Tokens
```http
GET /api/provisioning/tokens
```

### Create Provisioning Token
```http
POST /api/provisioning/tokens
Content-Type: application/json

{
  "device_id": 1,
  "expires_hours": 24
}
```

### Get Token QR Code
```http
GET /api/provisioning/tokens/{token}/qrcode
```
Returns QR code image for device provisioning.

### Revoke Token
```http
DELETE /api/provisioning/tokens/{id}
```

### Get Device Config (Public)
```http
GET /api/provision/{token}
```
Public endpoint for device auto-provisioning.

### List Profiles
```http
GET /api/provisioning/profiles
```

### Get Profile
```http
GET /api/provisioning/profiles/{id}
```

### Create Profile (Admin)
```http
POST /api/provisioning/profiles
Content-Type: application/json

{
  "name": "Standard Office",
  "vendor": "grandstream",
  "config": {}
}
```

### Update Profile (Admin)
```http
PUT /api/provisioning/profiles/{id}
```

### Delete Profile (Admin)
```http
DELETE /api/provisioning/profiles/{id}
```

### Get Recent Events
```http
GET /api/provisioning/events
```

---

## DIDs (Phone Numbers)

### List DIDs
```http
GET /api/dids
```

### Create DID
```http
POST /api/dids
Content-Type: application/json

{
  "phone_number": "+15551234567",
  "friendly_name": "Main Line",
  "capabilities": ["voice", "sms"]
}
```

### Sync DIDs from Twilio
```http
POST /api/dids/sync
```
Synchronizes DIDs with Twilio account.

### Get DID
```http
GET /api/dids/{id}
```

### Update DID
```http
PUT /api/dids/{id}
```

### Delete DID
```http
DELETE /api/dids/{id}
```

---

## Routes (Call Routing)

### List Routes
```http
GET /api/routes
```

### Create Route
```http
POST /api/routes
Content-Type: application/json

{
  "name": "Business Hours",
  "did_id": 1,
  "priority": 1,
  "conditions": {
    "time_start": "09:00",
    "time_end": "17:00",
    "days": ["mon", "tue", "wed", "thu", "fri"]
  },
  "action": "forward",
  "action_data": {"device_id": 1}
}
```

### Get Route
```http
GET /api/routes/{id}
```

### Update Route
```http
PUT /api/routes/{id}
```

### Delete Route
```http
DELETE /api/routes/{id}
```

### Reorder Routes
```http
PUT /api/routes/reorder
Content-Type: application/json

{
  "order": [3, 1, 2]
}
```

---

## CDRs (Call Detail Records)

### List CDRs
```http
GET /api/cdrs
GET /api/cdrs?limit=50&offset=0&from_date=2024-01-01&to_date=2024-01-31
```

### Get CDR
```http
GET /api/cdrs/{id}
```

### Get CDR Stats
```http
GET /api/cdrs/stats
GET /api/cdrs/stats?period=day
```
Returns call statistics (count, duration, etc.).

---

## Active Calls

### List Active Calls
```http
GET /api/calls
```

### Get Call
```http
GET /api/calls/{callID}
```

### Put Call on Hold
```http
POST /api/calls/{callID}/hold
Content-Type: application/json

{
  "hold": true
}
```

### Transfer Call
```http
POST /api/calls/{callID}/transfer
Content-Type: application/json

{
  "target": "+15559876543",
  "type": "blind"
}
```

### Cancel Transfer
```http
DELETE /api/calls/{callID}/transfer
```

### Hangup Call
```http
DELETE /api/calls/{callID}
```

### Get Music-on-Hold Status
```http
GET /api/calls/moh
```

### Update Music-on-Hold
```http
PUT /api/calls/moh
Content-Type: application/json

{
  "enabled": true,
  "source": "default"
}
```

### Upload MOH Audio
```http
POST /api/calls/moh/upload
Content-Type: multipart/form-data

file: <audio file>
```

### Validate MOH Audio
```http
POST /api/calls/moh/validate
Content-Type: multipart/form-data

file: <audio file>
```

---

## Voicemails

### List Voicemails
```http
GET /api/voicemails
GET /api/voicemails?limit=20&offset=0&read=false
```

### List Unread Voicemails
```http
GET /api/voicemails/unread
```

### Get Voicemail
```http
GET /api/voicemails/{id}
```
Includes audio URL and transcription.

### Mark as Read
```http
PUT /api/voicemails/{id}/read
```

### Delete Voicemail
```http
DELETE /api/voicemails/{id}
```

---

## MWI (Message Waiting Indicator)

### Get MWI Status
```http
GET /api/mwi/status
```
Returns current voicemail counts and subscription status.

### Trigger MWI Notification
```http
POST /api/mwi/notify
Content-Type: application/json

{
  "did_id": 1
}
```
Forces MWI NOTIFY to be sent to subscribed devices.

---

## Messages (SMS/MMS)

### List Messages
```http
GET /api/messages
GET /api/messages?limit=50&direction=inbound
```

### Send Message
```http
POST /api/messages
Content-Type: application/json

{
  "to": "+15559876543",
  "from_did_id": 1,
  "body": "Hello, world!"
}
```

### Get Message Stats
```http
GET /api/messages/stats
```

### Get Unread Count
```http
GET /api/messages/unread/count
```

### Get Conversations
```http
GET /api/messages/conversations
```
Returns grouped message threads by phone number.

### Get Conversation
```http
GET /api/messages/conversation/{number}
```

### Mark Conversation as Read
```http
PUT /api/messages/conversation/{number}/read
```

### Get Message
```http
GET /api/messages/{id}
```

### Mark Message as Read
```http
PUT /api/messages/{id}/read
```

### Resend Message
```http
POST /api/messages/{id}/resend
```

### Sync Message from Twilio
```http
POST /api/messages/{id}/sync
```

### Cancel Queued Message
```http
POST /api/messages/{id}/cancel
```

### Delete Message
```http
DELETE /api/messages/{id}
```

---

## Blocklist

### List Blocklist
```http
GET /api/blocklist
```

### Add to Blocklist
```http
POST /api/blocklist
Content-Type: application/json

{
  "phone_number": "+15551234567",
  "reason": "spam"
}
```

### Remove from Blocklist
```http
DELETE /api/blocklist/{id}
```

---

## Users (Admin Only)

### List Users
```http
GET /api/users
```

### Create User
```http
POST /api/users
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure-password",
  "role": "user"
}
```

### Get User
```http
GET /api/users/{id}
```

### Update User
```http
PUT /api/users/{id}
```

### Delete User
```http
DELETE /api/users/{id}
```

---

## System (Admin Only)

### Get System Config
```http
GET /api/system/config
```

### Update System Config
```http
PUT /api/system/config
Content-Type: application/json

{
  "twilio_account_sid": "AC...",
  "twilio_auth_token": "...",
  "voicemail_email": "admin@example.com"
}
```

### Get System Status
```http
GET /api/system/status
```
Returns SIP server status, Twilio connection health, etc.

### Create Backup
```http
POST /api/system/backup
```

### Restore Backup
```http
POST /api/system/restore
Content-Type: multipart/form-data

file: <backup file>
```

### Toggle DND (Do Not Disturb)
```http
PUT /api/dnd
Content-Type: application/json

{
  "enabled": true
}
```

---

## TLS/Encryption (Admin Only)

### Get TLS Status
```http
GET /api/system/tls/status
```

### Update TLS Config
```http
PUT /api/system/tls/config
Content-Type: application/json

{
  "enabled": true,
  "auto_renew": true
}
```

### Force Certificate Renewal
```http
POST /api/system/tls/renew
```

### Reload Certificates
```http
POST /api/system/tls/reload
```

### Get Certificate Info
```http
GET /api/system/tls/certificate
```

---

## SRTP (Admin Only)

### Get SRTP Status
```http
GET /api/system/srtp/status
```

### Update SRTP Config
```http
PUT /api/system/srtp/config
Content-Type: application/json

{
  "enabled": true,
  "required": false,
  "key_derivation_rate": 0
}
```

---

## ZRTP (Admin Only)

### Get ZRTP Status
```http
GET /api/system/zrtp/status
```

### Update ZRTP Config
```http
PUT /api/system/zrtp/config
Content-Type: application/json

{
  "enabled": true,
  "strict_mode": false
}
```

### Get ZRTP Sessions
```http
GET /api/system/zrtp/sessions
```

### Get ZRTP SAS
```http
GET /api/system/zrtp/sas
```

### Verify ZRTP SAS
```http
POST /api/system/zrtp/sas/verify
Content-Type: application/json

{
  "session_id": "...",
  "verified": true
}
```

---

## Encryption Status (Admin Only)

### Get Comprehensive Encryption Status
```http
GET /api/system/encryption/status
```
Returns status of TLS, SRTP, ZRTP, and Twilio trunk encryption.

---

## Twilio Trunk TLS (Admin Only)

### Get Trunk TLS Status
```http
GET /api/system/trunks/tls/status
```

### Enable Trunk TLS
```http
POST /api/system/trunks/tls/enable
```

### Migrate Trunk Origination
```http
POST /api/system/trunks/tls/migrate
```

### Create Secure Trunk
```http
POST /api/system/trunks/tls/create
Content-Type: application/json

{
  "name": "Secure SIP Trunk",
  "origination_uri": "sip:secure.example.com:5061;transport=tls"
}
```

---

## Webhooks (Twilio Callbacks)

These endpoints are called by Twilio and are secured by Twilio signature validation.

### Voice Incoming
```http
POST /api/webhooks/voice/incoming
```

### Voice Status
```http
POST /api/webhooks/voice/status
```

### SMS Incoming
```http
POST /api/webhooks/sms/incoming
```

### SMS Status
```http
POST /api/webhooks/sms/status
```

### Recording
```http
POST /api/webhooks/recording
```

### Transcription
```http
POST /api/webhooks/transcription
```

---

## Setup (First Run Only)

### Get Setup Status
```http
GET /api/setup/status
```
Only accessible before initial setup is complete.

### Complete Setup
```http
POST /api/setup/complete
Content-Type: application/json

{
  "admin_email": "admin@example.com",
  "admin_password": "secure-password",
  "twilio_account_sid": "AC...",
  "twilio_auth_token": "..."
}
```

---

## Error Responses

All error responses follow this format:

```json
{
  "error": {
    "code": "validation_error",
    "message": "Validation failed",
    "details": [
      {"field": "email", "message": "Email is required"}
    ]
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `validation_error` | 400 | Input validation failed |
| `bad_request` | 400 | Invalid request format |
| `authentication_error` | 401 | Not authenticated |
| `authorization_error` | 403 | Not authorized |
| `not_found` | 404 | Resource not found |
| `conflict` | 409 | Resource already exists |
| `rate_limited` | 429 | Too many requests |
| `internal_error` | 500 | Internal server error |

---

## Pagination

List endpoints support pagination:

```http
GET /api/messages?limit=20&offset=40
```

Response includes pagination metadata:

```json
{
  "data": [...],
  "pagination": {
    "total": 100,
    "limit": 20,
    "offset": 40
  }
}
```
