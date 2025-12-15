-- Migration 007 rollback: Remove disable unencrypted configuration

DELETE FROM config WHERE key = 'tls.disable_unencrypted';
DELETE FROM config WHERE key LIKE 'zrtp.%';
