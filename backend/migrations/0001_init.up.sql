-- 0001_init — baseline migration.
-- Establishes the migrations toolchain and the minimal foundations every later
-- migration relies on. Intentionally small: feature tables (users, listings,
-- categories) land in their own migrations under their respective feature issues.

BEGIN;

-- pgcrypto provides gen_random_uuid() for UUID primary keys.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- citext gives case-insensitive text (used later for emails / usernames).
CREATE EXTENSION IF NOT EXISTS "citext";

-- Lightweight metadata table so the baseline is verifiable and gives later
-- migrations a place to record app-level schema metadata if needed.
CREATE TABLE IF NOT EXISTS app_meta (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO app_meta (key, value)
VALUES ('schema_baseline', '0001_init')
ON CONFLICT (key) DO NOTHING;

COMMIT;
