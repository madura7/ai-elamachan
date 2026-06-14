# ElaMachan

A classified marketplace for the Sri Lankan market (inspired by [ikman.lk](https://ikman.lk)),
with **AI-assisted listing creation** powered by the Claude API as the headline feature.

Trilingual (Sinhala / Tamil / English), built as a monorepo with a Next.js frontend and a Go
backend. See [`.ai/project.md`](.ai/project.md) for the full product + engineering brief.

## Architecture

| Layer      | Tech                                   |
| ---------- | -------------------------------------- |
| Frontend   | Next.js 15 (App Router, TypeScript)    |
| Backend    | Go (HTTP API)                          |
| Database   | Postgres                               |
| Cache      | Redis                                  |
| Search     | Meilisearch                            |
| AI         | Claude API                             |
| Hosting    | GCP (+ Secret Manager for secrets)     |

```
.
├── api/        # openapi.yaml — the API contract (source of truth, ADR 0003)
├── frontend/   # Next.js app
├── backend/    # Go API + migrations
├── docs/       # engineering docs (secrets policy, etc.)
└── .github/    # CI workflows
```

## API contract

[`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.1) is the single source of
truth for the HTTP API, per [ADR 0003](docs/decisions/0003-api-contract.md).
Endpoints are versioned under `/api/v1`; every error uses the
`{ "error": { "code", "message" } }` envelope. The frontend generates its typed
client from the spec — no hand-written request types:

```bash
cd frontend
npm run api:lint    # validate the spec is OpenAPI 3.1
npm run api:gen     # regenerate src/lib/api/schema.ts from the spec
npm run api:check   # fail if the committed types drift from the spec
```

Changing the contract is a Board-escalation item — see [`api/README.md`](api/README.md).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) + Docker Compose v2
- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 20+
- [golang-migrate](https://github.com/golang-migrate/migrate) (optional locally — `make migrate-up`
  runs it via Docker, no install required)

## Quickstart

```bash
# 1. Configure local environment (no real secrets required for local infra)
cp .env.example .env

# 2. Bring up local infrastructure: Postgres + Redis + Meilisearch
docker compose up -d

# 3. Apply database migrations
make migrate-up
```

To tear everything down (and drop local data volumes):

```bash
docker compose down -v
```

## Database migrations

Migrations live in [`backend/migrations/`](backend/migrations) and are managed with
[golang-migrate](https://github.com/golang-migrate/migrate). The `Makefile` wraps the
`migrate/migrate` Docker image so no local install is needed:

```bash
make migrate-up                       # apply all pending migrations
make migrate-down                     # roll back the last migration
make migrate-create name=add_users    # scaffold a new migration pair
```

## Secrets

Never commit real secrets. The Claude API key, database credentials, and JWT secret live in
**GCP Secret Manager** in deployed environments and in your local untracked `.env` for
development. See [`docs/secrets.md`](docs/secrets.md) for the full policy.

## CI

Every pull request runs the [CI gate](.github/workflows/ci.yml): backend build/vet/test,
frontend lint/build, an API-contract check (spec validation + typed-client drift), and a
migrations apply check against a throwaway Postgres.

## Contributing

- Branch off `main`: `feat/VER-NNN-...` or `fix/...`. Never push to `main`.
- Open a PR; every PR is reviewed before merge.
- See the merge policy in [`.ai/project.md`](.ai/project.md).
