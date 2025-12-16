#!/bin/bash
#
# GoSIP Docker Backup Script
#
# This script creates backups of the GoSIP database and media files
# when running in a Docker container.
#
# Usage:
#   ./backup-docker.sh                    # Use default container name
#   ./backup-docker.sh gosip              # Specify container name
#   ./backup-docker.sh --full             # Full backup including media
#
# Cron example (daily at 2 AM):
#   0 2 * * * /opt/gosip/scripts/backup-docker.sh >> /var/log/gosip-backup.log 2>&1
#

set -euo pipefail

# Default configuration
CONTAINER_NAME="${1:-gosip}"
BACKUP_DIR="${GOSIP_BACKUP_DIR:-/backup/gosip}"
RETENTION_DAYS="${GOSIP_BACKUP_RETENTION:-30}"
FULL_BACKUP=false
DATE=$(date +%Y%m%d_%H%M%S)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Log functions
log() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $1"
}

log_success() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] ${GREEN}SUCCESS:${NC} $1"
}

log_warn() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] ${YELLOW}WARNING:${NC} $1"
}

log_error() {
    echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] ${RED}ERROR:${NC} $1"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --container)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        --backup-dir)
            BACKUP_DIR="$2"
            shift 2
            ;;
        --retention)
            RETENTION_DAYS="$2"
            shift 2
            ;;
        --full)
            FULL_BACKUP=true
            shift
            ;;
        --help)
            echo "GoSIP Docker Backup Script"
            echo ""
            echo "Usage: $0 [options] [container-name]"
            echo ""
            echo "Options:"
            echo "  --container NAME    Container name (default: gosip)"
            echo "  --backup-dir DIR    Backup directory (default: /backup/gosip)"
            echo "  --retention DAYS    Backup retention in days (default: 30)"
            echo "  --full              Full backup including media files"
            echo "  --help              Show this help message"
            exit 0
            ;;
        -*)
            log_error "Unknown option: $1"
            exit 1
            ;;
        *)
            CONTAINER_NAME="$1"
            shift
            ;;
    esac
done

# Create backup directory
mkdir -p "$BACKUP_DIR"

log "=========================================="
log "GoSIP Docker Backup Starting"
log "=========================================="
log "Container: $CONTAINER_NAME"
log "Backup directory: $BACKUP_DIR"
log "Retention days: $RETENTION_DAYS"
log "Full backup: $FULL_BACKUP"
log ""

# Check if container is running
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    log_error "Container '$CONTAINER_NAME' is not running!"
    log "Available containers:"
    docker ps --format '  {{.Names}}'
    exit 1
fi

# Track success
BACKUP_SUCCESS=true

# 1. Database backup (hot backup without stopping container)
log "Step 1: Creating database backup..."
DB_BACKUP_FILE="gosip_db_${DATE}.db"
INTERNAL_BACKUP="/app/data/backups/backup_${DATE}.db"

# Create backup inside container using SQLite VACUUM INTO
if docker exec "$CONTAINER_NAME" sqlite3 /app/data/gosip.db "VACUUM INTO '$INTERNAL_BACKUP'"; then
    # Copy backup from container
    if docker cp "$CONTAINER_NAME:$INTERNAL_BACKUP" "$BACKUP_DIR/$DB_BACKUP_FILE"; then
        # Remove backup from container
        docker exec "$CONTAINER_NAME" rm -f "$INTERNAL_BACKUP"

        DB_SIZE=$(stat -f%z "$BACKUP_DIR/$DB_BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_DIR/$DB_BACKUP_FILE" 2>/dev/null || echo "unknown")
        log_success "Database backup created: $DB_BACKUP_FILE (${DB_SIZE} bytes)"

        # Verify backup integrity
        INTEGRITY=$(sqlite3 "$BACKUP_DIR/$DB_BACKUP_FILE" "PRAGMA integrity_check;" 2>&1)
        if [ "$INTEGRITY" = "ok" ]; then
            log_success "Backup integrity verified: OK"
        else
            log_error "Backup integrity check failed: $INTEGRITY"
            BACKUP_SUCCESS=false
        fi
    else
        log_error "Failed to copy backup from container"
        BACKUP_SUCCESS=false
    fi
else
    log_error "Failed to create database backup!"
    BACKUP_SUCCESS=false
fi

