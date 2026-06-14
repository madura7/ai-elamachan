#!/usr/bin/env python3
"""
Agent Auditor (VER-70 / 68a) — daily, read-only fleet audit.

Produces a structured per-agent audit (token usage, week-over-week trend,
synthetic token-derived cost, run volume / drift signals, and a model-fit
check against the §3 model-assignment matrix) and appends a dated entry to a
queryable audit ledger document on the parent design issue (VER-68).

READ-ONLY with respect to agent configuration: this tool never writes
adapterConfig / model assignments. Its only write is appending to the
audit-ledger issue document. Proposals (68b), config writes (68c) and report
formatting (68d) are explicitly out of scope.

Data sources (all Paperclip control-plane API, verified live):
  - GET /api/companies/{c}/costs/by-agent[?from&to]  -> per-agent tokens + run counts (date-windowed)
  - GET /api/companies/{c}/adapters/claude_local/models -> selectable model ids
  - GET /api/companies/{c}/agents                    -> role + adapterConfig.model per agent
  - GET /api/companies/{c}/dashboard                  -> runActivity[] (fleet success/failure per day)
  - GET /api/companies/{c}/activity                   -> recent event stream (drift mining)

Auth/config from env (injected during a Paperclip heartbeat/routine run):
  PAPERCLIP_API_URL, PAPERCLIP_API_KEY, PAPERCLIP_COMPANY_ID
Optional:
  PAPERCLIP_RUN_ID         -> stamped into the X-Paperclip-Run-Id header + entry metadata
  AUDIT_LEDGER_ISSUE_ID    -> override target ledger issue (default: VER-68 below)

Usage:
  python3 scripts/agent_audit.py [--date YYYY-MM-DD] [--dry-run] [--ledger-issue-id ID]

--date defaults to "yesterday" (UTC) — the last complete day, because the cost
rollup does not include the in-flight current day. Re-running for a date that is
already in the ledger replaces that day's entry (idempotent).
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

# Ledger lives on a dedicated ledger issue (VER-79), a child of the parent design
# issue VER-68 — §5 approved storage ("a dedicated ledger issue"). A dedicated
# issue is required because VER-68 itself is outside the Auditor agent's write
# authorization boundary.
DEFAULT_LEDGER_ISSUE_ID = "3baf2078-b210-4ff8-b9b9-7ecac0f05fe3"  # VER-81
LEDGER_KEY = "audit-ledger"
SCHEMA_VERSION = 1

# --- Synthetic-cost pricing table (public Anthropic list pricing, USD / 1M tokens) ---
# NOTE: Fleet runs on claude_local *subscription* runs, so Paperclip bills $0
# (costCents == 0). These figures derive a *synthetic, directional* cost from
# token counts. Label everywhere as "estimated (subscription, token-derived)" —
# never present as billed spend. Fable/Mythos are newer; their prices are
# best-effort estimates pending a confirmed public rate.
PRICING = {
    # model_id: (input_per_mtok, cached_input_per_mtok, output_per_mtok), tier
    "claude-opus-4-8":            (15.0, 1.50, 75.0),
    "claude-opus-4-7":            (15.0, 1.50, 75.0),
    "claude-opus-4-6":            (15.0, 1.50, 75.0),
    "claude-fable-5":             (15.0, 1.50, 75.0),   # estimate (opus-tier)
    "claude-mythos-5":            (15.0, 1.50, 75.0),   # estimate (opus-tier)
    "claude-sonnet-4-6":          (3.0,  0.30, 15.0),
    "claude-sonnet-4-5-20250929": (3.0,  0.30, 15.0),
    "claude-haiku-4-6":           (1.0,  0.10,  5.0),
    "claude-haiku-4-5-20251001":  (1.0,  0.10,  5.0),
}
PRICING_ESTIMATE_MODELS = {"claude-fable-5", "claude-mythos-5"}

# Tier ranking for model-fit comparison: higher rank == more capable / costlier.
TIER_RANK = {"haiku": 1, "sonnet": 2, "fable": 3, "opus": 4, "mythos": 4}


def model_tier(model_id: str) -> str:
    for t in ("haiku", "sonnet", "fable", "mythos", "opus"):
        if t in model_id:
            return t
    return "opus"  # unknown -> treat as top tier (conservative)


# Harness default when an agent has empty adapterConfig (= today's fleet state).
HARNESS_DEFAULT_MODEL = "claude-opus-4-8"

# §3 model-assignment matrix: role -> (min-viable model id, escalation note).
ROLE_MATRIX = {
    "ceo":        ("claude-fable-5",  "Opus 4.8 for major planning"),
    "cto":        ("claude-sonnet-4-6", "Opus 4.7 for complex design"),
    "engineer":   ("claude-sonnet-4-6", "Opus 4.7 for flagged-hard issues / security / large diffs"),
    "qa":         ("claude-sonnet-4-6", "Haiku 4.6 for smoke/routine; Sonnet 4.6 for exploratory"),
    "researcher": ("claude-sonnet-4-6", "Fable 5 for deep research"),
    "designer":   ("claude-sonnet-4-6", "Opus for complex creative / brand-critical work"),
}


def api(method: str, path: str, body: dict | None = None) -> object:
    base = os.environ["PAPERCLIP_API_URL"].rstrip("/")
    url = base + path
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", "Bearer " + os.environ["PAPERCLIP_API_KEY"])
    req.add_header("Content-Type", "application/json")
    run_id = os.environ.get("PAPERCLIP_RUN_ID")
    if run_id:
        req.add_header("X-Paperclip-Run-Id", run_id)
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            raw = resp.read().decode()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        detail = e.read().decode(errors="replace")
        raise RuntimeError(f"{method} {path} -> HTTP {e.code}: {detail}") from None


def daterange_str(d: dt.date) -> str:
    return d.isoformat()


def window_tokens(company: str, start: dt.date, end: dt.date) -> dict[str, dict]:
    """Return {agentId: cost-row} for an INCLUSIVE [start, end] date window.

    The costs/by-agent `to` parameter is EXCLUSIVE (verified live: from=D&to=D
    returns nothing; from=D&to=D+1 returns day D). We add one day to `end` so
    callers can think in inclusive ranges.
    """
    q = urllib.parse.urlencode({
        "from": daterange_str(start),
        "to": daterange_str(end + dt.timedelta(days=1)),
    })
    rows = api("GET", f"/api/companies/{company}/costs/by-agent?{q}") or []
    return {r["agentId"]: r for r in rows}


def synth_cost(model_id: str, inp: int, cached: int, out: int) -> float:
    price = PRICING.get(model_id) or PRICING[HARNESS_DEFAULT_MODEL]
    pin, pcached, pout = price
    return round(inp / 1e6 * pin + cached / 1e6 * pcached + out / 1e6 * pout, 4)


def pct_trend(curr: float, prior: float) -> float | None:
    if prior == 0:
        return None  # undefined (no prior baseline)
    return round((curr - prior) / prior * 100.0, 1)


def build_entry(company: str, target: dt.date) -> dict:
    # Windows: the audited day, the trailing 7d, and the 7d before that (for WoW).
    day_start = day_end = target
    cur7_start, cur7_end = target - dt.timedelta(days=6), target
    prev7_start, prev7_end = target - dt.timedelta(days=13), target - dt.timedelta(days=7)

    day = window_tokens(company, day_start, day_end)
    cur7 = window_tokens(company, cur7_start, cur7_end)
    prev7 = window_tokens(company, prev7_start, prev7_end)

    agents = api("GET", f"/api/companies/{company}/agents") or []
    models = api("GET", f"/api/companies/{company}/adapters/claude_local/models") or []
    model_ids = {m["id"] for m in models}

    # Fleet-level run success/failure for the audited day (per-agent split not exposed).
    dash = api("GET", f"/api/companies/{company}/dashboard") or {}
    run_activity = {r["date"]: r for r in (dash.get("runActivity") or [])}
    day_runs = run_activity.get(target.isoformat(), {"succeeded": 0, "failed": 0, "other": 0, "total": 0})

    # Drift mining from the recent activity stream (covers ~last 2 days at limit=500).
    activity = api("GET", f"/api/companies/{company}/activity?limit=500") or []
    stale_by_agent: dict[str, int] = {}
    for ev in activity:
        if ev.get("action") == "heartbeat.output_stale_detected":
            aid = ev.get("agentId")
            stale_by_agent[aid] = stale_by_agent.get(aid, 0) + 1
    activity_oldest = activity[-1]["createdAt"] if activity else None

    per_agent = []
    fleet_est_cost = 0.0
    for a in agents:
        aid = a["id"]
        role = a.get("role") or "unknown"
        name = a.get("name") or aid
        configured_model = (a.get("adapterConfig") or {}).get("model")
        effective_model = configured_model or HARNESS_DEFAULT_MODEL

        d = day.get(aid, {})
        c7 = cur7.get(aid, {})
        p7 = prev7.get(aid, {})

        d_in = d.get("inputTokens", 0)
        d_cached = d.get("cachedInputTokens", 0)
        d_out = d.get("outputTokens", 0)
        d_runs = d.get("subscriptionRunCount", 0) + d.get("apiRunCount", 0)

        est_cost_day = synth_cost(effective_model, d_in, d_cached, d_out)
        fleet_est_cost += est_cost_day

        # Week-over-week trend on total (input+cached+output) tokens.
        cur_total = c7.get("inputTokens", 0) + c7.get("cachedInputTokens", 0) + c7.get("outputTokens", 0)
        prev_total = p7.get("inputTokens", 0) + p7.get("cachedInputTokens", 0) + p7.get("outputTokens", 0)

        # Drift / failure signal. Per-agent run success is not exposed by the API
        # (no run.* events); we use a derived signal: stale-heartbeat events +
        # current error status, relative to recent run volume. Confidence: low.
        stale = stale_by_agent.get(aid, 0)
        status = a.get("status") or d.get("agentStatus")
        c7_runs = c7.get("subscriptionRunCount", 0) + c7.get("apiRunCount", 0)
        drift_flag = stale > 0 or status == "error"
        failure_signals = stale + (1 if status == "error" else 0)
        # Derived failure-rate over the trailing-7d run volume (best-effort).
        failure_rate = round(min(1.0, failure_signals / c7_runs), 3) if c7_runs else None
        success_rate = round(1.0 - failure_rate, 3) if failure_rate is not None else None

        # Model-fit: compare configured tier vs role's matrix target tier.
        # Only flag over-provisioned when adapterConfig.model is explicitly set —
        # empty config means the harness manages the model and the actual running
        # model is unverified; we cannot safely conclude over-provisioning.
        target_model, escalation = ROLE_MATRIX.get(role, (None, None))
        fit = {"target_model": target_model, "effective_model": effective_model,
               "configured_explicitly": configured_model is not None,
               "escalation": escalation}
        if not configured_model:
            # Model unverified — harness-managed, not safe to assess fit.
            fit["over_provisioned"] = None
            fit["tiers_above_target"] = None
        elif target_model:
            eff_rank = TIER_RANK[model_tier(configured_model)]
            tgt_rank = TIER_RANK[model_tier(target_model)]
            fit["over_provisioned"] = eff_rank > tgt_rank
            fit["tiers_above_target"] = max(0, eff_rank - tgt_rank)
        else:
            fit["over_provisioned"] = None
            fit["tiers_above_target"] = None

        per_agent.append({
            "agentId": aid,
            "name": name,
            "role": role,
            "effectiveModel": effective_model,
            "modelSource": "configured" if configured_model else "harness-default(assumed)",
            "day": {
                "inputTokens": d_in, "cachedInputTokens": d_cached, "outputTokens": d_out,
                "runs": d_runs,
                "estCostUsd": est_cost_day,
                "estCostBasis": "estimated (subscription, token-derived)",
                "estCostPriceEstimate": effective_model in PRICING_ESTIMATE_MODELS,
            },
            "trend7d": {
                "currentWindow": [cur7_start.isoformat(), cur7_end.isoformat()],
                "priorWindow": [prev7_start.isoformat(), prev7_end.isoformat()],
                "currentTotalTokens": cur_total,
                "priorTotalTokens": prev_total,
                "wowPctChange": pct_trend(cur_total, prev_total),
                "currentRuns": c7_runs,
            },
            "reliability": {
                "successRate": success_rate,
                "failureRate": failure_rate,
                "driftFlag": drift_flag,
                "staleHeartbeatEvents": stale,
                "agentStatus": status,
                "confidence": "low (derived: no per-agent run.* telemetry; stale-heartbeat + status proxy)",
            },
            "modelFit": fit,
        })

    # Sort by day est cost desc for readability.
    per_agent.sort(key=lambda r: r["day"]["estCostUsd"], reverse=True)

    fleet_fail_rate = round(day_runs["failed"] / day_runs["total"], 3) if day_runs.get("total") else None
    return {
        "schemaVersion": SCHEMA_VERSION,
        "auditDate": target.isoformat(),
        "generatedByRunId": os.environ.get("PAPERCLIP_RUN_ID"),
        "windows": {
            "day": [day_start.isoformat(), day_end.isoformat()],
            "current7d": [cur7_start.isoformat(), cur7_end.isoformat()],
            "prior7d": [prev7_start.isoformat(), prev7_end.isoformat()],
        },
        "fleet": {
            "estCostUsdDay": round(fleet_est_cost, 4),
            "estCostBasis": "estimated (subscription, token-derived)",
            "runActivityDay": day_runs,
            "fleetFailureRateDay": fleet_fail_rate,
            "agentCount": len(per_agent),
        },
        "selectableModels": sorted(model_ids),
        "dataNotes": [
            "Real billed cost unavailable: fleet runs claude_local subscription runs (costCents=0). All costs are synthetic/token-derived.",
            "Per-agent run success/failure is not exposed (no run.* events); reliability fields are derived proxies (low confidence).",
            f"Drift mining covers recent activity only (oldest event seen: {activity_oldest}).",
            "Agents with empty adapterConfig.model use the harness default for synthetic cost only; model-fit is marked 'unverified' (over_provisioned=null) since the actual running model cannot be confirmed from adapterConfig alone.",
        ],
        "perAgent": per_agent,
    }


# --- Ledger document rendering (markdown + embedded JSON, append/replace by date) ---

ENTRY_BEGIN = "<!-- audit-entry:{date} -->"
ENTRY_END = "<!-- /audit-entry:{date} -->"


def render_entry_md(entry: dict) -> str:
    d = entry["auditDate"]
    fleet = entry["fleet"]
    lines = []
    lines.append(ENTRY_BEGIN.format(date=d))
    lines.append(f"## Audit {d}")
    lines.append("")
    lines.append(f"- Fleet synthetic spend (day): **${fleet['estCostUsdDay']:.4f}** "
                 f"_(estimated, subscription token-derived — not billed)_")
    ra = fleet["runActivityDay"]
    lines.append(f"- Fleet runs (day): {ra.get('total', 0)} total "
                 f"({ra.get('succeeded', 0)} ok / {ra.get('failed', 0)} failed / {ra.get('other', 0)} other)")
    lines.append(f"- Agents audited: {fleet['agentCount']}")
    lines.append("")
    lines.append("| Agent | Role | Eff. model | Day tokens (in/cache/out) | Day est$ | WoW tok% | Runs(7d) | Fail rate | Drift | Model-fit |")
    lines.append("|---|---|---|---|---|---|---|---|---|---|")
    for a in entry["perAgent"]:
        day = a["day"]
        tr = a["trend7d"]
        rel = a["reliability"]
        fit = a["modelFit"]
        wow = "n/a" if tr["wowPctChange"] is None else f"{tr['wowPctChange']:+.1f}%"
        fr = "n/a" if rel["failureRate"] is None else f"{rel['failureRate']:.0%}"
        drift = "⚠️" if rel["driftFlag"] else "—"
        if fit["over_provisioned"]:
            mf = f"⬇ over by {fit['tiers_above_target']} tier(s) → {fit['target_model']}"
        elif fit["over_provisioned"] is False:
            mf = "ok"
        else:
            mf = "n/a"
        toks = f"{day['inputTokens']}/{day['cachedInputTokens']}/{day['outputTokens']}"
        star = "*" if day["estCostPriceEstimate"] else ""
        lines.append(f"| {a['name']} | {a['role']} | {a['effectiveModel']}{star} | {toks} | "
                     f"${day['estCostUsd']:.4f} | {wow} | {tr['currentRuns']} | {fr} | {drift} | {mf} |")
    lines.append("")
    lines.append("<details><summary>Structured data (machine-queryable JSON)</summary>")
    lines.append("")
    lines.append("```json")
    lines.append(json.dumps(entry, indent=2))
    lines.append("```")
    lines.append("")
    lines.append("</details>")
    lines.append(ENTRY_END.format(date=d))
    return "\n".join(lines)


LEDGER_HEADER = """# Agent Audit Ledger

