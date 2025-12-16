# GoSIP Administration Guide

This guide covers system administration, configuration, and management of your GoSIP PBX system.

---

## Table of Contents

1. [Admin Dashboard Overview](#admin-dashboard-overview)
2. [User Management](#user-management)
3. [Device Management](#device-management)
4. [DID (Phone Number) Management](#did-phone-number-management)
5. [Call Routing](#call-routing)
6. [Blocklist Management](#blocklist-management)
7. [System Configuration](#system-configuration)
8. [Security Settings](#security-settings)
9. [Monitoring & Logs](#monitoring--logs)
10. [Maintenance Tasks](#maintenance-tasks)

---

## Admin Dashboard Overview

### Accessing Admin Panel

1. Login to GoSIP web UI at `http://your-server:8080`
2. Use admin credentials created during setup
3. Admin-only sections appear in the navigation menu

### Dashboard Widgets

The admin dashboard displays:

| Widget | Description |
|--------|-------------|
| **System Status** | SIP server, Twilio, database health |
| **Active Calls** | Currently active calls count |
| **Registered Devices** | Online SIP devices |
| **Recent Activity** | Latest calls, messages, voicemails |
| **Storage Usage** | Recordings and voicemail storage |

---

## User Management

### User Roles

GoSIP supports two user roles:

| Role | Permissions |
|------|-------------|
| **Admin** | Full system access, configuration, user management |
| **User** | Access own devices, calls, voicemails, messages |

### Creating Users

**Via Web UI:**
1. Go to **Admin** → **Users**
2. Click **Add User**
3. Enter email, password, and role
4. Assign DIDs and devices (optional)

**Via API:**
```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "email": "user@example.com",
    "password": "secure-password",
    "role": "user"
  }'
```

### Managing Users

| Action | How To |
|--------|--------|
| **View Users** | Admin → Users |
| **Edit User** | Click user → Edit |
| **Reset Password** | Click user → Reset Password |
| **Disable User** | Click user → Disable |
| **Delete User** | Click user → Delete (irreversible) |

### Password Requirements

- Minimum 8 characters
- At least one uppercase letter
- At least one number

### Session Security

| Setting | Value | Description |
|---------|-------|-------------|
| Session Duration | 24 hours | Auto-logout after inactivity |
| Max Failed Logins | 5 | Account lockout threshold |
| Lockout Duration | 15 minutes | Time before retry allowed |

---

## Device Management

### Adding SIP Devices

**Via Web UI:**
1. Go to **Admin** → **Devices**
2. Click **Add Device**
3. Configure:
   - **Name**: Friendly name (e.g., "Office Phone")
   - **Username**: SIP username (e.g., "1001")
   - **Password**: SIP password (auto-generated or custom)
   - **Type**: Device model (Grandstream, Generic, etc.)
   - **User**: Assign to a user account

**Via API:**
```bash
curl -X POST http://localhost:8080/api/devices \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "name": "Office Phone",
    "username": "1001",
    "password": "secure-sip-password",
    "type": "grandstream"
  }'
```

### Device Provisioning

GoSIP supports auto-provisioning for supported devices:

1. Go to **Admin** → **Provisioning**
2. Create or select a provisioning profile
3. Generate provisioning token
4. Configure device with provisioning URL

**Provisioning URL Format:**
```
http://your-server:8080/api/provision/{token}
```

**QR Code Provisioning:**
- Generate QR code from Admin → Provisioning
- Scan with supported softphones

### Supported Device Types

| Type | Auto-Provision | Notes |
|------|----------------|-------|
| Grandstream GXP | Yes | Full support |
| Generic SIP | No | Manual configuration |
| Softphones | Partial | QR code supported |

### Monitoring Device Status

| Status | Meaning |
|--------|---------|
| **Online** (Green) | Registered and ready |
| **Offline** (Gray) | Not registered |
| **Ringing** | Currently receiving call |
| **On Call** | Active call in progress |

### Device Configuration for Users

**SIP Settings to provide:**
```
SIP Server:    your-gosip-server.com
SIP Port:      5060 (UDP) or 5061 (TLS)
Username:      [device username]
Password:      [device password]
Transport:     UDP (or TCP/TLS)
Auth User:     [same as username]
```

---

## DID (Phone Number) Management

### Syncing DIDs from Twilio

1. Go to **Admin** → **DIDs**
2. Click **Sync from Twilio**
3. Imported DIDs will appear in the list

**Via API:**
```bash
curl -X POST http://localhost:8080/api/dids/sync \
  -H "Cookie: session=your-session-cookie"
```

### Configuring DIDs

Each DID can be configured with:

| Setting | Description |
|---------|-------------|
| **Friendly Name** | Display name (e.g., "Main Office") |
| **Capabilities** | Voice, SMS, MMS, Fax |
| **Default Route** | Where calls go by default |
| **Voicemail** | Enable/disable voicemail for this DID |
| **Assigned User** | User who owns this DID |

### DID Assignment

DIDs can be assigned to:
- **Users**: Primary number for a user
- **Devices**: Direct routing to a device
- **Routes**: Custom routing rules

---

## Call Routing

### Route Priority

Routes are evaluated in priority order (lower number = higher priority). First matching route wins.

### Creating Routes

**Via Web UI:**
1. Go to **Admin** → **Routes**
2. Click **Add Route**
3. Configure route conditions and actions

### Route Conditions

| Condition | Description | Example |
|-----------|-------------|---------|
| **Time Range** | Specific hours | 9:00-17:00 |
| **Days of Week** | Specific days | Mon-Fri |
| **Caller ID** | Match caller number | +1555* |
| **DID** | Specific incoming number | +15551234567 |

### Route Actions

| Action | Description |
|--------|-------------|
| **Forward to Device** | Ring a specific device |
| **Forward to User** | Ring all user's devices |
| **Forward to External** | Ring an external number |
| **Voicemail** | Send directly to voicemail |
| **Play Announcement** | Play audio, then route |
| **Reject** | Reject the call |

### Example Routing Configurations

**Business Hours Routing:**
```json
{
  "name": "Business Hours",
  "did_id": 1,
  "priority": 10,
  "conditions": {
    "time_start": "09:00",
    "time_end": "17:00",
    "days": ["mon", "tue", "wed", "thu", "fri"]
  },
  "action": "forward",
  "action_data": {"device_id": 1}
}
```

**After Hours Voicemail:**
```json
{
  "name": "After Hours",
  "did_id": 1,
  "priority": 20,
  "conditions": {},
  "action": "voicemail",
  "action_data": {"user_id": 1}
}
```

**VIP Caller:**
```json
{
  "name": "VIP Caller",
  "did_id": 1,
  "priority": 5,
  "conditions": {
    "caller_id_pattern": "+15559876543"
  },
  "action": "forward",
  "action_data": {"device_id": 2}
}
```

### Reordering Routes

Drag and drop in the web UI, or use API:
```bash
curl -X PUT http://localhost:8080/api/routes/reorder \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{"order": [3, 1, 2]}'
```

---

## Blocklist Management

### Adding Numbers to Blocklist

**Via Web UI:**
1. Go to **Admin** → **Blocklist**
2. Click **Add Number**
3. Enter phone number and reason

**Via API:**
```bash
curl -X POST http://localhost:8080/api/blocklist \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "phone_number": "+15551234567",
    "reason": "spam"
  }'
```

### Blocklist Features

| Feature | Description |
|---------|-------------|
| **Exact Match** | Block specific number |
| **Pattern Match** | Block number patterns (e.g., +1555*) |
| **Reason Tracking** | Record why number was blocked |
| **Block Anonymous** | Block calls with no caller ID |

### Managing Blocked Numbers

| Action | Description |
|--------|-------------|
| **View** | See all blocked numbers |
| **Remove** | Unblock a number |
| **Export** | Download blocklist as CSV |
| **Import** | Bulk import from CSV |

---

## System Configuration

### Twilio Settings

**Via Web UI:** Admin → Settings → Twilio

| Setting | Description |
|---------|-------------|
| **Account SID** | Your Twilio Account SID |
| **Auth Token** | Your Twilio Auth Token |

**Via API:**
```bash
curl -X PUT http://localhost:8080/api/system/config \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "twilio_account_sid": "ACxxxxxxxx",
    "twilio_auth_token": "your-token"
  }'
```

### Email Notification Settings

| Setting | Description |
|---------|-------------|
| **SMTP Host** | Mail server address |
| **SMTP Port** | Mail server port (587, 465, 25) |
| **SMTP User** | Authentication username |
| **SMTP Password** | Authentication password |

### Gotify Push Notifications

| Setting | Description |
|---------|-------------|
| **Gotify URL** | Gotify server URL |
| **Gotify Token** | Application token |

### Voicemail Settings

| Setting | Default | Description |
|---------|---------|-------------|
| **Voicemail Enabled** | true | Global voicemail on/off |
| **Voicemail Greeting** | Default | Custom greeting text |
| **Ring Timeout** | 30 seconds | Rings before voicemail |
| **Max Length** | 180 seconds | Maximum recording length |
| **Transcription** | true | Enable speech-to-text |

### Recording Settings

| Setting | Description |
|---------|-------------|
| **Recording Enabled** | Enable/disable call recording |
| **Storage Path** | Location for recordings |
| **Retention Days** | Auto-delete after X days |

### Timezone

Set system timezone for time-based routing:
```bash
TZ=America/New_York
```

---

## Security Settings

### TLS/Encryption

**Enable SIP over TLS:**

1. Go to **Admin** → **Security** → **TLS**
2. Enable TLS
3. Configure certificate (auto or manual)

**API:**
```bash
curl -X PUT http://localhost:8080/api/system/tls/config \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "enabled": true,
    "auto_renew": true
  }'
```

### SRTP (Secure RTP)

Enable encrypted media:
```bash
curl -X PUT http://localhost:8080/api/system/srtp/config \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{
    "enabled": true,
    "required": false
  }'
```

### Security Best Practices

1. **Use TLS** for SIP signaling when possible
2. **Strong Passwords** for all accounts and devices
3. **Firewall** - Only allow necessary ports
4. **Regular Updates** - Keep GoSIP updated
5. **Backup** - Regular automated backups
6. **Monitor** - Watch for unusual activity

---

## Monitoring & Logs

### System Status

**Check overall system health:**
```bash
curl http://localhost:8080/api/system/status
```

**Response includes:**
- SIP server status
- Twilio connectivity
- Database health
- Active calls
- Registered devices

### Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/api/health` | Basic health check |
| `/api/ready` | Readiness probe (Kubernetes) |
| `/api/live` | Liveness probe (Kubernetes) |

### Call Detail Records (CDRs)

**View call history:**
- Web UI: **Admin** → **Call History**
- API: `GET /api/cdrs`

**Filter options:**
- Date range
- Direction (inbound/outbound)
- DID
- User

**CDR Statistics:**
```bash
curl http://localhost:8080/api/cdrs/stats?period=day
```

### Docker Logs

```bash
# View logs
docker compose logs gosip

# Follow logs
docker compose logs -f gosip

# Last 100 lines
docker compose logs --tail=100 gosip
```

### Log Levels

Configure log verbosity:
```bash
GOSIP_LOG_LEVEL=debug  # debug, info, warn, error
```

---

## Maintenance Tasks

### Regular Maintenance Checklist

| Task | Frequency | Description |
|------|-----------|-------------|
| **Backup Database** | Daily | See Backup Guide |
| **Check Disk Space** | Weekly | Monitor recordings storage |
| **Review CDRs** | Weekly | Look for anomalies |
| **Update Software** | Monthly | Check for updates |
| **Review Logs** | Weekly | Check for errors |
| **Clean Old Recordings** | Monthly | Remove old data |

### Updating GoSIP

**Docker:**
```bash
# Pull latest image
docker compose pull

# Recreate container with new image
docker compose up -d

# Verify
docker compose ps
docker compose logs gosip
```

**Manual Installation:**
```bash
cd /opt/gosip
git pull
make build
sudo systemctl restart gosip
```

### Database Maintenance

**Vacuum Database (reclaim space):**
```bash
# Docker
docker compose exec gosip sqlite3 /app/data/gosip.db "VACUUM;"

# Manual
sqlite3 data/gosip.db "VACUUM;"
```

**Check Database Integrity:**
```bash
sqlite3 data/gosip.db "PRAGMA integrity_check;"
```

### Cleaning Old Data

**Delete old recordings:**
```bash
# Delete recordings older than 90 days
find /app/data/recordings -type f -mtime +90 -delete
```

**Delete old voicemails:**
```bash
# Delete voicemails older than 180 days
find /app/data/voicemails -type f -mtime +180 -delete
```

### Service Management

**Docker:**
```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart
docker compose restart

# View status
docker compose ps
```

**Systemd:**
```bash
# Start
sudo systemctl start gosip

# Stop
sudo systemctl stop gosip

# Restart
sudo systemctl restart gosip

# View status
sudo systemctl status gosip

# View logs
sudo journalctl -u gosip -f
```

---

## Performance Tuning

### Configuration Constants

These values are optimized for small deployments (2-5 phones):

| Setting | Value | Notes |
|---------|-------|-------|
| MaxConcurrentCalls | 5 | Increase for more capacity |
| SIPRegistrationTimeout | 500ms | Registration response time |
| CallSetupTimeout | 2s | Time to establish call |
| DefaultPageSize | 20 | API pagination |
| MaxPageSize | 100 | Maximum API results |

### Resource Monitoring

**CPU and Memory (Docker):**
```bash
docker stats gosip
```

**Database Size:**
```bash
ls -lh data/gosip.db
```

**Storage Usage:**
```bash
du -sh data/recordings
du -sh data/voicemails
```

---

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| **SIP devices not registering** | Check firewall, verify credentials |
| **Calls not routing** | Check route priorities, verify DID configuration |
| **No audio** | Check NAT settings, verify EXTERNAL_IP |
| **Webhook failures** | Ensure server is publicly accessible |
| **High CPU** | Check log level, reduce concurrent calls |

### Getting Help

1. **Check Logs**: Most issues appear in logs
2. **API Health**: Verify `/api/health` returns healthy
3. **Twilio Debugger**: Check Twilio console for webhook errors
4. **SIP Trace**: Enable debug logging for SIP issues

---

**Version**: 1.0
**Last Updated**: December 2025
