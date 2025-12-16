#!/bin/bash
#
# GoSIP Backup Script
#
# This script creates backups of the GoSIP database and media files.
# Designed to run via cron for automated daily backups.
#
# Usage:
#   ./backup.sh                    # Use default settings
#   ./backup.sh --data-dir /path   # Custom data directory
#   ./backup.sh --retention 30     # Custom retention (days)
#   ./backup.sh --no-media         # Skip media backup
#   ./backup.sh --verify           # Verify backup after creation
#
# Cron example (daily at 2 AM):
#   0 2 * * * /opt/gosip/scripts/backup.sh >> /var/log/gosip-backup.log 2>&1
#

set -euo pipefail

# Default configuration
DATA_DIR="${GOSIP_DATA_DIR:-/opt/gosip/data}"
BACKUP_DIR="${GOSIP_BACKUP_DIR:-}"
RETENTION_DAYS="${GOSIP_BACKUP_RETENTION:-30}"
BACKUP_MEDIA=true
VERIFY_BACKUP=false
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
        --data-dir)
            DATA_DIR="$2"
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
        --no-media)
            BACKUP_MEDIA=false
            shift
            ;;
        --verify)
            VERIFY_BACKUP=true
            shift
            ;;
        --help)
            echo "GoSIP Backup Script"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --data-dir DIR      Data directory (default: /opt/gosip/data)"
            echo "  --backup-dir DIR    Backup directory (default: DATA_DIR/backups)"
            echo "  --retention DAYS    Backup retention in days (default: 30)"
            echo "  --no-media          Skip media files backup"
            echo "  --verify            Verify backup integrity after creation"
            echo "  --help              Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Set backup directory if not specified
if [ -z "$BACKUP_DIR" ]; then
    BACKUP_DIR="$DATA_DIR/backups"
fi

# Paths
DB_PATH="$DATA_DIR/gosip.db"
VOICEMAILS_DIR="$DATA_DIR/voicemails"
RECORDINGS_DIR="$DATA_DIR/recordings"

# Validate data directory
if [ ! -d "$DATA_DIR" ]; then
    log_error "Data directory not found: $DATA_DIR"
    exit 1
fi

# Validate database exists
if [ ! -f "$DB_PATH" ]; then
    log_error "Database not found: $DB_PATH"
    exit 1
fi

# Create backup directory if needed
mkdir -p "$BACKUP_DIR"

log "=========================================="
log "GoSIP Backup Starting"
log "=========================================="
log "Data directory: $DATA_DIR"
log "Backup directory: $BACKUP_DIR"
log "Retention days: $RETENTION_DAYS"
log "Backup media: $BACKUP_MEDIA"
log "Verify backup: $VERIFY_BACKUP"
log ""

# Track success
BACKUP_SUCCESS=true
DB_BACKUP_FILE=""

# 1. Database backup using SQLite hot backup
log "Step 1: Creating database backup..."
DB_BACKUP_FILE="backup_${DATE}.db"
DB_BACKUP_PATH="$BACKUP_DIR/$DB_BACKUP_FILE"

if sqlite3 "$DB_PATH" "VACUUM INTO '$DB_BACKUP_PATH'"; then
    DB_SIZE=$(stat -f%z "$DB_BACKUP_PATH" 2>/dev/null || stat -c%s "$DB_BACKUP_PATH" 2>/dev/null || echo "unknown")
    log_success "Database backup created: $DB_BACKUP_FILE (${DB_SIZE} bytes)"
else
    log_error "Failed to create database backup!"
    BACKUP_SUCCESS=false
fi

# 2. Verify backup integrity if requested
if [ "$VERIFY_BACKUP" = true ] && [ -f "$DB_BACKUP_PATH" ]; then
    log "Step 2: Verifying backup integrity..."
    INTEGRITY=$(sqlite3 "$DB_BACKUP_PATH" "PRAGMA integrity_check;" 2>&1)
    if [ "$INTEGRITY" = "ok" ]; then
        log_success "Backup integrity verified: OK"
    else
        log_error "Backup integrity check failed: $INTEGRITY"
        BACKUP_SUCCESS=false
    fi
else
    log "Step 2: Skipping backup verification"
fi

# 3. Voicemail backup
if [ "$BACKUP_MEDIA" = true ]; then
    log "Step 3: Creating voicemail backup..."
    VOICEMAIL_BACKUP="voicemails_${DATE}.tar.gz"
    VOICEMAIL_BACKUP_PATH="$BACKUP_DIR/$VOICEMAIL_BACKUP"

    if [ -d "$VOICEMAILS_DIR" ] && [ "$(ls -A "$VOICEMAILS_DIR" 2>/dev/null)" ]; then
        if tar -czf "$VOICEMAIL_BACKUP_PATH" -C "$DATA_DIR" voicemails/ 2>/dev/null; then
            VM_SIZE=$(stat -f%z "$VOICEMAIL_BACKUP_PATH" 2>/dev/null || stat -c%s "$VOICEMAIL_BACKUP_PATH" 2>/dev/null || echo "unknown")
            log_success "Voicemail backup created: $VOICEMAIL_BACKUP (${VM_SIZE} bytes)"
        else
            log_warn "Failed to create voicemail backup"
        fi
    else
        log "No voicemails to backup"
    fi
else
    log "Step 3: Skipping voicemail backup"
fi

