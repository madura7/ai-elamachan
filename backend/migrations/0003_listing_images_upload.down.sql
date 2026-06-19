-- Rollback for 0003_listing_images_upload.

BEGIN;

DROP INDEX IF EXISTS idx_listing_images_listing_status;

ALTER TABLE listing_images
  DROP COLUMN IF EXISTS status,
  DROP COLUMN IF EXISTS content_type,
  DROP COLUMN IF EXISTS size_bytes,
  DROP COLUMN IF EXISTS width,
  DROP COLUMN IF EXISTS height,
  DROP COLUMN IF EXISTS url;

COMMIT;
