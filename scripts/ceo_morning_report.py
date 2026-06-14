#!/usr/bin/env python3
"""
CEO Morning Report (VER-72 / 68d) — daily, read-only.

Reads the audit ledger produced by the Agent Auditor (68a) from the dedicated
ledger issue (VER-81, key `audit-ledger`), synthesises a morning briefing for the
CEO, and delivers it as a new Paperclip issue assigned to the CEO agent.

The report covers:
  - Fleet synthetic spend + WoW trend (token-derived; never billed spend)
  - Per-agent token / cost / WoW table
  - Drift flags surfaced by the auditor
  - Model-fit flags (all agents vs §3 model-assignment matrix)
  - Proposed model-tier optimizations split into:
      auto-approve-eligible  — no drift, low failure rate, not CEO's own config
      escalate-to-CEO        — drift, high failure rate, or CEO's own config

Delivery: creates one issue per run assigned to the CEO agent with the report as
its description. Each issue is idempotent for the calendar date; re-running on the
same day creates a second issue (acts as a re-send if needed).

Auth/config from env (auto-injected during a Paperclip heartbeat/routine run):
  PAPERCLIP_API_URL, PAPERCLIP_API_KEY, PAPERCLIP_COMPANY_ID
Optional:
  PAPERCLIP_RUN_ID         -> stamped into the X-Paperclip-Run-Id header
  AUDIT_LEDGER_ISSUE_ID    -> override target ledger issue (default: VER-81)

Usage:
  python3 scripts/ceo_morning_report.py [--dry-run] [--ledger-issue-id ID]

--dry-run prints the report to stdout without creating any Paperclip issue.
Stdlib only — no external dependencies.
"""
from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import re
import sys
import urllib.error
import urllib.request

# VER-81 — dedicated Agent Optimization Ledger issue (§5 approved storage)
DEFAULT_LEDGER_ISSUE_ID = "3baf2078-b210-4ff8-b9b9-7ecac0f05fe3"
LEDGER_KEY = "audit-ledger"

# CEO agent id (receives the daily report issue)
CEO_AGENT_ID = "520599f1-fb06-4df0-9dc1-83994a2f19ac"

# Regexes for parsing the ledger markdown
ENTRY_RE = re.compile(
    r"<!-- audit-entry:(\d{4}-\d{2}-\d{2}) -->(.*?)<!-- /audit-entry:\1 -->",
    re.DOTALL,
)
JSON_BLOCK_RE = re.compile(r"```json\n(.*?)\n```", re.DOTALL)


# ---------------------------------------------------------------------------
# Paperclip API helper
# ---------------------------------------------------------------------------

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


# ---------------------------------------------------------------------------
# Ledger parsing
# ---------------------------------------------------------------------------

def fetch_audit_entries(ledger_issue_id: str) -> list[dict]:
    """Return all parsed audit entries from the ledger, newest first."""
    try:
        doc = api("GET", f"/api/issues/{ledger_issue_id}/documents/{LEDGER_KEY}")
    except RuntimeError as e:
        if "HTTP 404" in str(e):
            return []
        raise
    body = (doc or {}).get("body") or ""
    entries: list[dict] = []
    for m in ENTRY_RE.finditer(body):
        section = m.group(2)
        jm = JSON_BLOCK_RE.search(section)
        if jm:
            try:
                entries.append(json.loads(jm.group(1)))
            except json.JSONDecodeError:
                pass
    entries.sort(key=lambda e: e.get("auditDate", ""), reverse=True)
    return entries


# ---------------------------------------------------------------------------
# Optimization classification
# ---------------------------------------------------------------------------

