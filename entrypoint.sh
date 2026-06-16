#!/bin/sh
# Container boot sequence (idempotent):
#   1. migrate up    — apply any pending Postgres migrations
#   2. meilisearch   — start in background, wait for /health
#   3. seed          — re-index Meilisearch and upsert demo Postgres rows
#   4. exec api      — hand off to the Go HTTP server (PID 1)
set -e

# ── 1. Migrations ──────────────────────────────────────────────────────────
# DATABASE_URL must include sslmode=require for Neon.
echo "[boot] Running migrations..."
# If the database is in a dirty state (partially applied migration), reset to
# the last clean version so the transactional migration can be re-applied.
DIRTY_VERSION=$(migrate -path /migrations -database "$DATABASE_URL" version 2>&1 | grep -oP '^\d+(?= \(dirty\))' || true)
if [ -n "$DIRTY_VERSION" ]; then
  CLEAN_VERSION=$(( DIRTY_VERSION - 1 ))
  echo "[boot] Dirty migration at version ${DIRTY_VERSION}, forcing reset to ${CLEAN_VERSION}..."
  migrate -path /migrations -database "$DATABASE_URL" force "$CLEAN_VERSION"
fi
migrate -path /migrations -database "$DATABASE_URL" up
echo "[boot] Migrations OK."

# ── 2. Meilisearch ─────────────────────────────────────────────────────────
echo "[boot] Starting Meilisearch on 127.0.0.1:7700..."
mkdir -p /meili_data
meilisearch \
  --db-path /meili_data \
  --http-addr 127.0.0.1:7700 \
  --env "${MEILI_ENV:-production}" \
  &

echo "[boot] Waiting for Meilisearch health (up to 60 s)..."
i=0
until curl -sf http://127.0.0.1:7700/health > /dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 60 ]; then
    echo "[boot] ERROR: Meilisearch did not become healthy within 60 s"
    exit 1
  fi
  sleep 1
done
echo "[boot] Meilisearch ready."

# ── 3. Seed ────────────────────────────────────────────────────────────────
echo "[boot] Running seed..."
MEILI_URL="${MEILI_URL:-http://127.0.0.1:7700}" seed
echo "[boot] Seed OK."

# ── 4. API ─────────────────────────────────────────────────────────────────
echo "[boot] Starting API on :${BACKEND_PORT:-8080}..."
exec api
