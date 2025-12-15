-- Add linphone to device_type CHECK constraint
-- SQLite doesn't support ALTER TABLE to modify constraints, so we need to recreate the table

-- Create new table with updated constraint
CREATE TABLE devices_new (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    device_type TEXT CHECK(device_type IN ('grandstream', 'softphone', 'webrtc', 'linphone')),
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    -- Provisioning fields (from migration 003)
    mac_address TEXT,
    vendor TEXT,
    model TEXT,
    firmware_version TEXT,
    provisioning_status TEXT DEFAULT 'unknown' CHECK(provisioning_status IN ('pending', 'provisioned', 'failed', 'unknown')),
    last_config_fetch DATETIME,
    last_registration DATETIME,
    config_template TEXT
);

-- Copy existing data
INSERT INTO devices_new (id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
    mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template)
SELECT id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
    mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
FROM devices;

-- Drop old table
DROP TABLE devices;

-- Rename new table
ALTER TABLE devices_new RENAME TO devices;

-- Recreate indexes (from migration 002)
CREATE INDEX IF NOT EXISTS idx_devices_username ON devices(username);
CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id);
