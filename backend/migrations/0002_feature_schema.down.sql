-- Roll back 0002_feature_schema.
-- Drops feature tables in reverse dependency order, then the enum types.
-- Returns the schema to the 0001 baseline (extensions + app_meta).

BEGIN;

DROP TABLE IF EXISTS listing_images;
DROP TABLE IF EXISTS listing_translations;
DROP TABLE IF EXISTS listings;

DROP TABLE IF EXISTS attribute_translations;
DROP TABLE IF EXISTS attributes;

DROP TABLE IF EXISTS category_translations;
DROP TABLE IF EXISTS categories;

DROP TABLE IF EXISTS otp_challenges;
DROP TABLE IF EXISTS users;

-- Enum types are dropped after the tables that reference them.
DROP TYPE IF EXISTS listing_translation_source;
DROP TYPE IF EXISTS otp_purpose;

COMMIT;
