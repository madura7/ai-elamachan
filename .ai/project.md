# ElaMachan — Project Brief (canonical)

> This is the canonical product + engineering brief. Read this before assigning or doing work.

## What we are building

**ElaMachan** is a classified marketplace for the Sri Lankan market (modeled on
[ikman.lk](https://ikman.lk)). The key differentiator is **AI-assisted listing creation**
powered by the Claude API.

## MVP scope (target: working MVP in 6–8 weeks)

- **User auth** — sign up / sign in, sessions (JWT).
- **Listings** — create / edit / browse listings with images.
- **Categories** — browse and filter by category.
- **Search** — full-text + faceted search via **Meilisearch**.
- **Trilingual UI** — Sinhala / Tamil / English.
- **AI-assisted listing creation** — Claude API drafts title / description / category
  suggestions from minimal user input (the headline feature).

## Stack

Monorepo:

- `frontend/` — **Next.js 15**, App Router, TypeScript.
- `backend/` — **Go** (HTTP API).
- **Postgres** — primary datastore.
- **Redis** — cache / sessions / rate limiting.
- **Meilisearch** — search index.
- **GCP** — hosting + **Secret Manager** for secrets.
- **Claude API** — AI listing assistance.

## Repository

- GitHub: `https://github.com/madura7/ai-elamachan` (account: **madura7**).
- Branching: feature branches `feat/VER-NNN-...` / `fix/...`. Never commit to `main`.
- PRs open under the **madura7** account; every PR is reviewed before merge.

## Local development

```bash
cp .env.example .env      # fill in local values (no real secrets needed for infra)
docker compose up -d      # Postgres + Redis + Meilisearch
make migrate-up           # apply DB migrations (see Makefile)
```

See [README.md](../README.md) for the full developer quickstart and
[docs/secrets.md](../docs/secrets.md) for the secrets policy.

## Engineering foundations (laid by VER-40)

- `docker-compose.yml` — local Postgres + Redis + Meilisearch.
- `.env.example` + `docs/secrets.md` — config contract + secrets policy (GCP Secret Manager).
- `backend/migrations/` + `golang-migrate` — schema migrations.
- `.github/workflows/ci.yml` — lint + build + test gate on every PR.

## Merge policy (engineering)

Low-risk PRs (docs, tests, styling, small isolated fixes, dependency bumps) may be merged
after Reviewer approval. **Schema/migration changes, auth, payments, API-contract changes,
trust/fraud logic, new security-surface dependencies, or anything > ~200 lines** must be
escalated to the Board (via the CEO) and must not be merged unilaterally. Never deploy to
production without explicit Board approval.
