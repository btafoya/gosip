CREATE TABLE trunks (
    id INTEGER PRIMARY KEY,
    twilio_sid TEXT UNIQUE NOT NULL,
    friendly_name TEXT,
    domain_name TEXT,
    secure BOOLEAN DEFAULT FALSE,
    transfer_mode TEXT DEFAULT 'disable-all',
    cnam_lookup_enabled BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_trunks_twilio_sid ON trunks(twilio_sid);

ALTER TABLE dids ADD COLUMN trunk_id INTEGER REFERENCES trunks(id) ON DELETE SET NULL;
CREATE INDEX idx_dids_trunk_id ON dids(trunk_id);