def classify_optimizations(
    entry: dict, ceo_agent_id: str
) -> tuple[list[dict], list[dict]]:
    """Split over-provisioned agents into auto-approve-eligible vs escalate-to-CEO.

    Auto-approve criteria (all must hold):
      - Agent is over-provisioned (modelFit.over_provisioned == True)
      - No drift flag
      - failureRate <= 5% OR no run data at all (0 runs, null failureRate)
      - Not the CEO's own config (avoid conflict of interest)

    Everything else escalates.
    """
    auto: list[dict] = []
    escalate: list[dict] = []

    for a in entry.get("perAgent", []):
        fit = a.get("modelFit", {})
        if not fit.get("over_provisioned"):
            continue

        rel = a.get("reliability", {})
        drift = rel.get("driftFlag", False)
        fail_rate = rel.get("failureRate")  # None = unknown / no runs
        c7_runs = a.get("trend7d", {}).get("currentRuns", 0)
        is_ceo_agent = a["agentId"] == ceo_agent_id
        high_fail = fail_rate is not None and fail_rate > 0.05
        # Unknown rate with actual runs in the window is suspicious
        unknown_with_runs = fail_rate is None and c7_runs > 0

        proposal = {
            "agent": a["name"],
            "agentId": a["agentId"],
            "role": a.get("role"),
            "current": fit.get("effective_model"),
            "target": fit.get("target_model"),
            "tiers_above": fit.get("tiers_above_target", 0),
            "escalation_note": fit.get("escalation"),
        }

        if is_ceo_agent or drift or high_fail or unknown_with_runs:
            reasons: list[str] = []
            if is_ceo_agent:
                reasons.append("CEO config — always escalate")
            if drift:
                stale = rel.get("staleHeartbeatEvents", 0)
                reasons.append(f"drift flag ({stale} stale-heartbeat event(s))")
            if high_fail:
                reasons.append(f"{fail_rate:.0%} failure rate")
            if unknown_with_runs:
                reasons.append(f"failure rate unknown ({c7_runs} runs in window)")
            proposal["reason"] = "; ".join(reasons) or "caution"
            escalate.append(proposal)
        else:
            if fail_rate is None:
                proposal["reason"] = "no run data (0 runs in 7d window); no evidence of issues"
            else:
                proposal["reason"] = f"no drift, {fail_rate:.0%} failure rate"
            auto.append(proposal)

    return auto, escalate


# ---------------------------------------------------------------------------
# Report rendering
# ---------------------------------------------------------------------------

