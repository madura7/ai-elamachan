-- 0006_listing_images_upload — adds presign→confirm lifecycle columns to listing_images.
-- Board approval required before merge (schema policy, VER-127).
--
-- listing_images was created in 0002 with minimal columns:
--   id, listing_id, object_key, sort_order, created_at
-- This migration adds the upload-flow columns:
--   status        — pending (presigned, awaiting confirm) | active (confirmed)
--   content_type  — validated MIME type (image/jpeg, image/png, image/webp)
--   size_bytes    — validated at presign time
--   width, height — optional layout hints (nullable; populated post-confirm if available)
--   url           — public CDN URL (derived from object_key at confirm time)
--
-- All existing rows get status='active' via DEFAULT.

BEGIN;

ALTER TABLE listing_images
  ADD COLUMN IF NOT EXISTS status       TEXT    NOT NULL DEFAULT 'active'
    CHECK (status IN ('pending', 'active')),
  ADD COLUMN IF NOT EXISTS content_type TEXT,
  ADD COLUMN IF NOT EXISTS size_bytes   BIGINT,
  ADD COLUMN IF NOT EXISTS width        INTEGER,
  ADD COLUMN IF NOT EXISTS height       INTEGER,
  ADD COLUMN IF NOT EXISTS url          TEXT;

-- Hot path: fetch active images ordered by sort_order for a given listing.
CREATE INDEX IF NOT EXISTS idx_listing_images_listing_status
  ON listing_images (listing_id, status, sort_order);

COMMIT;
