-- Migration 007: Add disable unencrypted SIP configuration
-- This allows completely disabling UDP/TCP on port 5060

-- Add disable unencrypted setting (when true, only TLS connections accepted)
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.disable_unencrypted', 'false', datetime('now'));

-- ZRTP configuration entries for end-to-end encryption
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('zrtp.enabled', 'false', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('zrtp.cache_expiry_days', '90', datetime('now'));
