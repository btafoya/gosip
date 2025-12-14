-- Rollback provisioning extensions

-- Drop indexes first
DROP INDEX IF EXISTS idx_devices_mac;
DROP INDEX IF EXISTS idx_devices_provisioning_status;
DROP INDEX IF EXISTS idx_provisioning_tokens_token;
DROP INDEX IF EXISTS idx_provisioning_tokens_device;
DROP INDEX IF EXISTS idx_provisioning_tokens_expires;
DROP INDEX IF EXISTS idx_provisioning_profiles_vendor;
DROP INDEX IF EXISTS idx_provisioning_profiles_vendor_model;
DROP INDEX IF EXISTS idx_device_events_device;
DROP INDEX IF EXISTS idx_device_events_type;
DROP INDEX IF EXISTS idx_device_events_created;

-- Drop new tables
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS provisioning_profiles;
DROP TABLE IF EXISTS provisioning_tokens;

-- SQLite doesn't support DROP COLUMN, so we need to recreate the devices table
-- Create temporary table with original schema
CREATE TABLE devices_backup (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    device_type TEXT CHECK(device_type IN ('grandstream', 'softphone', 'webrtc')),
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Copy data to backup
INSERT INTO devices_backup (id, user_id, name, username, password_hash, device_type, recording_enabled, created_at)
SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at FROM devices;

-- Drop original table
DROP TABLE devices;

-- Rename backup to original
ALTER TABLE devices_backup RENAME TO devices;