def render_report(entry: dict, report_date: str, ceo_agent_id: str) -> str:
    audit_date = entry.get("auditDate", "unknown")
    fleet = entry.get("fleet", {})
    ra = fleet.get("runActivityDay", {})
    agents = entry.get("perAgent", [])

    auto_opts, escalate_opts = classify_optimizations(entry, ceo_agent_id)

    # Fleet WoW: aggregate current and prior 7d token totals across all agents
    cur_total_fleet = sum(
        a.get("trend7d", {}).get("currentTotalTokens", 0) for a in agents
    )
    prior_total_fleet = sum(
        a.get("trend7d", {}).get("priorTotalTokens", 0) for a in agents
    )
    if prior_total_fleet > 0:
        wow_fleet_pct = (cur_total_fleet - prior_total_fleet) / prior_total_fleet * 100.0
        wow_fleet_str = f"{wow_fleet_pct:+.1f}% (7d vs prior 7d)"
    else:
        wow_fleet_str = "n/a (no prior-week baseline yet)"

    drift_agents = [a for a in agents if a.get("reliability", {}).get("driftFlag")]
    over_agents = [a for a in agents if a.get("modelFit", {}).get("over_provisioned")]
    ok_agents = [a for a in agents if a.get("modelFit", {}).get("over_provisioned") is False]

    total = ra.get("total", 0)
    ok_runs = ra.get("succeeded", 0)
    fail_runs = ra.get("failed", 0)
    other_runs = ra.get("other", 0)
    fleet_fail_rate = fleet.get("fleetFailureRateDay")
    fail_rate_str = f"{fleet_fail_rate:.0%}" if fleet_fail_rate is not None else "n/a"

    lines: list[str] = []

    # ----- Header -----
    lines += [
        f"# Fleet Morning Report — {report_date}",
        "",
        f"*Audit date: {audit_date} (latest available ledger entry) · Report generated: {report_date}*",
        "",
        "> **Cost disclaimer:** This fleet runs on `claude_local` *subscription* runs",
        "> (`costCents = 0` in Paperclip; no charges incurred). All cost figures below",
        "> are **estimated (subscription, token-derived)** using public Anthropic list",
        "> pricing. **These are not billed spend.** Treat as directional only.",
        "",
        "---",
        "",
        "## Summary",
        "",
        f"| Metric | Value |",
        f"|---|---|",
        f"| Fleet synthetic spend (day) | ${fleet.get('estCostUsdDay', 0):.2f} _(est., not billed)_ |",
        f"| Fleet runs (day) | {total} total ({ok_runs} ok / {fail_runs} failed / {other_runs} other) |",
        f"| Fleet fail rate (day) | {fail_rate_str} |",
        f"| Agents audited | {fleet.get('agentCount', 0)} |",
        f"| Fleet WoW token trend | {wow_fleet_str} |",
        f"| Drift flags | {len(drift_agents)} agent(s) |",
        f"| Model-fit flags (over-provisioned) | {len(over_agents)}/{fleet.get('agentCount', 0)} agent(s) |",
        "",
        "---",
        "",
        "## Per-Agent Usage *(costs estimated, subscription, token-derived — not billed)*",
        "",
        "| Agent | Role | Day est$ | 7d WoW tok% | 7d runs | Fail rate | Drift | Model-fit |",
        "|---|---|---|---|---|---|---|---|",
    ]

    for a in agents:
        day = a.get("day", {})
        tr = a.get("trend7d", {})
        rel = a.get("reliability", {})
        fit = a.get("modelFit", {})
        wow = (
            "n/a"
            if tr.get("wowPctChange") is None
            else f"{tr['wowPctChange']:+.1f}%"
        )
        fr = rel.get("failureRate")
        fr_str = "n/a" if fr is None else f"{fr:.0%}"
        drift_sym = "⚠️" if rel.get("driftFlag") else "—"
        ovp = fit.get("over_provisioned")
        if ovp is True:
            tiers = fit.get("tiers_above_target", "?")
            mf = f"⬇ → {fit.get('target_model')} ({tiers} tier{'s' if tiers != 1 else ''})"
        elif ovp is False:
            mf = "✅ ok"
        else:
            mf = "n/a"
        lines.append(
            f"| {a.get('name')} | {a.get('role')} | ${day.get('estCostUsd', 0):.2f} |"
            f" {wow} | {tr.get('currentRuns', 0)} | {fr_str} | {drift_sym} | {mf} |"
        )

    lines.append("")

    # ----- Drift flags -----
    if drift_agents:
        lines += ["---", "", "## ⚠️ Drift Flags", ""]
        for a in drift_agents:
            rel = a.get("reliability", {})
            fr = rel.get("failureRate")
            fr_str = f"{fr:.0%}" if fr is not None else "unknown"
            stale = rel.get("staleHeartbeatEvents", 0)
            status = rel.get("agentStatus", "unknown")
            lines.append(
                f"- **{a.get('name')}** ({a.get('role')}): {stale} stale-heartbeat event(s),"
                f" {fr_str} fail rate, status `{status}` _(low-confidence derived signal — no per-agent run.* telemetry)_"
            )
        lines.append("")

    # ----- Model-fit summary -----
    lines += ["---", "", "## Model-Fit Summary", ""]
    if over_agents:
        lines.append(
            f"**{len(over_agents)}/{len(agents)} agents** are over-provisioned vs the §3 model-assignment matrix."
        )
        if all(
            not a.get("modelFit", {}).get("configured_explicitly") for a in over_agents
        ):
            lines.append(
                "All are on `claude-opus-4-8` via the harness default (no `adapterConfig.model` set explicitly)."
            )
        lines.append("")
        lines += [
            "| Agent | Role | Current | Target | Tiers over | Escalation path |",
            "|---|---|---|---|---|---|",
        ]
        for a in over_agents:
            fit = a.get("modelFit", {})
            lines.append(
                f"| {a.get('name')} | {a.get('role')} | {fit.get('effective_model')} |"
                f" {fit.get('target_model')} | {fit.get('tiers_above_target')} |"
                f" {fit.get('escalation') or '—'} |"
            )
        lines.append("")
    if ok_agents:
        names = ", ".join(a.get("name", "") for a in ok_agents)
        lines += [f"**On-target agents ({len(ok_agents)}):** {names}", ""]

    # ----- Proposed optimizations -----
    lines += ["---", "", "## Proposed Optimizations", ""]

    lines += [
        "### Auto-approve-eligible",
        "*No drift, clean run history, not the CEO's own config.*",
        "",
    ]
    if auto_opts:
        for opt in auto_opts:
            tiers = opt.get("tiers_above", 0)
            lines.append(
                f"- **{opt['agent']}** ({opt['role']}): `{opt['current']}` → `{opt['target']}`"
                f" (over by {tiers} tier{'s' if tiers != 1 else ''}) — {opt['reason']}"
            )
    else:
        lines.append("_None — all over-provisioned agents have flags that require CEO review._")
    lines.append("")

    lines += [
        "### Escalate to CEO",
        "*Requires CEO decision before any config change.*",
        "",
    ]
    if escalate_opts:
        for opt in escalate_opts:
            tiers = opt.get("tiers_above", 0)
            lines.append(
                f"- **{opt['agent']}** ({opt['role']}): `{opt['current']}` → `{opt['target']}`"
                f" (over by {tiers} tier{'s' if tiers != 1 else ''}) — {opt['reason']}"
            )
    else:
        lines.append("_None._")
    lines.append("")

    # ----- Data notes -----
    lines += ["---", "", "## Data Notes", ""]
    for note in entry.get("dataNotes", []):
        lines.append(f"- {note}")
    lines += [
        "",
        "*Source: [VER-81](/VER/issues/VER-81) `audit-ledger` doc (Agent Auditor — VER-70 / 68a).*",
        "*Delivered by [VER-72](/VER/issues/VER-72) morning-report routine (68d).*",
    ]

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Delivery: create a CEO issue with the report
# ---------------------------------------------------------------------------

