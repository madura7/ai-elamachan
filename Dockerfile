# syntax=docker/dockerfile:1
#
# Multi-stage build: Go API + seed binaries, golang-migrate v4.17.1,
# and Meilisearch v1.8 co-located in a single slim runtime image.
#
# Build context: repo root (COPY backend/ and COPY entrypoint.sh both work).

# ── Stage 1: Build Go binaries ─────────────────────────────────────────────
FROM golang:1.25-bookworm AS gobuilder

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/seed ./cmd/seed

# ── Stage 2: golang-migrate binary ─────────────────────────────────────────
FROM debian:bookworm-slim AS migrate-dl

RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL \
    "https://github.com/golang-migrate/migrate/releases/download/v4.17.1/migrate.linux-amd64.tar.gz" \
    | tar xz -C /usr/local/bin migrate

# ── Stage 3: Meilisearch binary ────────────────────────────────────────────
FROM getmeili/meilisearch:v1.8 AS meilisearch

# ── Stage 4: Runtime ───────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=gobuilder   /out/api                  /usr/local/bin/api
COPY --from=gobuilder   /out/seed                 /usr/local/bin/seed
COPY --from=migrate-dl  /usr/local/bin/migrate    /usr/local/bin/migrate
COPY --from=meilisearch /bin/meilisearch          /usr/local/bin/meilisearch

COPY backend/migrations /migrations
COPY entrypoint.sh      /entrypoint.sh
RUN  chmod +x /entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["/entrypoint.sh"]
