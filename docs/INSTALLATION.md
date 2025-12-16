# GoSIP Installation Guide

This guide covers installing and deploying GoSIP, a SIP-to-Twilio bridge PBX for small home office/family use.

---

## Table of Contents

1. [System Requirements](#system-requirements)
2. [Installation Methods](#installation-methods)
3. [Docker Installation (Recommended)](#docker-installation-recommended)
4. [Manual Installation](#manual-installation)
5. [Initial Configuration](#initial-configuration)
6. [Network Configuration](#network-configuration)
7. [Twilio Setup](#twilio-setup)
8. [Verification](#verification)
9. [Troubleshooting Installation](#troubleshooting-installation)

---

## System Requirements

### Minimum Requirements

| Resource | Requirement |
|----------|-------------|
| **CPU** | 1 core (x86_64 or ARM64) |
| **RAM** | 512 MB |
| **Storage** | 1 GB (+ storage for recordings) |
| **Network** | Static IP recommended |
| **OS** | Linux (Ubuntu 20.04+, Debian 11+, Alpine) |

### Recommended for Production

| Resource | Recommendation |
|----------|----------------|
| **CPU** | 2 cores |
| **RAM** | 1 GB |
| **Storage** | 10 GB SSD |
| **Network** | Static public IP with port forwarding |

### Software Prerequisites

- **Docker** 20.10+ and Docker Compose v2 (for Docker installation)
- **Go** 1.21+ (for manual installation)
- **Node.js** 20+ with pnpm (for manual installation)
- **SQLite** 3.35+ (included in most systems)

---

## Installation Methods

GoSIP supports two installation methods:

| Method | Best For | Complexity |
|--------|----------|------------|
| **Docker** | Production, easy updates | Simple |
| **Manual** | Development, customization | Moderate |

---

## Docker Installation (Recommended)

### Step 1: Install Docker

**Ubuntu/Debian:**
```bash
# Install Docker
curl -fsSL https://get.docker.com | sh

# Add your user to docker group
sudo usermod -aG docker $USER

# Install Docker Compose v2 (usually included with Docker)
docker compose version
```

**Verify installation:**
```bash
docker --version
docker compose version
```

### Step 2: Create Project Directory

```bash
# Create directory for GoSIP
mkdir -p /opt/gosip
cd /opt/gosip
```

### Step 3: Create Docker Compose File

Create `docker-compose.yml`:

```yaml
services:
  gosip:
    image: btafoya/gosip:latest
    # Or build from source:
    # build:
    #   context: .
    #   dockerfile: Dockerfile
    container_name: gosip
    restart: unless-stopped
    ports:
      - "8080:8080"       # HTTP API and Web UI
      - "5060:5060/udp"   # SIP UDP
      - "5060:5060/tcp"   # SIP TCP
    volumes:
      - gosip-data:/app/data
    environment:
      - GOSIP_DATA_DIR=/app/data
      - GOSIP_DB_PATH=/app/data/gosip.db
      - GOSIP_HTTP_PORT=8080
      - GOSIP_SIP_PORT=5060
      - GOSIP_LOG_LEVEL=info
      - GOSIP_EXTERNAL_IP=${GOSIP_EXTERNAL_IP:-}
      - TZ=${TZ:-America/New_York}
    networks:
      - gosip-net
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

volumes:
  gosip-data:
    driver: local

networks:
  gosip-net:
    driver: bridge
```

### Step 4: Configure Environment

Create `.env` file for environment-specific settings:

```bash
# External IP for SIP (required for NAT traversal)
GOSIP_EXTERNAL_IP=your.public.ip.address

# Timezone
TZ=America/New_York
```

### Step 5: Start GoSIP

```bash
# Start the container
docker compose up -d

# Check logs
docker compose logs -f

# Verify it's running
docker compose ps
```

### Step 6: Access Web Interface

Open your browser and navigate to:
```
http://your-server-ip:8080
```

You'll be presented with the Setup Wizard on first run.

---

## Manual Installation

### Step 1: Install Prerequisites

**Ubuntu/Debian:**
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Go
wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Install Node.js and pnpm
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs
npm install -g pnpm

# Install build dependencies
sudo apt install -y gcc sqlite3 libsqlite3-dev git
```

### Step 2: Clone Repository

```bash
git clone https://github.com/btafoya/gosip.git
cd gosip
```

### Step 3: Build Application

```bash
# Install dependencies and build
make setup
make build
```

This will:
1. Download Go dependencies
2. Install frontend dependencies
3. Build the Vue frontend
4. Compile the Go backend

### Step 4: Create Data Directories

```bash
mkdir -p data/recordings data/voicemails data/backups
```

### Step 5: Run GoSIP

**Direct execution:**
```bash
./bin/gosip
```

**With environment variables:**
```bash
GOSIP_DATA_DIR=./data \
GOSIP_HTTP_PORT=8080 \
GOSIP_SIP_PORT=5060 \
GOSIP_LOG_LEVEL=info \
./bin/gosip
```

### Step 6: Set Up as Service (Recommended)

Create systemd service file `/etc/systemd/system/gosip.service`:

```ini
[Unit]
Description=GoSIP SIP-to-Twilio Bridge PBX
After=network.target

[Service]
Type=simple
User=gosip
Group=gosip
WorkingDirectory=/opt/gosip
ExecStart=/opt/gosip/bin/gosip
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

# Environment
Environment=GOSIP_DATA_DIR=/opt/gosip/data
Environment=GOSIP_HTTP_PORT=8080
Environment=GOSIP_SIP_PORT=5060
Environment=GOSIP_LOG_LEVEL=info

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/gosip/data

[Install]
WantedBy=multi-user.target
```

**Enable and start the service:**
```bash
# Create service user
sudo useradd -r -s /bin/false gosip

# Set permissions
sudo chown -R gosip:gosip /opt/gosip

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable gosip
sudo systemctl start gosip

# Check status
sudo systemctl status gosip
```

---

## Initial Configuration

### Setup Wizard

On first access, the Setup Wizard will guide you through:

1. **Twilio Credentials**
   - Account SID (starts with `AC`)
   - Auth Token

2. **Admin Account**
   - Email address
   - Password (minimum 8 characters)

3. **Optional: Email Notifications**
   - SMTP server settings
   - Or Postmark API key

4. **Optional: Push Notifications**
   - Gotify server URL
   - Application token

### API-Based Setup

Alternatively, complete setup via API:

```bash
curl -X POST http://localhost:8080/api/setup/complete \
  -H "Content-Type: application/json" \
  -d '{
    "admin_email": "admin@example.com",
    "admin_password": "your-secure-password",
    "twilio_account_sid": "ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "twilio_auth_token": "your-auth-token"
  }'
```

---

## Network Configuration

### Required Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | TCP | Web UI and REST API |
| 5060 | UDP | SIP signaling (primary) |
| 5060 | TCP | SIP signaling (alternative) |
| 5061 | TCP | SIP over TLS (if enabled) |
| 10000-20000 | UDP | RTP media (if not using Twilio media) |

### Firewall Configuration

**UFW (Ubuntu):**
```bash
sudo ufw allow 8080/tcp
sudo ufw allow 5060/udp
sudo ufw allow 5060/tcp
sudo ufw allow 10000:20000/udp
```

**firewalld (CentOS/RHEL):**
```bash
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=5060/udp
sudo firewall-cmd --permanent --add-port=5060/tcp
sudo firewall-cmd --permanent --add-port=10000-20000/udp
sudo firewall-cmd --reload
```

### NAT/Router Configuration

If GoSIP is behind a NAT router:

1. **Port Forward** the above ports to your GoSIP server
2. **Set External IP** in environment:
   ```bash
   GOSIP_EXTERNAL_IP=your.public.ip.address
   ```
3. **Configure STUN** in your SIP devices if needed

### Reverse Proxy (Optional)

For HTTPS on the web interface, use nginx:

```nginx
server {
    listen 443 ssl http2;
    server_name pbx.example.com;

    ssl_certificate /etc/letsencrypt/live/pbx.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pbx.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

**Note:** The reverse proxy is only for the web UI. SIP traffic should go directly to GoSIP.

---

## Twilio Setup

### Step 1: Create Twilio Account

1. Sign up at [twilio.com](https://www.twilio.com)
2. Note your **Account SID** and **Auth Token** from the dashboard

### Step 2: Purchase Phone Number(s)

1. Go to **Phone Numbers** → **Buy a Number**
2. Select a number with Voice and SMS capabilities
3. Note the phone number (DID)

### Step 3: Configure SIP Trunk

1. Go to **Elastic SIP Trunking** → **Trunks**
2. Create a new trunk
3. Configure **Origination**:
   - URI: `sip:your-gosip-server:5060`
   - Priority: 10
   - Weight: 10

4. Configure **Termination**:
   - Termination SIP URI (for outbound calls)
   - Authentication credentials

### Step 4: Configure Phone Number Webhook

1. Go to **Phone Numbers** → Select your number
2. Configure **Voice & Fax**:
   - **A CALL COMES IN**: Webhook
   - URL: `http://your-gosip-server:8080/api/webhooks/voice/incoming`
   - Method: POST

3. Configure **Messaging**:
   - **A MESSAGE COMES IN**: Webhook
   - URL: `http://your-gosip-server:8080/api/webhooks/sms/incoming`
   - Method: POST

### Step 5: Sync DIDs in GoSIP

After configuring Twilio, sync your DIDs:

1. Login to GoSIP web UI
2. Go to **Admin** → **DIDs**
3. Click **Sync from Twilio**

---

## Verification

### Check Service Health

**Via API:**
```bash
curl http://localhost:8080/api/health
```

Expected response:
```json
{"status": "healthy"}
```

**Via Docker:**
```bash
docker compose ps
# Should show "healthy" status
```

### Check System Status

```bash
curl http://localhost:8080/api/system/status
```

Expected response includes:
- SIP server status: "online"
- Twilio status: "healthy"
- Database status: "healthy"

### Test SIP Registration

1. Configure a SIP phone with:
   - Server: your-gosip-server
   - Port: 5060
   - Username: (from device configuration)
   - Password: (from device configuration)

2. Check registration in GoSIP:
   - Web UI: **Devices** → Device should show "Online"
   - API: `GET /api/devices/registrations`

### Test Inbound Call

1. Call your Twilio number from an external phone
2. Your SIP device should ring
3. Check CDR in GoSIP after the call

---

## Troubleshooting Installation

### Container Won't Start

**Check logs:**
```bash
docker compose logs gosip
```

**Common issues:**
- Port already in use: Change ports in docker-compose.yml
- Permission denied: Check volume permissions

### SIP Devices Not Registering

1. **Check firewall**: Ensure port 5060 is open
2. **Check NAT**: Set GOSIP_EXTERNAL_IP if behind NAT
3. **Check credentials**: Verify device username/password

**Debug SIP traffic:**
```bash
# Inside container
tcpdump -i any port 5060 -w /tmp/sip.pcap
```

### Can't Access Web UI

1. **Check if running**: `docker compose ps` or `systemctl status gosip`
2. **Check firewall**: Ensure port 8080 is accessible
3. **Check logs**: Look for binding errors

### Database Errors

**Reset database (WARNING: loses all data):**
```bash
# Docker
docker compose down -v
docker compose up -d

# Manual
rm data/gosip.db
./bin/gosip  # Will recreate on startup
```

### Twilio Webhooks Not Working

1. **Check URL accessibility**: Twilio must reach your server
2. **Check webhook URL**: Correct path and port
3. **Check Twilio logs**: Console → Debugger

---

## Next Steps

After installation:

1. **Configure SIP Devices**: See [User Guide](USER_GUIDE.md)
2. **Set Up Call Routing**: See [Administration Guide](ADMINISTRATION.md)
3. **Configure Backups**: See [Backup & Recovery](BACKUP.md)

---

**Version**: 1.0
**Last Updated**: December 2025