# 4. Recordings backup (optional - can be large)
if [ "$BACKUP_MEDIA" = true ]; then
    log "Step 4: Creating recordings backup..."
    RECORDING_BACKUP="recordings_${DATE}.tar.gz"
    RECORDING_BACKUP_PATH="$BACKUP_DIR/$RECORDING_BACKUP"

    if [ -d "$RECORDINGS_DIR" ] && [ "$(ls -A "$RECORDINGS_DIR" 2>/dev/null)" ]; then
        if tar -czf "$RECORDING_BACKUP_PATH" -C "$DATA_DIR" recordings/ 2>/dev/null; then
            REC_SIZE=$(stat -f%z "$RECORDING_BACKUP_PATH" 2>/dev/null || stat -c%s "$RECORDING_BACKUP_PATH" 2>/dev/null || echo "unknown")
            log_success "Recordings backup created: $RECORDING_BACKUP (${REC_SIZE} bytes)"
        else
            log_warn "Failed to create recordings backup"
        fi
    else
        log "No recordings to backup"
    fi
else
    log "Step 4: Skipping recordings backup"
fi

# 5. Clean old backups
log "Step 5: Cleaning old backups (older than $RETENTION_DAYS days)..."

# Count deleted files
DELETED_COUNT=0

# Clean old database backups
for file in "$BACKUP_DIR"/backup_*.db; do
    if [ -f "$file" ]; then
        FILE_AGE=$(( ( $(date +%s) - $(stat -f%m "$file" 2>/dev/null || stat -c%Y "$file" 2>/dev/null) ) / 86400 ))
        if [ "$FILE_AGE" -gt "$RETENTION_DAYS" ]; then
            rm -f "$file"
            ((DELETED_COUNT++))
            log "Deleted old backup: $(basename "$file") (${FILE_AGE} days old)"
        fi
    fi
done

# Clean old voicemail archives
for file in "$BACKUP_DIR"/voicemails_*.tar.gz; do
    if [ -f "$file" ]; then
        FILE_AGE=$(( ( $(date +%s) - $(stat -f%m "$file" 2>/dev/null || stat -c%Y "$file" 2>/dev/null) ) / 86400 ))
        if [ "$FILE_AGE" -gt "$RETENTION_DAYS" ]; then
            rm -f "$file"
            ((DELETED_COUNT++))
            log "Deleted old voicemail backup: $(basename "$file") (${FILE_AGE} days old)"
        fi
    fi
done

# Clean old recording archives
for file in "$BACKUP_DIR"/recordings_*.tar.gz; do
    if [ -f "$file" ]; then
        FILE_AGE=$(( ( $(date +%s) - $(stat -f%m "$file" 2>/dev/null || stat -c%Y "$file" 2>/dev/null) ) / 86400 ))
        if [ "$FILE_AGE" -gt "$RETENTION_DAYS" ]; then
            rm -f "$file"
            ((DELETED_COUNT++))
            log "Deleted old recordings backup: $(basename "$file") (${FILE_AGE} days old)"
        fi
    fi
done

if [ "$DELETED_COUNT" -gt 0 ]; then
    log_success "Cleaned $DELETED_COUNT old backup file(s)"
else
    log "No old backups to clean"
fi

# 6. Summary
log ""
log "=========================================="
log "Backup Summary"
log "=========================================="

# Calculate total backup size
TOTAL_SIZE=0
if [ -f "$DB_BACKUP_PATH" ]; then
    SIZE=$(stat -f%z "$DB_BACKUP_PATH" 2>/dev/null || stat -c%s "$DB_BACKUP_PATH" 2>/dev/null || echo "0")
    TOTAL_SIZE=$((TOTAL_SIZE + SIZE))
fi
if [ -f "$VOICEMAIL_BACKUP_PATH" ]; then
    SIZE=$(stat -f%z "$VOICEMAIL_BACKUP_PATH" 2>/dev/null || stat -c%s "$VOICEMAIL_BACKUP_PATH" 2>/dev/null || echo "0")
    TOTAL_SIZE=$((TOTAL_SIZE + SIZE))
fi
if [ -f "$RECORDING_BACKUP_PATH" ]; then
    SIZE=$(stat -f%z "$RECORDING_BACKUP_PATH" 2>/dev/null || stat -c%s "$RECORDING_BACKUP_PATH" 2>/dev/null || echo "0")
    TOTAL_SIZE=$((TOTAL_SIZE + SIZE))
fi

# Human-readable size
if [ "$TOTAL_SIZE" -gt 1073741824 ]; then
    HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1073741824" | bc) GB"
elif [ "$TOTAL_SIZE" -gt 1048576 ]; then
    HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1048576" | bc) MB"
elif [ "$TOTAL_SIZE" -gt 1024 ]; then
    HUMAN_SIZE="$(echo "scale=2; $TOTAL_SIZE / 1024" | bc) KB"
else
    HUMAN_SIZE="$TOTAL_SIZE bytes"
fi

log "Total backup size: $HUMAN_SIZE"
log "Backup directory: $BACKUP_DIR"

# List recent backups
log ""
log "Recent backups:"
ls -lh "$BACKUP_DIR"/backup_*.db 2>/dev/null | tail -5 || log "No database backups found"

if [ "$BACKUP_SUCCESS" = true ]; then
    log ""
    log_success "Backup completed successfully!"
    exit 0
else
    log ""
    log_error "Backup completed with errors!"
    exit 1
fi