Append-only daily fleet audit produced by the **Agent Auditor** routine (read-only).
Each dated section carries a human-readable summary table plus a machine-queryable
JSON block (consumed by 68b Improver / 68d morning report). Costs are **synthetic
(subscription, token-derived)** — never billed spend.

Newest entry first.
"""


ENTRY_RE = re.compile(
    r"<!-- audit-entry:(\d{4}-\d{2}-\d{2}) -->.*?<!-- /audit-entry:\1 -->",
    re.DOTALL,
)


def splice_entry(existing_body: str | None, entry_md: str, date: str) -> str:
    """Insert/replace the dated entry and re-emit all entries newest-date first.

    Robust to out-of-order writes (e.g. backfill): entries are parsed, the new
    one upserted by date, then sorted descending. Idempotent for a given date.
    """
    entries: dict[str, str] = {}
    if existing_body:
        for m in ENTRY_RE.finditer(existing_body):
            entries[m.group(1)] = m.group(0)
    entries[date] = entry_md  # upsert (replaces same-date entry)
    ordered = [entries[d] for d in sorted(entries, reverse=True)]
    return LEDGER_HEADER + "\n" + "\n\n".join(ordered) + "\n"


def upsert_ledger(issue_id: str, entry: dict, dry_run: bool) -> None:
    entry_md = render_entry_md(entry)
    if dry_run:
        print(entry_md)
        return
    base_rev = None
    existing_body = None
    try:
        doc = api("GET", f"/api/issues/{issue_id}/documents/{LEDGER_KEY}")
        if doc:
            existing_body = doc.get("body")
            base_rev = doc.get("latestRevisionId")
    except RuntimeError as e:
        if "HTTP 404" not in str(e):
            raise  # genuine error; 404 just means first-ever entry

    new_body = splice_entry(existing_body, entry_md, entry["auditDate"])
    api("PUT", f"/api/issues/{issue_id}/documents/{LEDGER_KEY}", {
        "title": "Agent Audit Ledger",
        "format": "markdown",
        "body": new_body,
        "baseRevisionId": base_rev,
    })
    print(f"Ledger updated: issue {issue_id} doc '{LEDGER_KEY}' (audit {entry['auditDate']}, "
          f"{entry['fleet']['agentCount']} agents).")


def main() -> int:
    ap = argparse.ArgumentParser(description="Daily read-only agent fleet auditor (VER-70 / 68a)")
    ap.add_argument("--date", help="Audit date YYYY-MM-DD (default: yesterday UTC)")
    ap.add_argument("--ledger-issue-id", default=os.environ.get("AUDIT_LEDGER_ISSUE_ID", DEFAULT_LEDGER_ISSUE_ID))
    ap.add_argument("--dry-run", action="store_true", help="Print the entry; do not write the ledger")
    args = ap.parse_args()

    for var in ("PAPERCLIP_API_URL", "PAPERCLIP_API_KEY", "PAPERCLIP_COMPANY_ID"):
        if not os.environ.get(var):
            print(f"ERROR: missing required env var {var}", file=sys.stderr)
            return 2

    company = os.environ["PAPERCLIP_COMPANY_ID"]
    if args.date:
        target = dt.date.fromisoformat(args.date)
    else:
        target = dt.datetime.now(dt.timezone.utc).date() - dt.timedelta(days=1)

    entry = build_entry(company, target)
    upsert_ledger(args.ledger_issue_id, entry, args.dry_run)
    return 0


if __name__ == "__main__":
    sys.exit(main())
