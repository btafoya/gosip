-- Device provisioning extensions for Theme A

-- Extend devices table with provisioning fields
ALTER TABLE devices ADD COLUMN mac_address TEXT;
ALTER TABLE devices ADD COLUMN vendor TEXT;
ALTER TABLE devices ADD COLUMN model TEXT;
ALTER TABLE devices ADD COLUMN firmware_version TEXT;
ALTER TABLE devices ADD COLUMN provisioning_status TEXT CHECK(provisioning_status IN ('pending', 'provisioned', 'failed', 'unknown')) DEFAULT 'unknown';
ALTER TABLE devices ADD COLUMN last_config_fetch DATETIME;
ALTER TABLE devices ADD COLUMN last_registration DATETIME;
ALTER TABLE devices ADD COLUMN config_template TEXT;

-- Provisioning tokens for tokened URLs (short-lived, revocable)
CREATE TABLE provisioning_tokens (
    id INTEGER PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    device_id INTEGER REFERENCES devices(id) ON DELETE CASCADE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    revoked BOOLEAN DEFAULT FALSE,
    revoked_at DATETIME,
    used_count INTEGER DEFAULT 0,
    max_uses INTEGER DEFAULT 1,
    ip_restriction TEXT,
    created_by INTEGER REFERENCES users(id)
);

-- Device provisioning profiles (vendor/model templates)
CREATE TABLE provisioning_profiles (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    vendor TEXT NOT NULL,
    model TEXT,
    description TEXT,
    config_template TEXT NOT NULL,
    variables JSON,
    is_default BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Device events for operational visibility
CREATE TABLE device_events (
    id INTEGER PRIMARY KEY,
    device_id INTEGER REFERENCES devices(id) ON DELETE CASCADE,
    event_type TEXT CHECK(event_type IN (
        'config_fetch', 'config_fetch_failed',
        'registration', 'registration_failed', 'unregistration',
        'provision_start', 'provision_complete', 'provision_failed',
        'call_start', 'call_end',
        'firmware_check', 'firmware_update',
        'error', 'warning', 'info'
    )) NOT NULL,
    event_data JSON,
    ip_address TEXT,
    user_agent TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient queries
CREATE INDEX idx_devices_mac ON devices(mac_address);
CREATE INDEX idx_devices_provisioning_status ON devices(provisioning_status);
CREATE INDEX idx_provisioning_tokens_token ON provisioning_tokens(token);
CREATE INDEX idx_provisioning_tokens_device ON provisioning_tokens(device_id);
CREATE INDEX idx_provisioning_tokens_expires ON provisioning_tokens(expires_at);
CREATE INDEX idx_provisioning_profiles_vendor ON provisioning_profiles(vendor);
CREATE INDEX idx_provisioning_profiles_vendor_model ON provisioning_profiles(vendor, model);
CREATE INDEX idx_device_events_device ON device_events(device_id);
CREATE INDEX idx_device_events_type ON device_events(event_type);
CREATE INDEX idx_device_events_created ON device_events(created_at);

-- Insert default Grandstream GXP1760W profile
INSERT INTO provisioning_profiles (name, vendor, model, description, config_template, variables, is_default) VALUES (
    'Grandstream GXP1760W Default',
    'grandstream',
    'GXP1760W',
    'Default configuration template for Grandstream GXP1760W phones',
    '<?xml version="1.0" encoding="UTF-8"?>
<gs_provision version="1">
    <!-- Account 1 Settings -->
    <config name="P271" value="{{.SIPServer}}"/>
    <config name="P47" value="{{.SIPPort}}"/>
    <config name="P35" value="{{.AuthID}}"/>
    <config name="P36" value="{{.AuthPassword}}"/>
    <config name="P3" value="{{.DisplayName}}"/>
    <config name="P34" value="{{.Username}}"/>

    <!-- Registration Settings -->
    <config name="P81" value="1"/>
    <config name="P32" value="300"/>

    <!-- Codec Settings (G.711u preferred) -->
    <config name="P57" value="0"/>
    <config name="P58" value="8"/>

    <!-- NAT Settings -->
    <config name="P52" value="2"/>
    <config name="P48" value="{{.STUNServer}}"/>

    <!-- Time Settings -->
    <config name="P64" value="{{.NTPServer}}"/>
    <config name="P75" value="{{.Timezone}}"/>

    <!-- Security -->
    <config name="P2" value="{{.AdminPassword}}"/>
</gs_provision>',
    '{"SIPServer": "", "SIPPort": "5060", "AuthID": "", "AuthPassword": "", "DisplayName": "", "Username": "", "STUNServer": "stun.l.google.com", "NTPServer": "pool.ntp.org", "Timezone": "America/New_York", "AdminPassword": ""}',
    1
);
