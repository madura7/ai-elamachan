-- Roll back 0001_init.

BEGIN;

DROP TABLE IF EXISTS app_meta;

-- Extensions are left in place: dropping them can affect other objects and they are
-- harmless to keep. Uncomment if a truly clean teardown is required.
-- DROP EXTENSION IF EXISTS "citext";
-- DROP EXTENSION IF EXISTS "pgcrypto";

COMMIT;
