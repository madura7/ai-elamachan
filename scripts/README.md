# Fleet ops scripts

Meta-tooling for operating the Paperclip agent fleet. These are **not** part of
the ElaMachan product; they talk to the Paperclip control-plane API, not the app.

## `agent_audit.py` — Agent Auditor (VER-70 / 68a)

Daily, **read-only** fleet audit. Gathers per-agent telemetry and appends a dated
entry to the `audit-ledger` issue document on the dedicated ledger issue VER-79
(a child of the parent design issue VER-68; §5 storage). VER-68 itself is outside
the Auditor agent's write boundary, so the ledger lives on a dedicated issue.

It computes, per agent per day:

- **Token totals** (input / cached / output) + **week-over-week** token trend.
- **Synthetic cost** = tokens × public model pricing. The fleet runs on
  `claude_local` *subscription* runs, so Paperclip bills `$0`; this cost is
  always labelled **`estimated (subscription, token-derived)`** — never billed spend.
- **Run volume + drift signals.** Per-agent run success/failure is not exposed
  by the API (no `run.*` events), so reliability fields are derived proxies
  (stale-heartbeat events + agent error status over trailing-7d run volume) and
  marked low-confidence. Fleet-level success/failure comes from the dashboard.
- **Model-fit check** vs the §3 model-assignment matrix — flags any agent on a
  tier higher than its role's min-viable target.

### Run

Auth/config come from env (auto-injected during a Paperclip heartbeat/routine run):
`PAPERCLIP_API_URL`, `PAPERCLIP_API_KEY`, `PAPERCLIP_COMPANY_ID`.

```bash
# Audit yesterday (default) and append to the ledger:
python3 scripts/agent_audit.py

# Audit a specific day without writing (preview):
python3 scripts/agent_audit.py --date 2026-06-12 --dry-run

# Override the target ledger issue:
python3 scripts/agent_audit.py --ledger-issue-id <issueId>
```

Re-running for a date already in the ledger **replaces** that day's entry (idempotent).
Stdlib only — no dependencies.

### Important: `costs/by-agent` date semantics

The `to` query parameter is **exclusive** (verified live: `from=D&to=D` returns
nothing; `from=D&to=D+1` returns day `D`). `window_tokens()` adds one day to the
inclusive `end` it is given, so the rest of the code works in inclusive ranges.

### Daily routine

The auditor runs unattended via a Paperclip **routine** (`schedule` trigger,
daily). Each fire creates an execution issue assigned to the Engineer agent; the
heartbeat runs `python3 scripts/agent_audit.py`, which audits the prior complete
day and appends to the ledger. See VER-70 for the routine id.

### Scope

Read-only inventory + compute + record. Out of scope: optimization proposals
(68b), config writes (68c), report formatting (68d).

---

## `ceo_morning_report.py` — CEO Morning Report (VER-72 / 68d)

Daily report generator. Reads the latest `audit-ledger` entry from VER-81 and
delivers a morning briefing to the CEO as a new Paperclip issue.

The report includes:

- **Fleet synthetic spend** (labeled `estimated (subscription, token-derived)` — never billed spend) + WoW trend.
- **Per-agent usage table** (tokens, synthetic cost, WoW %, drift, model-fit).
- **Drift flags** — agents with stale-heartbeat events or error status.
- **Model-fit summary** vs the §3 model-assignment matrix.
- **Proposed optimizations** split into:
  - `auto-approve-eligible` — no drift, ≤ 5% failure rate, not the CEO's own config.
  - `escalate-to-CEO` — drift, high failure rate, or the CEO's own config.

### Run

```bash
# Generate and deliver to CEO:
python3 scripts/ceo_morning_report.py

# Preview without creating an issue:
python3 scripts/ceo_morning_report.py --dry-run

# Override the target ledger issue:
python3 scripts/ceo_morning_report.py --ledger-issue-id <issueId>
```

Stdlib only — no dependencies.

### Daily routine

Runs unattended via a Paperclip routine ("Daily CEO Morning Report (68d)",
schedule `0 9 * * *` UTC, 30 min after the 08:30 UTC auditor). Each fire
creates an execution issue assigned to the Engineer agent. See VER-72 for
the routine id (`b96e72d1`).

### Ledger schema

The `audit-ledger` entry schema is formally documented in the `ledger-schema`
document on VER-81 (query approach by date, by agent; full field reference;
pricing table). Schema version: 1.
