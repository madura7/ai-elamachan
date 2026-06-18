-- 0003_listing_images_upload — additive columns for the presign→confirm upload
-- flow (VER-299). The listing_images table was created in 0002 with only
-- (id, listing_id, object_key, sort_order, created_at). The direct-to-storage
-- upload flow needs a pending→active lifecycle plus validation/audit metadata.
--
-- ADDITIVE ONLY: no existing column is altered or dropped, so this is safe to
-- apply to a populated table. Existing rows default to status='active' so any
-- already-served images keep showing.
--
-- Columns:
--   status        presign→confirm lifecycle; only 'active' rows are served.
--   content_type  validated MIME (image/jpeg|png|webp); audit + Content-Type.
--   size_bytes    uploaded object size for audit / limit enforcement (<=8MB).
--   width,height  optional layout hints recorded at confirm time.
--   url           public-read URL derived from object_key at confirm time.

BEGIN;

ALTER TABLE listing_images
    ADD COLUMN IF NOT EXISTS status       TEXT   NOT NULL DEFAULT 'active'
        CHECK (status IN ('pending', 'active')),
    ADD COLUMN IF NOT EXISTS content_type TEXT,
    ADD COLUMN IF NOT EXISTS size_bytes   BIGINT,
    ADD COLUMN IF NOT EXISTS width        INTEGER,
    ADD COLUMN IF NOT EXISTS height       INTEGER,
    ADD COLUMN IF NOT EXISTS url          TEXT;

-- Serving query path: active images for a listing, ordered by sort_order.
CREATE INDEX IF NOT EXISTS idx_listing_images_active
    ON listing_images (listing_id, status, sort_order);

COMMIT;
