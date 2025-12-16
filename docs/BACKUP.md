# GoSIP Backup & Recovery Guide

This guide covers backup strategies, disaster recovery procedures, and data protection for your GoSIP PBX system.

---

## Table of Contents

1. [Backup Overview](#backup-overview)
2. [What Gets Backed Up](#what-gets-backed-up)
3. [Manual Backup](#manual-backup)
4. [Automated Backups](#automated-backups)
5. [Backup Storage](#backup-storage)
6. [Restore Procedures](#restore-procedures)
7. [Disaster Recovery](#disaster-recovery)
8. [Backup Best Practices](#backup-best-practices)

---

## Backup Overview

### Backup Strategy

GoSIP requires backing up:

| Component | Priority | Frequency |
|-----------|----------|-----------|
| **Database** (gosip.db) | Critical | Daily |
| **Configuration** | Critical | After changes |
| **Voicemails** | High | Daily |
| **Recordings** | Medium | Weekly |

### Backup Methods

| Method | Best For | Automation |
|--------|----------|------------|
| **API Backup** | Database only | Scriptable |
| **File System Backup** | Complete backup | External tools |
| **Docker Volume Backup** | Docker deployments | Docker commands |

---

## What Gets Backed Up

### Data Directory Structure

```
data/
├── gosip.db          # SQLite database (CRITICAL)
├── recordings/       # Call recordings
├── voicemails/       # Voicemail audio files
└── backups/          # API-generated backups
```

### Database Contents

The SQLite database (`gosip.db`) contains:

| Table | Description |
|-------|-------------|
| `config` | System configuration |
| `users` | User accounts |
| `devices` | SIP device configurations |
| `registrations` | Active SIP registrations |
| `dids` | Phone numbers |
| `routes` | Call routing rules |
| `blocklist` | Blocked numbers |
| `cdrs` | Call detail records |
| `voicemails` | Voicemail metadata |
| `messages` | SMS/MMS records |
| `auto_replies` | Auto-reply configurations |

### Media Files

| Directory | Contents | Size Estimate |
|-----------|----------|---------------|
| `recordings/` | Call recordings (WAV/MP3) | ~1 MB per minute |
| `voicemails/` | Voicemail audio | ~100 KB per message |

---

## Manual Backup

### Method 1: API Backup (Database Only)

Create a database backup via the web UI or API:

**Via Web UI:**
1. Login as admin
2. Go to **Admin** → **System** → **Backup**
3. Click **Create Backup**
4. Download the backup file

**Via API:**
```bash
# Create backup
curl -X POST http://localhost:8080/api/system/backup \
  -H "Cookie: session=your-session-cookie"

# Response:
# {
#   "filename": "backup_20251215_143022.db",
#   "size": 524288,
#   "created_at": "2025-12-15T14:30:22Z"
# }
```

The backup file is created in `data/backups/`.

### Method 2: File System Backup (Complete)

For a complete backup including media files:

**Docker Deployment:**
```bash
# Stop container (recommended for consistency)
docker compose stop gosip

# Backup the entire data volume
docker run --rm \
  -v gosip_gosip-data:/data:ro \
  -v $(pwd)/backups:/backup \
  alpine tar czf /backup/gosip-backup-$(date +%Y%m%d).tar.gz -C /data .

# Restart container
docker compose start gosip
```

**Manual Installation:**
```bash
# Stop service (recommended)
sudo systemctl stop gosip

# Create backup
tar -czf /backup/gosip-backup-$(date +%Y%m%d).tar.gz -C /opt/gosip/data .

# Restart service
sudo systemctl start gosip
```

### Method 3: Hot Backup (Database Only, No Downtime)

SQLite supports hot backups using the `.backup` command:

```bash
# Docker
docker compose exec gosip sqlite3 /app/data/gosip.db ".backup /app/data/backups/hot-backup.db"

# Manual
sqlite3 /opt/gosip/data/gosip.db ".backup /backup/hot-backup.db"
```

Or using VACUUM INTO:
```bash
sqlite3 /opt/gosip/data/gosip.db "VACUUM INTO '/backup/vacuum-backup.db';"
```

---

## Automated Backups

### Cron-Based Backup Script

Create `/opt/gosip/scripts/backup.sh`:

```bash
#!/bin/bash
#
# GoSIP Backup Script
# Run via cron: 0 2 * * * /opt/gosip/scripts/backup.sh
#

# Configuration
BACKUP_DIR="/backup/gosip"
DATA_DIR="/opt/gosip/data"
RETENTION_DAYS=30
DATE=$(date +%Y%m%d_%H%M%S)

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Log function
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log "Starting GoSIP backup..."

# 1. Database backup (hot backup)
log "Backing up database..."
sqlite3 "$DATA_DIR/gosip.db" ".backup $BACKUP_DIR/gosip_db_$DATE.db"

if [ $? -eq 0 ]; then
    log "Database backup successful"
    gzip "$BACKUP_DIR/gosip_db_$DATE.db"
else
    log "ERROR: Database backup failed!"
    exit 1
fi

# 2. Voicemail backup
log "Backing up voicemails..."
tar -czf "$BACKUP_DIR/voicemails_$DATE.tar.gz" -C "$DATA_DIR" voicemails/ 2>/dev/null

# 3. Recordings backup (optional - can be large)
if [ -d "$DATA_DIR/recordings" ] && [ "$(ls -A $DATA_DIR/recordings)" ]; then
    log "Backing up recordings..."
    tar -czf "$BACKUP_DIR/recordings_$DATE.tar.gz" -C "$DATA_DIR" recordings/ 2>/dev/null
fi

# 4. Clean old backups
log "Cleaning backups older than $RETENTION_DAYS days..."
find "$BACKUP_DIR" -type f -mtime +$RETENTION_DAYS -delete

# 5. Calculate total backup size
BACKUP_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)
log "Total backup size: $BACKUP_SIZE"

log "Backup completed successfully"
```

**Make executable and set up cron:**
```bash
chmod +x /opt/gosip/scripts/backup.sh

# Add to crontab (daily at 2 AM)
crontab -e
0 2 * * * /opt/gosip/scripts/backup.sh >> /var/log/gosip-backup.log 2>&1
```

### Docker Backup Script

Create `backup-docker.sh`:

```bash
#!/bin/bash
#
# GoSIP Docker Backup Script
#

BACKUP_DIR="/backup/gosip"
DATE=$(date +%Y%m%d_%H%M%S)
CONTAINER_NAME="gosip"

mkdir -p "$BACKUP_DIR"

echo "Starting GoSIP Docker backup..."

# Database backup without stopping container
docker exec $CONTAINER_NAME sqlite3 /app/data/gosip.db ".backup /app/data/backups/backup_$DATE.db"

# Copy backup from container
docker cp $CONTAINER_NAME:/app/data/backups/backup_$DATE.db "$BACKUP_DIR/gosip_db_$DATE.db"
gzip "$BACKUP_DIR/gosip_db_$DATE.db"

# Full volume backup (optional - stops container briefly)
# docker compose stop
# docker run --rm -v gosip_gosip-data:/data:ro -v $BACKUP_DIR:/backup alpine \
#     tar czf /backup/gosip-full-$DATE.tar.gz -C /data .
# docker compose start

# Clean old backups
find "$BACKUP_DIR" -type f -mtime +30 -delete

echo "Backup completed: gosip_db_$DATE.db.gz"
```

### Systemd Timer (Alternative to Cron)

Create `/etc/systemd/system/gosip-backup.service`:
```ini
[Unit]
Description=GoSIP Backup Service
After=gosip.service

[Service]
Type=oneshot
ExecStart=/opt/gosip/scripts/backup.sh
User=gosip
Group=gosip
```

Create `/etc/systemd/system/gosip-backup.timer`:
```ini
[Unit]
Description=Daily GoSIP Backup

[Timer]
OnCalendar=*-*-* 02:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

**Enable timer:**
```bash
sudo systemctl daemon-reload
sudo systemctl enable gosip-backup.timer
sudo systemctl start gosip-backup.timer

# Check status
sudo systemctl list-timers gosip-backup.timer
```

---

## Backup Storage

### Local Storage

| Location | Pros | Cons |
|----------|------|------|
| Same server | Fast, simple | Lost if server fails |
| NFS mount | Network accessible | Single point of failure |
| External drive | Offline capable | Manual management |

### Remote/Cloud Storage

**Rsync to remote server:**
```bash
rsync -avz --delete /backup/gosip/ backup-server:/backup/gosip/
```

**S3-compatible storage:**
```bash
# Install rclone
curl https://rclone.org/install.sh | sudo bash

# Configure (run once)
rclone config

# Upload backups
rclone sync /backup/gosip remote:gosip-backups/
```

**Sample rclone cron job:**
```bash
# After local backup, sync to cloud
30 3 * * * rclone sync /backup/gosip s3:my-bucket/gosip-backups/ --log-file=/var/log/rclone.log
```

### Backup Verification

Always verify backups are valid:

```bash
#!/bin/bash
# verify-backup.sh

BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file.db.gz>"
    exit 1
fi

# Decompress if needed
if [[ "$BACKUP_FILE" == *.gz ]]; then
    gunzip -k "$BACKUP_FILE"
    DB_FILE="${BACKUP_FILE%.gz}"
else
    DB_FILE="$BACKUP_FILE"
fi

# Verify integrity
echo "Checking database integrity..."
RESULT=$(sqlite3 "$DB_FILE" "PRAGMA integrity_check;")

if [ "$RESULT" == "ok" ]; then
    echo "✓ Backup is valid"

    # Show some stats
    echo ""
    echo "Backup contents:"
    echo "- Users: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM users;")"
    echo "- Devices: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM devices;")"
    echo "- DIDs: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM dids;")"
    echo "- CDRs: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM cdrs;")"
else
    echo "✗ Backup is CORRUPT!"
    exit 1
fi

# Clean up
if [[ "$BACKUP_FILE" == *.gz ]]; then
    rm "$DB_FILE"
fi
```

---

## Restore Procedures

### Restore from API Backup

**Via Web UI:**
1. Login as admin
2. Go to **Admin** → **System** → **Backup**
3. Click **Restore**
4. Select or upload backup file
5. Confirm restore

**Via API:**
```bash
curl -X POST http://localhost:8080/api/system/restore \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{"filename": "backup_20251215_143022.db"}'
```

### Restore from File System Backup

**Docker Deployment:**
```bash
# Stop container
docker compose down

# Remove old data (CAUTION!)
docker volume rm gosip_gosip-data

# Create new volume
docker volume create gosip_gosip-data

# Restore from backup
docker run --rm \
  -v gosip_gosip-data:/data \
  -v $(pwd)/backups:/backup:ro \
  alpine sh -c "cd /data && tar xzf /backup/gosip-backup-20251215.tar.gz"

# Start container
docker compose up -d

# Verify
docker compose logs gosip
```

**Manual Installation:**
```bash
# Stop service
sudo systemctl stop gosip

# Backup current data (just in case)
mv /opt/gosip/data /opt/gosip/data.old

# Create new data directory
mkdir /opt/gosip/data

# Restore from backup
tar -xzf /backup/gosip-backup-20251215.tar.gz -C /opt/gosip/data

# Fix permissions
chown -R gosip:gosip /opt/gosip/data

# Start service
sudo systemctl start gosip

# Verify
sudo systemctl status gosip
```

### Restore Database Only

If you only need to restore the database:

```bash
# Stop GoSIP
sudo systemctl stop gosip

# Backup current database
cp /opt/gosip/data/gosip.db /opt/gosip/data/gosip.db.old

# Restore from backup
gunzip -c /backup/gosip_db_20251215.db.gz > /opt/gosip/data/gosip.db

# Fix permissions
chown gosip:gosip /opt/gosip/data/gosip.db

# Start GoSIP
sudo systemctl start gosip
```

### Post-Restore Verification

After restoring, verify:

1. **Login works**: Try admin login
2. **Devices register**: Check device status
3. **Calls work**: Make a test call
4. **Data present**: Check users, DIDs, routes

```bash
# Check system status
curl http://localhost:8080/api/system/status

# Should return healthy status
```

---

## Disaster Recovery

### Complete System Recovery

If your server is completely lost:

#### Step 1: Deploy New Server

```bash
# Install Docker
curl -fsSL https://get.docker.com | sh

# Create directory
mkdir -p /opt/gosip
cd /opt/gosip
```

#### Step 2: Restore Docker Compose

Create `docker-compose.yml` (same as original):

```yaml
services:
  gosip:
    image: btafoya/gosip:latest
    container_name: gosip
    restart: unless-stopped
    ports:
      - "8080:8080"
      - "5060:5060/udp"
      - "5060:5060/tcp"
    volumes:
      - gosip-data:/app/data
    environment:
      - GOSIP_DATA_DIR=/app/data
      - GOSIP_DB_PATH=/app/data/gosip.db
      - GOSIP_HTTP_PORT=8080
      - GOSIP_SIP_PORT=5060
      - GOSIP_EXTERNAL_IP=${GOSIP_EXTERNAL_IP:-}
      - TZ=${TZ:-America/New_York}
    networks:
      - gosip-net

volumes:
  gosip-data:

networks:
  gosip-net:
```

#### Step 3: Restore Data

```bash
# Create volume
docker volume create gosip_gosip-data

# Restore from backup
docker run --rm \
  -v gosip_gosip-data:/data \
  -v /path/to/backup:/backup:ro \
  alpine sh -c "cd /data && tar xzf /backup/gosip-full-backup.tar.gz"
```

#### Step 4: Start and Verify

```bash
# Start GoSIP
docker compose up -d

# Check logs
docker compose logs -f gosip

# Verify health
curl http://localhost:8080/api/health
```

#### Step 5: Update Network Configuration

1. Update DNS to point to new server
2. Update firewall rules
3. Update Twilio webhook URLs
4. Reconfigure router port forwarding

### Recovery Time Objectives

| Scenario | RTO Target | Notes |
|----------|------------|-------|
| **Database corruption** | 15 minutes | Restore from latest backup |
| **Server failure** | 1-2 hours | Deploy new server + restore |
| **Data center failure** | 4-8 hours | Depends on offsite backup access |

---

## Backup Best Practices

### Recommended Backup Schedule

| Backup Type | Frequency | Retention |
|-------------|-----------|-----------|
| Database (hot) | Daily | 30 days |
| Full backup | Weekly | 4 weeks |
| Monthly archive | Monthly | 12 months |
| Yearly archive | Yearly | 7 years |

### 3-2-1 Backup Rule

- **3** copies of your data
- **2** different storage media
- **1** offsite location

**Example implementation:**
1. Primary: Server storage
2. Secondary: NAS/external drive
3. Offsite: Cloud storage (S3, Backblaze, etc.)

### Security Considerations

1. **Encrypt backups** containing sensitive data:
   ```bash
   # Encrypt backup
   gpg -c --cipher-algo AES256 gosip-backup.tar.gz

   # Decrypt backup
   gpg -d gosip-backup.tar.gz.gpg > gosip-backup.tar.gz
   ```

2. **Restrict backup access** - only admins should access backups

3. **Secure transfer** - use encrypted channels (SSH, HTTPS)

4. **Test restores regularly** - backups are worthless if they don't restore

### Monitoring Backups

Create a simple monitoring script:

```bash
#!/bin/bash
# check-backup.sh

BACKUP_DIR="/backup/gosip"
MAX_AGE_HOURS=25

# Find most recent backup
LATEST=$(find "$BACKUP_DIR" -name "*.db.gz" -type f -printf '%T@ %p\n' | sort -n | tail -1 | cut -d' ' -f2)

if [ -z "$LATEST" ]; then
    echo "CRITICAL: No backups found!"
    exit 2
fi

# Check age
AGE_SECONDS=$(( $(date +%s) - $(stat -c %Y "$LATEST") ))
AGE_HOURS=$(( AGE_SECONDS / 3600 ))

if [ $AGE_HOURS -gt $MAX_AGE_HOURS ]; then
    echo "WARNING: Latest backup is $AGE_HOURS hours old"
    exit 1
else
    echo "OK: Latest backup is $AGE_HOURS hours old ($LATEST)"
    exit 0
fi
```

### Backup Checklist

- [ ] Daily database backups running
- [ ] Backups verified weekly
- [ ] Offsite copy maintained
- [ ] Restore tested quarterly
- [ ] Retention policy enforced
- [ ] Backup monitoring in place
- [ ] Recovery procedures documented
- [ ] Team trained on restore process

---

## Quick Reference

### Essential Commands

```bash
# Create database backup (API)
curl -X POST http://localhost:8080/api/system/backup \
  -H "Cookie: session=your-session-cookie"

# Create hot backup (SQLite)
sqlite3 /app/data/gosip.db ".backup /backup/backup.db"

# Full backup (Docker)
docker run --rm -v gosip_gosip-data:/data:ro -v $(pwd):/backup \
  alpine tar czf /backup/gosip-backup.tar.gz -C /data .

# Restore backup (API)
curl -X POST http://localhost:8080/api/system/restore \
  -H "Content-Type: application/json" \
  -H "Cookie: session=your-session-cookie" \
  -d '{"filename": "backup_20251215.db"}'

# Verify backup integrity
sqlite3 backup.db "PRAGMA integrity_check;"
```

### Backup File Locations

| Type | Location |
|------|----------|
| API backups | `data/backups/` |
| Database | `data/gosip.db` |
| Voicemails | `data/voicemails/` |
| Recordings | `data/recordings/` |

---

**Version**: 1.0
**Last Updated**: December 2025
