# ADR 0003 — Frontend↔backend contract (REST + OpenAPI)

- **Status:** Accepted
- **Date:** 2026-06-12
- **Owner:** Eng Manager
- **Context:** [VER-39](/VER/issues/VER-39), from [VER-38](/VER/issues/VER-38) finding C1

## Context

The Next.js (TypeScript) frontend and the Go backend need one agreed contract.
Candidates: **REST + OpenAPI** vs **gRPC**. The contract shapes every endpoint
and every data-fetching call, so it is locked before feature code.

## Decision

**REST over HTTP with JSON, described by a single OpenAPI 3.1 spec that is the
source of truth.**

- The spec lives in the repo at the canonical path `api/openapi.yaml` (repo
  root, shared by both frontend and backend — not nested under `backend/`).
- Frontend generates typed TS client/types from the spec (e.g.
  `openapi-typescript` / `orval`) — no hand-written request types drifting from
  the server.
- Backend validates requests/responses against the spec (handler-side or
  middleware) so the spec cannot silently rot.
- Endpoints are versioned under `/api/v1/...`.
- Errors use a consistent JSON envelope (e.g. `{ "error": { "code", "message" } }`),
  finalized in the API-conventions task.

gRPC is **not** used for the browser↔backend contract. It may be reconsidered
later, via a superseding ADR, only for internal service-to-service traffic if we
split services.

## Rationale

- **MVP speed:** REST+OpenAPI is the fastest path — native to browsers and
  Next.js (Server Components, route handlers, `fetch`), no gRPC-web proxy layer.
- **Tooling:** mature codegen for typed TS clients and Go server stubs; easy to
  curl, log, and debug.
- **Fit:** a classifieds CRUD + search MVP has no need for gRPC's streaming or
  binary-perf advantages. Those benefits don't pay for the added browser-side
  complexity here.

## Consequences

- Any change to the API contract is a change to `api/openapi.yaml` first; per the
  merge policy, **API-contract changes escalate to the Board**, so the spec is
  the controlled artifact.
- Both frontend and backend tasks depend on the spec existing; standing up the
  initial `openapi.yaml` skeleton belongs with the dev/CI/migrations foundation
  ([VER-40](/VER/issues/VER-40)) or the first API task.
- CI should fail if generated TS types are out of sync with the spec.
