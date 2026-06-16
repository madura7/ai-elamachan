-- 0002_feature_schema — first feature migration.
-- Creates the MVP domain tables that 0001 (baseline: extensions + app_meta)
-- intentionally left out: users + OTP auth, the localized taxonomy
-- (categories / attributes), and listings with their translation + image
-- companions.
--
-- Design references:
--   ADR 0001 (multi-language storage): every translatable entity has a
--     normalized *_translations companion keyed by (entity_id, lang); listings
--     carry content_language and store authored + machine text in
--     listing_translations. lang is CHECK-constrained to ('si','ta','en').
--   ADR 0002 (auth): phone/OTP primary, schema kept auth-method-agnostic
--     (email / password_hash present but unused at MVP). OTP codes stored hashed.
--
-- Relies on pgcrypto (gen_random_uuid) and citext from 0001_init.

BEGIN;

-- Reusable language guard. lang is fixed to the three MVP languages; adding a
-- 4th later is a small migration, but per ADR 0001 the *values* live in data,
-- not the schema. CHAR(2): 'si' | 'ta' | 'en'.
-- (Declared inline per-table below rather than as a DOMAIN to keep each table
-- self-describing and rollback trivial.)

-- ---------------------------------------------------------------------------
-- Auth: users + OTP challenges (ADR 0002)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Nullable + UNIQUE: multiple NULLs are allowed in Postgres, so future
    -- email-only / OAuth users need no phone. E.164 is at most 16 chars ('+' + 15 digits).
    phone_e164          VARCHAR(16) UNIQUE,
    phone_verified_at   TIMESTAMPTZ,
    -- Reserved for the future email/OAuth path; unused at MVP. citext = case-insensitive.
    email               CITEXT UNIQUE,
    password_hash       TEXT,
    display_name        TEXT NOT NULL,
    preferred_language  CHAR(2) NOT NULL DEFAULT 'en'
                        CHECK (preferred_language IN ('si','ta','en')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE IF NOT EXISTS otp_purpose AS ENUM ('signup','login');

CREATE TABLE IF NOT EXISTS otp_challenges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_e164      VARCHAR(16) NOT NULL,
    -- Store a HASH of the OTP, never the plaintext code (ADR 0002).
    code_hash       TEXT NOT NULL,
    purpose         otp_purpose NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    consumed_at     TIMESTAMPTZ,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Verify-path lookups fetch the latest live challenge for a phone.
CREATE INDEX IF NOT EXISTS idx_otp_challenges_phone ON otp_challenges (phone_e164, created_at DESC);
-- Supports cleanup / expiry sweeps.
CREATE INDEX IF NOT EXISTS idx_otp_challenges_expires_at ON otp_challenges (expires_at);

-- ---------------------------------------------------------------------------
-- Taxonomy: categories + attributes, each with a translations companion (ADR 0001)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    parent_id   UUID REFERENCES categories (id) ON DELETE RESTRICT,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories (parent_id);

CREATE TABLE IF NOT EXISTS category_translations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    lang        CHAR(2) NOT NULL CHECK (lang IN ('si','ta','en')),
    name        TEXT NOT NULL,
    UNIQUE (category_id, lang)
);

-- "List categories in <lang>, ordered by localized name" is a hot path.
CREATE INDEX IF NOT EXISTS idx_category_translations_lang_name ON category_translations (lang, name);

CREATE TABLE IF NOT EXISTS attributes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key         TEXT NOT NULL UNIQUE,
    data_type   TEXT NOT NULL CHECK (data_type IN ('string','number','boolean','enum')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS attribute_translations (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attribute_id UUID NOT NULL REFERENCES attributes (id) ON DELETE CASCADE,
    lang         CHAR(2) NOT NULL CHECK (lang IN ('si','ta','en')),
    label        TEXT NOT NULL,
    UNIQUE (attribute_id, lang)
);

CREATE INDEX IF NOT EXISTS idx_attribute_translations_lang ON attribute_translations (lang);

-- ---------------------------------------------------------------------------
-- Listings + translations + images (ADR 0001)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS listings (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    category_id       UUID NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    -- The single language the seller authored in. Title/description text lives
    -- in listing_translations (source='human' for the original) — never here.
    content_language  CHAR(2) NOT NULL CHECK (content_language IN ('si','ta','en')),
    price_cents       BIGINT CHECK (price_cents IS NULL OR price_cents >= 0),
    currency          CHAR(3) NOT NULL DEFAULT 'LKR',
    status            TEXT NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft','active','sold','expired','removed')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_listings_user_id ON listings (user_id);
-- Category browse feed: active listings in a category, newest first.
CREATE INDEX IF NOT EXISTS idx_listings_category_status_created ON listings (category_id, status, created_at DESC);

CREATE TYPE IF NOT EXISTS listing_translation_source AS ENUM ('human','machine');

CREATE TABLE IF NOT EXISTS listing_translations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id    UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    lang          CHAR(2) NOT NULL CHECK (lang IN ('si','ta','en')),
    title         TEXT NOT NULL,
    description   TEXT,
    -- 'human' = the seller's original; 'machine' = Claude-generated, regenerable
    -- and clearly flagged in the UI. Machine text never overwrites the original.
    source        listing_translation_source NOT NULL,
    generated_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (listing_id, lang)
);

CREATE TABLE IF NOT EXISTS listing_images (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id  UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    -- Object store key (e.g. S3/GCS); CDN URL is derived at read time.
    object_key  TEXT NOT NULL,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (listing_id, sort_order)
);

CREATE INDEX IF NOT EXISTS idx_listing_images_listing_id ON listing_images (listing_id);

COMMIT;
