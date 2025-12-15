-- Migration 006 rollback: Remove TLS/Encryption configuration settings

-- Remove TLS configuration entries
DELETE FROM config WHERE key LIKE 'tls.%';

-- Remove ACME configuration entries
DELETE FROM config WHERE key LIKE 'acme.%';

-- Remove Cloudflare configuration entries
DELETE FROM config WHERE key LIKE 'cloudflare.%';

-- Remove SRTP configuration entries
DELETE FROM config WHERE key LIKE 'srtp.%';
