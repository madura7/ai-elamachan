-- 0003_inquiries — inquiry persistence + connection events (VER-297, VER-295 M1)
--
-- Adds buyer→seller contact tracking tables:
--   inquiries: a buyer's message about a specific listing
--   connection_events: deduplicated first-contact records per (buyer, listing)
--     with UNIQUE constraint to guarantee at most one event per pair.

BEGIN;

CREATE TABLE IF NOT EXISTS inquiries (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id       UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    buyer_user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Denormalized from listing.user_id at insert time so inbox queries avoid
    -- a join to listings on every row.
    seller_user_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    message          TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new','read','replied')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at          TIMESTAMPTZ
);

-- Seller inbox: all inquiries for a seller, newest first.
CREATE INDEX IF NOT EXISTS idx_inquiries_seller_created
    ON inquiries (seller_user_id, created_at DESC);

-- Per-listing inbox and dedup checks.
CREATE INDEX IF NOT EXISTS idx_inquiries_listing_created
    ON inquiries (listing_id, created_at DESC);

CREATE TABLE IF NOT EXISTS connection_events (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    seller_user_id   UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    listing_id       UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    -- The first inquiry that triggered this connection event.
    first_inquiry_id UUID NOT NULL REFERENCES inquiries (id) ON DELETE CASCADE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- North-Star constraint: exactly one connection event per (buyer, listing).
    UNIQUE (buyer_user_id, listing_id)
);

COMMIT;
