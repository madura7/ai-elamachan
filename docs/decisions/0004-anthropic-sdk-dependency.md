# ADR 0004 — Anthropic Go SDK for AI-assisted listings

- **Status:** Proposed (pending Board merge approval per the engineering merge policy)
- **Date:** 2026-06-13
- **Owner:** Eng Manager
- **Context:** [VER-58](/VER/issues/VER-58) (productionize the AI-assist endpoint),
  parent [VER-45](/VER/issues/VER-45), spike [VER-42](/VER/issues/VER-42)
- **Security note:** this introduces a **new dependency with security surface**
  (an outbound LLM API client) plus an **API-contract change**. Per the merge
  policy in [.ai/project.md](../../.ai/project.md), the implementing PR is a
  Board-escalation item, not a Reviewer-only merge.

## Context

The headline differentiator (AI-assisted listing creation) needs the Go backend
to call the Claude API. The VER-42 spike validated a single forced-tool-use call
that returns a schema-valid trilingual draft. Productionizing it requires an
HTTP client for the Anthropic Messages API. Options:

- **Official `github.com/anthropics/anthropic-sdk-go`** — typed params, model id
  constants (`anthropic.ModelClaudeHaiku4_5`), tool-use + structured-output
  helpers, maintained by the provider.
- **Hand-rolled `net/http` client** — no new dependency, but we reimplement
  request/response types, tool-use plumbing, retries, and model constants, and
  carry that maintenance ourselves.

## Decision

Adopt the **official `github.com/anthropics/anthropic-sdk-go`** (pinned at
`v1.50.1`) as the backend's Claude client.

- The model id is **pinned** to `claude-haiku-4-5` in code (`internal/aiassist`),
  not read from the environment, per the VER-42 cost/latency finding.
- The SDK declares `go 1.24`, so the backend module's `go` directive and the CI
  Go version are bumped from `1.22` to `1.24` (see Consequences).

## Rationale

- **Correctness/safety:** the SDK's typed tool params and forced-`tool_choice`
  support make the no-free-text-parsing, no-prefill contract from VER-42 easy to
  express and hard to get wrong.
- **Maintenance:** model-id constants and Messages API types track the provider;
  we avoid drift in a security-sensitive path.
- **Cost is negligible at MVP scale** (~0.7¢/listing on Haiku 4.5), so the value
  is in correctness and speed-to-MVP, not avoiding a trivial client.

## Consequences

- **New third-party dependency with security surface** → Board-approval merge.
- `backend/go.mod` `go` directive: `1.22` → `1.24`; `.github/workflows/ci.yml`
  backend job `go-version`: `1.22` → `1.24`. Transitive deps are recorded in
  `go.sum`.
- The `ANTHROPIC_API_KEY` is a secret sourced from GCP Secret Manager
  ([docs/secrets.md](../secrets.md)); it is never committed and is read from the
  environment at runtime.
- Reversible: the SDK is isolated behind `internal/aiassist`; swapping it for a
  hand-rolled client later is a contained change.
