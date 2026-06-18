-- Roll back 0003_listing_images_upload.
-- Drops the serving index and the additive columns, returning listing_images
-- to its 0002 shape (id, listing_id, object_key, sort_order, created_at).

BEGIN;

DROP INDEX IF EXISTS idx_listing_images_active;

ALTER TABLE listing_images
    DROP COLUMN IF EXISTS url,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS size_bytes,
    DROP COLUMN IF EXISTS content_type,
    DROP COLUMN IF EXISTS status;

COMMIT;
