#!/usr/bin/env python3
"""
CEO Morning Report (VER-72 / 68d) — daily briefing from the audit ledger.

Reads the `audit-ledger` document on the ledger host issue (VER-81), synthesises
the last available audit entry, computes week-over-week token trends, synthetic
cost summary, model-fit flags, drift alerts, and a split optimisation proposal
(auto-approve-eligible vs escalate-to-CEO), then:

  1. Upserts a `morning-report` document on the report host issue (VER-99, Engineer-owned).
  2. Creates a daily `CEO Morning Brief {date}` issue assigned to CEO for inbox delivery.

READ-ONLY with respect to agent configuration — never writes adapterConfig.

Data source: the `audit-ledger` document on the ledger host issue (VER-81),
written daily by the Agent Auditor routine (VER-70 / 68a). JSON blocks are
embedded inside <!-- audit-entry:YYYY-MM-DD --> … <!-- /audit-entry --> sections.

All costs labelled "estimated (subscription, token-derived)" — NEVER billed spend.

Usage:
  python3 scripts/ceo_morning_report.py [--date YYYY-MM-DD] [--dry-run]
                                        [--ledger-issue-id ID]
                                        [--report-issue-id ID]

  --date       : report date (defaults to most-recent entry in the ledger)
  --dry-run    : print the report; do not write or comment
  --ledger-issue-id  : override ledger host (default: VER-81)
  --report-issue-id  : override report host (default: VER-99, Engineer-owned)
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

# ── Default issue IDs ──────────────────────────────────────────────────────────
DEFAULT_LEDGER_ISSUE_ID = "3baf2078-b210-4ff8-b9b9-7ecac0f05fe3"   # VER-81 (audit ledger)
DEFAULT_REPORT_ISSUE_ID = "52dc23d9-87c6-470f-80e1-7a6ee9fec98b"   # VER-104 (report archive, Engineer-owned)
CEO_AGENT_ID = "520599f1-fb06-4df0-9dc1-83994a2f19ac"
VER68_PARENT_ID = "29da461a-0c48-4011-bcbd-33c0ce89e29d"           # VER-68 parent
AUDIT_LEDGER_KEY = "audit-ledger"
MORNING_REPORT_KEY = "morning-report"

# ── Optimization classification thresholds ────────────────────────────────────
# Agents whose role is in this set require CEO escalation for model changes.
ESCALATE_ROLES = {"ceo", "cto"}
# Agents 2+ tiers over target on a "low-risk" role are auto-approve candidates.
AUTO_APPROVE_TIERS_THRESHOLD = 2

# ── Ledger parsing ─────────────────────────────────────────────────────────────
ENTRY_RE = re.compile(
    r"<!-- audit-entry:(\d{4}-\d{2}-\d{2}) -->.*?<!-- /audit-entry:\1 -->",
    re.DOTALL,
)
JSON_BLOCK_RE = re.compile(r"```json\s*(\{.*?\})\s*```", re.DOTALL)


def parse_ledger_entries(body: str) -> dict[str, dict]:
    """Return {date_str: entry_dict} parsed from markdown audit-ledger body."""
    entries: dict[str, dict] = {}
    for m in ENTRY_RE.finditer(body):
        date_str = m.group(1)
        section = m.group(0)
        jm = JSON_BLOCK_RE.search(section)
        if jm:
            try:
                entries[date_str] = json.loads(jm.group(1))
            except json.JSONDecodeError:
                pass  # skip malformed blocks
    return entries


# ── Paperclip API helper ───────────────────────────────────────────────────────

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


# ── Optimization proposal logic ────────────────────────────────────────────────

def classify_optimization(agent: dict) -> tuple[str, str] | None:
    """Return (category, rationale) or None if no optimization is warranted.

    category: "auto_approve" | "escalate_ceo"
    """
    fit = agent.get("modelFit", {})
    role = agent.get("role", "unknown")
    name = agent.get("name", agent.get("agentId", "?"))
    if not fit.get("over_provisioned"):
        return None
    tiers_above = fit.get("tiers_above_target", 0) or 0
    target = fit.get("target_model", "unknown")
    effective = fit.get("effective_model", "unknown")
    explicitly_configured = fit.get("configured_explicitly", False)

    rationale = (
        f"{name} ({role}) is on {effective} but target is {target} "
        f"({tiers_above} tier(s) above). "
    )
    if explicitly_configured:
        rationale += "Model was explicitly configured — review before changing."
    else:
        rationale += "Running on harness default; no explicit config to override."

    if role in ESCALATE_ROLES:
        return (
            "escalate_ceo",
            rationale + " Strategic role: CEO approval required before any model change.",
        )
    if tiers_above >= AUTO_APPROVE_TIERS_THRESHOLD and not explicitly_configured:
        return (
            "auto_approve",
            rationale + " Low-risk: non-strategic role, harness default, 2+ tiers over target.",
        )
    # 1-tier over, or explicitly configured non-strategic role → escalate as a precaution
    return (
        "escalate_ceo",
        rationale + " Borderline: 1-tier gap or explicit config — escalate for confirmation.",
    )


# ── Report rendering ───────────────────────────────────────────────────────────

REPORT_HEADER = """# CEO Morning Report