# 2. Full volume backup (optional)
if [ "$FULL_BACKUP" = true ]; then
    log "Step 2: Creating full volume backup..."
    FULL_BACKUP_FILE="gosip_full_${DATE}.tar.gz"

    # Get the volume name
    VOLUME_NAME=$(docker inspect "$CONTAINER_NAME" --format '{{range .Mounts}}{{if eq .Destination "/app/data"}}{{.Name}}{{end}}{{end}}')

    if [ -n "$VOLUME_NAME" ]; then
        # Create backup using a temporary container
        if docker run --rm \
            -v "${VOLUME_NAME}:/data:ro" \
            -v "$BACKUP_DIR:/backup" \
            alpine tar czf "/backup/$FULL_BACKUP_FILE" -C /data . 2>/dev/null; then

            FULL_SIZE=$(stat -f%z "$BACKUP_DIR/$FULL_BACKUP_FILE" 2>/dev/null || stat -c%s "$BACKUP_DIR/$FULL_BACKUP_FILE" 2>/dev/null || echo "unknown")
            log_success "Full backup created: $FULL_BACKUP_FILE (${FULL_SIZE} bytes)"
        else
            log_warn "Failed to create full volume backup"
        fi
    else
        log_warn "Could not determine volume name for full backup"
    fi
else
    log "Step 2: Skipping full volume backup (use --full to enable)"
fi

# 3. Clean old backups
log "Step 3: Cleaning old backups (older than $RETENTION_DAYS days)..."
DELETED_COUNT=0

# Clean old database backups
for file in "$BACKUP_DIR"/gosip_db_*.db; do
    if [ -f "$file" ]; then
        FILE_AGE=$(( ( $(date +%s) - $(stat -f%m "$file" 2>/dev/null || stat -c%Y "$file" 2>/dev/null) ) / 86400 ))
        if [ "$FILE_AGE" -gt "$RETENTION_DAYS" ]; then
            rm -f "$file"
            ((DELETED_COUNT++))
            log "Deleted old backup: $(basename "$file") (${FILE_AGE} days old)"
        fi
    fi
done

# Clean old full backups
for file in "$BACKUP_DIR"/gosip_full_*.tar.gz; do
    if [ -f "$file" ]; then
        FILE_AGE=$(( ( $(date +%s) - $(stat -f%m "$file" 2>/dev/null || stat -c%Y "$file" 2>/dev/null) ) / 86400 ))
        if [ "$FILE_AGE" -gt "$RETENTION_DAYS" ]; then
            rm -f "$file"
            ((DELETED_COUNT++))
            log "Deleted old backup: $(basename "$file") (${FILE_AGE} days old)"
        fi
    fi
done

if [ "$DELETED_COUNT" -gt 0 ]; then
    log_success "Cleaned $DELETED_COUNT old backup file(s)"
else
    log "No old backups to clean"
fi

# 4. Summary
log ""
log "=========================================="
log "Backup Summary"
log "=========================================="

# Calculate total backup size
TOTAL_SIZE=0
for file in "$BACKUP_DIR"/*_${DATE}*; do
    if [ -f "$file" ]; then
        SIZE=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo "0")
        TOTAL_SIZE=$((TOTAL_SIZE + SIZE))
    fi
done

# Human-readable size
if command -v bc &> /dev/null; then
    if [ "$TOTAL_SIZE" -gt 1073741824 ]; then
        HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1073741824" | bc) GB"
    elif [ "$TOTAL_SIZE" -gt 1048576 ]; then
        HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1048576" | bc) MB"
    elif [ "$TOTAL_SIZE" -gt 1024 ]; then
        HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1024" | bc) KB"
    else
        HUMAN_SIZE="$TOTAL_SIZE bytes"
    fi
else
    HUMAN_SIZE="$TOTAL_SIZE bytes"
fi

log "Total backup size: $HUMAN_SIZE"
log "Backup directory: $BACKUP_DIR"

# List recent backups
log ""
log "Recent backups:"
ls -lh "$BACKUP_DIR"/gosip_*.db 2>/dev/null | tail -5 || log "No database backups found"

if [ "$BACKUP_SUCCESS" = true ]; then
    log ""
    log_success "Backup completed successfully!"
    exit 0
else
    log ""
    log_error "Backup completed with errors!"
    exit 1
fi
