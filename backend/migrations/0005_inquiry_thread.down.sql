-- Rollback 0005_inquiry_thread
BEGIN;
DROP TABLE IF EXISTS inquiry_reports;
DROP TABLE IF EXISTS inquiry_messages;
COMMIT;