def deliver_report(company: str, report_md: str, report_date: str, dry_run: bool) -> str:
    """Create a 'Fleet Morning Report' issue assigned to the CEO."""
    if dry_run:
        print(report_md)
        return "(dry-run)"
    issue = api(
        "POST",
        f"/api/companies/{company}/issues",
        {
            "title": f"Fleet Morning Report — {report_date}",
            "description": report_md,
            "assigneeAgentId": CEO_AGENT_ID,
            "priority": "medium",
            "status": "todo",
        },
    )
    return (issue or {}).get("identifier", "unknown")


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> int:
    ap = argparse.ArgumentParser(description="CEO Morning Report generator (VER-72 / 68d)")
    ap.add_argument(
        "--ledger-issue-id",
        default=os.environ.get("AUDIT_LEDGER_ISSUE_ID", DEFAULT_LEDGER_ISSUE_ID),
    )
    ap.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the report to stdout; do not create a CEO issue",
    )
    args = ap.parse_args()

    for var in ("PAPERCLIP_API_URL", "PAPERCLIP_API_KEY", "PAPERCLIP_COMPANY_ID"):
        if not os.environ.get(var):
            print(f"ERROR: missing required env var {var}", file=sys.stderr)
            return 2

    company = os.environ["PAPERCLIP_COMPANY_ID"]
    report_date = dt.datetime.now(dt.timezone.utc).date().isoformat()

    entries = fetch_audit_entries(args.ledger_issue_id)
    if not entries:
        print(
            "ERROR: no audit entries found in ledger. "
            "Ensure agent_audit.py has run at least once.",
            file=sys.stderr,
        )
        return 1

    latest = entries[0]
    report_md = render_report(latest, report_date, CEO_AGENT_ID)
    identifier = deliver_report(company, report_md, report_date, args.dry_run)

    if not args.dry_run:
        print(
            f"Morning report delivered: issue {identifier} assigned to CEO. "
            f"Audit date: {latest['auditDate']}. "
            f"Fleet est$ (day): ${latest['fleet'].get('estCostUsdDay', 0):.2f} "
            f"(estimated, subscription, token-derived — not billed)."
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
