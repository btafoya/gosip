DROP INDEX IF EXISTS idx_dids_trunk_id;
ALTER TABLE dids DROP COLUMN trunk_id;
DROP TABLE IF EXISTS trunks;
DROP INDEX IF EXISTS idx_trunks_twilio_sid;
