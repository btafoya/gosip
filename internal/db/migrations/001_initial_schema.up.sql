-- System configuration (key-value)
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Admin and user accounts
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT CHECK(role IN ('admin', 'user')) NOT NULL DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_login DATETIME
);

-- Registered SIP devices
CREATE TABLE devices (
    id INTEGER PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name TEXT NOT NULL,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    device_type TEXT CHECK(device_type IN ('grandstream', 'softphone', 'webrtc')),
    recording_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Active SIP registrations
CREATE TABLE registrations (
    id INTEGER PRIMARY KEY,
    device_id INTEGER REFERENCES devices(id) ON DELETE CASCADE,
    contact TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    transport TEXT CHECK(transport IN ('udp', 'tcp', 'tls', 'ws', 'wss')),
    last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Phone numbers (DIDs)
CREATE TABLE dids (
    id INTEGER PRIMARY KEY,
    number TEXT UNIQUE NOT NULL,
    twilio_sid TEXT,
    name TEXT,
    sms_enabled BOOLEAN DEFAULT FALSE,
    voice_enabled BOOLEAN DEFAULT TRUE
);

-- Call routing rules
CREATE TABLE routes (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id) ON DELETE CASCADE,
    priority INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL,
    condition_type TEXT CHECK(condition_type IN ('time', 'callerid', 'default')),
    condition_data JSON,
    action_type TEXT CHECK(action_type IN ('ring', 'forward', 'voicemail', 'reject')),
    action_data JSON,
    enabled BOOLEAN DEFAULT TRUE
);

-- Blocked numbers
CREATE TABLE blocklist (
    id INTEGER PRIMARY KEY,
    pattern TEXT NOT NULL,
    pattern_type TEXT CHECK(pattern_type IN ('exact', 'prefix', 'regex')),
    reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Call detail records
CREATE TABLE cdrs (
    id INTEGER PRIMARY KEY,
    call_sid TEXT UNIQUE,
    direction TEXT CHECK(direction IN ('inbound', 'outbound')),
    from_number TEXT NOT NULL,
    to_number TEXT NOT NULL,
    did_id INTEGER REFERENCES dids(id),
    device_id INTEGER REFERENCES devices(id),
    started_at DATETIME NOT NULL,
    answered_at DATETIME,
    ended_at DATETIME,
    duration INTEGER DEFAULT 0,
    disposition TEXT CHECK(disposition IN ('answered', 'voicemail', 'missed', 'blocked', 'busy', 'failed')),
    recording_url TEXT,
    spam_score REAL
);

-- Voicemails
CREATE TABLE voicemails (
    id INTEGER PRIMARY KEY,
    cdr_id INTEGER REFERENCES cdrs(id),
    user_id INTEGER REFERENCES users(id),
    from_number TEXT NOT NULL,
    audio_url TEXT,
    transcript TEXT,
    duration INTEGER DEFAULT 0,
    is_read BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- SMS/MMS messages
CREATE TABLE messages (
    id INTEGER PRIMARY KEY,
    message_sid TEXT UNIQUE,
    direction TEXT CHECK(direction IN ('inbound', 'outbound')),
    from_number TEXT NOT NULL,
    to_number TEXT NOT NULL,
    did_id INTEGER REFERENCES dids(id),
    body TEXT,
    media_urls JSON,
    status TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    is_read BOOLEAN DEFAULT FALSE
);

-- Auto-reply rules
CREATE TABLE auto_replies (
    id INTEGER PRIMARY KEY,
    did_id INTEGER REFERENCES dids(id) ON DELETE CASCADE,
    trigger_type TEXT CHECK(trigger_type IN ('dnd', 'after_hours', 'keyword')),
    trigger_data JSON,
    reply_text TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE
)
