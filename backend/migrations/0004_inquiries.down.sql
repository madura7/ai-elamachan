-- Rollback for 0003_inquiries

BEGIN;

DROP TABLE IF EXISTS connection_events;
DROP TABLE IF EXISTS inquiries;

COMMIT;
