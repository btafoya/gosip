-- CDR indexes for call history queries
CREATE INDEX idx_cdrs_started ON cdrs(started_at DESC);
CREATE INDEX idx_cdrs_disposition ON cdrs(disposition);
CREATE INDEX idx_cdrs_did ON cdrs(did_id);

-- Message indexes for SMS history
CREATE INDEX idx_messages_created ON messages(created_at DESC);
CREATE INDEX idx_messages_did ON messages(did_id);

-- Voicemail indexes
CREATE INDEX idx_voicemails_user ON voicemails(user_id);
CREATE INDEX idx_voicemails_read ON voicemails(is_read);

-- Registration indexes for SIP operations
CREATE INDEX idx_registrations_device ON registrations(device_id);
CREATE INDEX idx_registrations_expires ON registrations(expires_at);

-- Route lookup index
CREATE INDEX idx_routes_did_priority ON routes(did_id, priority)
