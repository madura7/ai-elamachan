# API contract (`openapi.yaml`)

`openapi.yaml` is the **single source of truth** for the ElaMachan HTTP API
(OpenAPI 3.1), per [ADR 0003](../docs/decisions/0003-api-contract.md). Both apps
derive from it — do not hand-write request/response types on either side.

## Locked conventions

- **Versioning:** every endpoint is under `/api/v1` (the spec `servers` base).
- **Error envelope:** all non-2xx responses are `{ "error": { "code", "message" } }`.
- **Trilingual content:** `en` / `si` / `ta` variants (ADR 0001).
- **Auth:** phone + OTP, bearer session token on protected endpoints (ADR 0002).

## How each app uses it

- **Frontend (`frontend/`)** generates a typed client from this file:
  - `npm run api:gen` → writes `frontend/src/lib/api/schema.ts` (committed).
  - `frontend/src/lib/api/client.ts` wraps it with `openapi-fetch` (typed `api`).
  - `npm run api:check` fails if the committed types drift from the spec (CI-enforced).
  - `npm run api:lint` validates the spec is OpenAPI 3.1 (CI-enforced).
- **Backend (`backend/`)** treats this file as the authoritative contract:
  request/response shapes must match it. `POST /listings/ai-draft` (VER-58) is the
  only implemented surface today.

## Changing the contract = Board escalation

Per the merge policy, any change to `openapi.yaml` is an API-contract change and
**escalates to the Board** — open a PR, do not self-merge.

## Surface status

| Path | Status |
| --- | --- |
| `POST /listings/ai-draft` | implemented (VER-58) |
| `POST /auth/otp/request`, `POST /auth/otp/verify` | stub (auth issue, ADR 0002) |
| `GET /categories` | stub (taxonomy issue, ADR 0001) |
| `GET /listings`, `POST /listings`, `GET /listings/{id}` | stub (listings issue) |
| `GET /search` | stub (search issue) |

> The backend's implemented ai-draft route currently lives at
> `/api/listings/ai-draft` (pre-dating the `/api/v1` lock). Moving it under
> `/api/v1` to match this spec is a small follow-up tracked separately.
