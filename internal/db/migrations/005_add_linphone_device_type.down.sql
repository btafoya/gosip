-- Revert linphone device_type (remove linphone from CHECK constraint)
-- Note: Any devices with device_type='linphone' will be converted to 'softphone'

-- Create table with original constraint
CREATE TABLE devices_old (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    device_type TEXT CHECK(device_type IN ('grandstream', 'softphone', 'webrtc')),
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    mac_address TEXT,
    vendor TEXT,
    model TEXT,
    firmware_version TEXT,
    provisioning_status TEXT DEFAULT 'unknown' CHECK(provisioning_status IN ('pending', 'provisioned', 'failed', 'unknown')),
    last_config_fetch DATETIME,
    last_registration DATETIME,
    config_template TEXT
);

-- Copy data, converting linphone to softphone
INSERT INTO devices_old (id, user_id, name, username, password_hash, device_type, recording_enabled, created_at,
    mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template)
SELECT id, user_id, name, username, password_hash,
    CASE WHEN device_type = 'linphone' THEN 'softphone' ELSE device_type END,
    recording_enabled, created_at,
    mac_address, vendor, model, firmware_version, provisioning_status, last_config_fetch, last_registration, config_template
FROM devices;

-- Drop new table
DROP TABLE devices;

-- Rename old table
ALTER TABLE devices_old RENAME TO devices;

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_devices_username ON devices(username);
CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id);