Daily agent fleet briefing generated by the CEO Morning Report routine ([VER-72](/VER/issues/VER-72)).
Audit data sourced from the Agent Audit Ledger ([VER-81](/VER/issues/VER-81)).

> **Important:** All cost figures below are *estimated (subscription, token-derived)* —
> this fleet runs on a subscription plan (billed cost = $0). Figures are synthetic and
> directional only. They are **never billed spend**.

Newest report first.

"""

REPORT_ENTRY_BEGIN = "<!-- report-entry:{date} -->"
REPORT_ENTRY_END = "<!-- /report-entry:{date} -->"

ENTRY_PARSE_RE = re.compile(
    r"<!-- report-entry:(\d{4}-\d{2}-\d{2}) -->.*?<!-- /report-entry:\1 -->",
    re.DOTALL,
)


def render_report_entry(entry: dict, all_entries: dict[str, dict]) -> str:
    """Render one morning report entry as markdown."""
    date = entry["auditDate"]
    fleet = entry["fleet"]
    per_agent = entry.get("perAgent", [])

    # --- Week-over-week token trend (latest entry vs prior-7d window in entry) ---
    wow_rows = []
    for a in per_agent:
        tr = a.get("trend7d", {})
        cur = tr.get("currentTotalTokens", 0)
        wow_pct = tr.get("wowPctChange")
        wow_str = "n/a (first week)" if wow_pct is None else f"{wow_pct:+.1f}%"
        wow_rows.append((a["name"], a.get("role", "?"), cur, wow_str))
    # Sort by current-window tokens desc
    wow_rows.sort(key=lambda r: r[2], reverse=True)

    # --- Model-fit flags ---
    drift_agents = [a["name"] for a in per_agent if a.get("reliability", {}).get("driftFlag")]
    over_prov = [
        (a["name"], a["modelFit"]["tiers_above_target"], a["modelFit"]["target_model"])
        for a in per_agent
        if a.get("modelFit", {}).get("over_provisioned")
    ]

    # --- Optimization proposals ---
    auto_approve = []
    escalate = []
    for a in per_agent:
        result = classify_optimization(a)
        if result:
            cat, rationale = result
            if cat == "auto_approve":
                auto_approve.append((a["name"], rationale))
            else:
                escalate.append((a["name"], rationale))

    lines = []
    lines.append(REPORT_ENTRY_BEGIN.format(date=date))
    lines.append(f"## Morning Report {date}")
    lines.append("")
    lines.append("### Fleet Overview")
    lines.append("")
    ra = fleet.get("runActivityDay", {})
    total_runs = ra.get("total", 0)
    ok_runs = ra.get("succeeded", 0)
    fail_runs = ra.get("failed", 0)
    fleet_fail_pct = (
        f"{fleet.get('fleetFailureRateDay', 0) * 100:.1f}%"
        if fleet.get("fleetFailureRateDay") is not None
        else "n/a"
    )
    lines.append(
        f"- **Synthetic fleet spend (day):** ${fleet['estCostUsdDay']:.4f} "
        f"_(estimated, subscription token-derived — not billed)_"
    )
    lines.append(
        f"- **Fleet runs (day):** {total_runs} total — "
        f"{ok_runs} succeeded / {fail_runs} failed (failure rate: {fleet_fail_pct})"
    )
    lines.append(f"- **Agents audited:** {fleet.get('agentCount', len(per_agent))}")
    lines.append("")

    # Token + WoW trend table
    lines.append("### Token Usage & Week-over-Week Trend")
    lines.append("")
    lines.append(
        "> WoW% compares the 7-day window ending on the audit date against the "
        "prior 7-day window. `n/a (first week)` = no prior baseline yet."
    )
    lines.append("")
    lines.append("| Agent | Role | 7d tokens | WoW% |")
    lines.append("|---|---|---|---|")
    for name, role, cur_tok, wow_str in wow_rows:
        lines.append(f"| {name} | {role} | {cur_tok:,} | {wow_str} |")
    lines.append("")

    # Drift flags
    lines.append("### Drift & Reliability Flags")
    lines.append("")
    if drift_agents:
        for name in drift_agents:
            lines.append(f"- ⚠️ **{name}** — stale-heartbeat events or error status detected")
    else:
        lines.append("- No drift flags on this audit date.")
    lines.append("")

    # Model-fit flags
    lines.append("### Model-Fit Flags")
    lines.append("")
    if over_prov:
        for name, tiers, target in over_prov:
            lines.append(f"- **{name}**: {tiers} tier(s) over target (recommend → {target})")
    else:
        lines.append("- No over-provisioned agents.")
    lines.append("")

    # Optimization proposals
    lines.append("### Proposed Optimisations")
    lines.append("")
    lines.append("#### Auto-approve-eligible")
    lines.append("")
    lines.append(
        "> These agents are non-strategic roles on the harness default (no explicit config), "
        "2+ tiers over target. Eng Manager may apply without CEO sign-off."
    )
    lines.append("")
    if auto_approve:
        for name, rationale in auto_approve:
            lines.append(f"- **{name}**: {rationale}")
    else:
        lines.append("- None.")
    lines.append("")
    lines.append("#### Escalate to CEO")
    lines.append("")
    lines.append(
        "> These require your explicit approval: strategic roles, explicitly-configured "
        "models, or borderline cases."
    )
    lines.append("")
    if escalate:
        for name, rationale in escalate:
            lines.append(f"- **{name}**: {rationale}")
    else:
        lines.append("- None.")
    lines.append("")

    # Data notes
    notes = entry.get("dataNotes", [])
    if notes:
        lines.append("<details><summary>Data quality notes</summary>")
        lines.append("")
        for note in notes:
            lines.append(f"- {note}")
        lines.append("")
        lines.append("</details>")
        lines.append("")

    lines.append(REPORT_ENTRY_END.format(date=date))
    return "\n".join(lines)


def splice_report(existing_body: str | None, entry_md: str, date: str) -> str:
    """Insert/replace the dated report entry, newest first. Idempotent per date."""
    entries: dict[str, str] = {}
    if existing_body:
        for m in ENTRY_PARSE_RE.finditer(existing_body):
            entries[m.group(1)] = m.group(0)
    entries[date] = entry_md
    ordered = [entries[d] for d in sorted(entries, reverse=True)]
    return REPORT_HEADER + "\n".join(ordered) + "\n"


# ── Comment helper ─────────────────────────────────────────────────────────────

def build_ceo_brief(entry: dict, company_id: str) -> tuple[str, str]:
    """Return (issue_title, issue_description) for a daily CEO brief issue."""
    date = entry["auditDate"]
    fleet = entry["fleet"]
    per_agent = entry.get("perAgent", [])

    drift_agents = [a["name"] for a in per_agent if a.get("reliability", {}).get("driftFlag")]
    over_prov_count = sum(1 for a in per_agent if a.get("modelFit", {}).get("over_provisioned"))
    auto_approve_count = 0
    escalate_count = 0
    for a in per_agent:
        result = classify_optimization(a)
        if result:
            if result[0] == "auto_approve":
                auto_approve_count += 1
            else:
                escalate_count += 1

    ra = fleet.get("runActivityDay", {})
    fail_pct = (
        f"{fleet.get('fleetFailureRateDay', 0) * 100:.1f}%"
        if fleet.get("fleetFailureRateDay") is not None
        else "n/a"
    )
    drift_str = (
        f"⚠️ Drift flags: {', '.join(drift_agents)}"
        if drift_agents
        else "✅ No drift flags"
    )

    title = f"CEO Morning Brief {date}"
    desc_lines = [
        f"## Morning Brief {date}",
        "",
        "> All costs are *estimated (subscription, token-derived)* — not billed spend.",
        "",
        "### Headlines",
        "",
        f"- **Fleet synthetic spend:** ${fleet['estCostUsdDay']:.4f} _(estimated, subscription token-derived — not billed)_",
        f"- **Fleet runs:** {ra.get('total', 0)} total "
        f"({ra.get('succeeded', 0)} ok / {ra.get('failed', 0)} failed — {fail_pct} fail rate)",
        f"- **Model-fit:** {over_prov_count} agent(s) over-provisioned",
        f"- {drift_str}",
        f"- **Proposed optimisations:** {auto_approve_count} auto-approve-eligible, "
        f"{escalate_count} escalate-to-CEO",
        "",
        "### Links",
        "",
        "- Full report: [VER-104](/VER/issues/VER-104#document-morning-report)",
        "- Audit ledger: [VER-81](/VER/issues/VER-81#document-audit-ledger)",
        "- Source: [VER-72](/VER/issues/VER-72)",
        "",
        "_This issue is auto-created daily by the CEO Morning Report routine. Mark it done once reviewed._",
    ]
    return title, "\n".join(desc_lines)


# ── Main ───────────────────────────────────────────────────────────────────────

def main() -> int:
    ap = argparse.ArgumentParser(
        description="CEO Morning Report generator (VER-72 / 68d)"
    )
    ap.add_argument(
        "--date",
        help="Report date YYYY-MM-DD (default: most-recent ledger entry)",
    )
    ap.add_argument(
        "--ledger-issue-id",
        default=os.environ.get("AUDIT_LEDGER_ISSUE_ID", DEFAULT_LEDGER_ISSUE_ID),
    )
    ap.add_argument(
        "--report-issue-id",
        default=os.environ.get("REPORT_ISSUE_ID", DEFAULT_REPORT_ISSUE_ID),
    )
    ap.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the report and comment; do not write to Paperclip",
    )
    args = ap.parse_args()

    for var in ("PAPERCLIP_API_URL", "PAPERCLIP_API_KEY", "PAPERCLIP_COMPANY_ID"):
        if not os.environ.get(var):
            print(f"ERROR: missing required env var {var}", file=sys.stderr)
            return 2

    # 1. Read the audit ledger
    try:
        doc = api("GET", f"/api/issues/{args.ledger_issue_id}/documents/{AUDIT_LEDGER_KEY}")
    except RuntimeError as e:
        print(f"ERROR: could not read audit ledger: {e}", file=sys.stderr)
        return 1

    if not doc or not doc.get("body"):
        print("ERROR: audit ledger is empty — nothing to report.", file=sys.stderr)
        return 1

    entries = parse_ledger_entries(doc["body"])
    if not entries:
        print("ERROR: no parseable entries in audit ledger.", file=sys.stderr)
        return 1

    # Pick target date
    if args.date:
        target_date = args.date
        if target_date not in entries:
            print(
                f"ERROR: no ledger entry for {target_date}. "
                f"Available: {sorted(entries, reverse=True)}",
                file=sys.stderr,
            )
            return 1
    else:
        target_date = sorted(entries, reverse=True)[0]

    entry = entries[target_date]

    # 2. Render the report entry
    entry_md = render_report_entry(entry, entries)

    if args.dry_run:
        print("=== REPORT ENTRY ===")
        print(entry_md)
        print("=== CEO BRIEF ISSUE ===")
        company = os.environ["PAPERCLIP_COMPANY_ID"]
        brief_title, brief_desc = build_ceo_brief(entry, company)
        print(f"Title: {brief_title}")
        print(brief_desc)
        return 0

    # 4. Upsert the morning-report document
    base_rev = None
    existing_body = None
    try:
        existing_doc = api(
            "GET",
            f"/api/issues/{args.report_issue_id}/documents/{MORNING_REPORT_KEY}",
        )
        if existing_doc:
            existing_body = existing_doc.get("body")
            base_rev = existing_doc.get("latestRevisionId")
    except RuntimeError as e:
        if "HTTP 404" not in str(e):
            raise

    new_body = splice_report(existing_body, entry_md, target_date)
    api(
        "PUT",
        f"/api/issues/{args.report_issue_id}/documents/{MORNING_REPORT_KEY}",
        {
            "title": "CEO Morning Report",
            "format": "markdown",
            "body": new_body,
            "baseRevisionId": base_rev,
        },
    )
    print(f"Morning report updated: issue {args.report_issue_id} doc '{MORNING_REPORT_KEY}' (date {target_date}).")

    # 5. Create a daily CEO brief issue (assigned to CEO) for delivery
    company = os.environ["PAPERCLIP_COMPANY_ID"]
    brief_title, brief_desc = build_ceo_brief(entry, company)
    brief = api(
        "POST",
        f"/api/companies/{company}/issues",
        {
            "title": brief_title,
            "description": brief_desc,
            "status": "todo",
            "priority": "medium",
            "assigneeAgentId": CEO_AGENT_ID,
            "parentId": VER68_PARENT_ID,
        },
    )
    brief_id = brief.get("identifier", "?") if brief else "?"
    print(f"CEO brief issue created: {brief_id}.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
