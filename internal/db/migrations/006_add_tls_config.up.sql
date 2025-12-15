-- Migration 006: Add TLS/Encryption configuration settings
-- This migration adds config entries for TLS/SIPS support and SRTP encryption

-- TLS configuration entries
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.enabled', 'false', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.port', '5061', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.wss_port', '5081', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.cert_mode', 'acme', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.cert_file', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.key_file', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.ca_file', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.min_version', '1.2', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.client_auth', 'none', datetime('now'));

-- ACME/Let's Encrypt configuration entries
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('acme.email', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('acme.domain', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('acme.domains', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('acme.ca', 'staging', datetime('now'));

-- Cloudflare DNS challenge configuration
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('cloudflare.api_token', '', datetime('now'));

-- SRTP configuration entries
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('srtp.enabled', 'false', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('srtp.profile', 'AES_CM_128_HMAC_SHA1_80', datetime('now'));

-- Certificate status tracking
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.cert_expiry', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.cert_issuer', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.last_renewal', '', datetime('now'));
INSERT OR IGNORE INTO config (key, value, updated_at) VALUES ('tls.next_renewal', '', datetime('now'));
