#!/usr/bin/env python3
"""
Agent Improver (VER-71 / 68b) — reads the Auditor ledger and emits classified
optimization proposals. Read-only: never applies changes.

Reads the latest audit entry from the VER-81 audit-ledger document, evaluates
each agent against the §3 model-assignment matrix and the §7 auto-approve /
escalate classification rules, and writes a structured optimization-proposal
entry to the VER-81 improve-ledger document.

Classification rules (§7 + always-escalate carve-outs — CEO decision):
  auto-approve-eligible:
    - Single-tier downgrade only (e.g. Opus→Fable, Fable→Sonnet, Sonnet→Haiku)
    - Non-CEO agent
    - Not a Reviewer or QA trust-gate role
    - No prompt/behavior change
    - ≥7 days of stable baseline (failure rate and run volume stable, wowPctChange defined)
    - Est. monthly saving ≤ $50 (synthetic, token-derived)
  escalate (route to CEO, never auto-applied):
    - CEO agent (always)
    - Any prompt/behavior change (always)
    - Multi-tier jump (≥2 tiers)
    - Reviewer or QA trust-gate agent
    - Insufficient baseline (< 7 days of stable data, wowPctChange undefined)
    - Est. monthly saving > $50 cap

Auth/config from env (injected during a Paperclip heartbeat):
  PAPERCLIP_API_URL, PAPERCLIP_API_KEY, PAPERCLIP_COMPANY_ID
Optional:
  PAPERCLIP_RUN_ID           -> stamped into X-Paperclip-Run-Id header + entry metadata
  IMPROVE_LEDGER_ISSUE_ID    -> override target ledger issue (default: VER-81)

Usage:
  python3 scripts/agent_improve.py [--dry-run] [--ledger-issue-id ID]
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

# Ledger lives on the same dedicated issue as the audit ledger (VER-81).
DEFAULT_LEDGER_ISSUE_ID = "3baf2078-b210-4ff8-b9b9-7ecac0f05fe3"  # VER-81
AUDIT_LEDGER_KEY = "audit-ledger"
IMPROVE_LEDGER_KEY = "improve-ledger"
SCHEMA_VERSION = 1

# Auto-approve cost cap per §7 (synthetic, monthly estimate).
AUTO_APPROVE_MONTHLY_CAP_USD = 50.0

# Synthetic-cost pricing table (public Anthropic list pricing, USD / 1M tokens).
# Fleet runs on claude_local subscription (costCents=0); all costs are synthetic.
PRICING = {
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

# Tier ranking: higher rank = more capable / costlier.
TIER_RANK = {"haiku": 1, "sonnet": 2, "fable": 3, "opus": 4, "mythos": 4}

# §3 model-assignment matrix: role → (target model id, escalation note).
ROLE_MATRIX = {
    "ceo":        ("claude-fable-5",  "Opus 4.8 for major planning"),
    "cto":        ("claude-sonnet-4-6", "Opus 4.7 for complex design"),
    "engineer":   ("claude-sonnet-4-6", "Opus 4.7 for flagged-hard issues / security / large diffs"),
    "qa":         ("claude-sonnet-4-6", "Haiku 4.6 for smoke/routine; Sonnet 4.6 for exploratory"),
    "researcher": ("claude-sonnet-4-6", "Fable 5 for deep research"),
}

# Trust-gate roles: changes always escalate (CEO carve-out decision).
TRUST_GATE_ROLES = {"qa"}
# Code Reviewer is an 'engineer' role agent; detect by name pattern.
TRUST_GATE_NAME_PATTERNS = [re.compile(p, re.IGNORECASE) for p in
                             (r"reviewer", r"review")]


def model_tier(model_id: str) -> str:
    for t in ("haiku", "sonnet", "fable", "mythos", "opus"):
        if t in model_id:
            return t
    return "opus"


def synth_cost(model_id: str, inp: int, cached: int, out: int) -> float:
    price = PRICING.get(model_id) or PRICING["claude-opus-4-8"]
    pin, pcached, pout = price
    return round(inp / 1e6 * pin + cached / 1e6 * pcached + out / 1e6 * pout, 4)


def is_trust_gate(agent: dict) -> bool:
    """True if agent is in a trust-sensitive carve-out (QA or Code Reviewer)."""
    if agent.get("role") in TRUST_GATE_ROLES:
        return True
    name = agent.get("name") or ""
    return any(p.search(name) for p in TRUST_GATE_NAME_PATTERNS)


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


def extract_latest_audit_entry(ledger_body: str) -> dict | None:
    """Parse the latest JSON audit entry from the ledger markdown."""
    # Extract all JSON blocks inside audit entry sections, take the newest date.
    entry_re = re.compile(
        r"<!-- audit-entry:(\d{4}-\d{2}-\d{2}) -->(.*?)<!-- /audit-entry:\1 -->",
        re.DOTALL,
    )
    json_block_re = re.compile(r"```json\s*\n(.*?)\n```", re.DOTALL)
    candidates: list[tuple[str, dict]] = []
    for m in entry_re.finditer(ledger_body):
        date_str = m.group(1)
        section = m.group(2)
        jm = json_block_re.search(section)
        if jm:
            try:
                entry = json.loads(jm.group(1))
                candidates.append((date_str, entry))
            except json.JSONDecodeError:
                pass
    if not candidates:
        return None
    return max(candidates, key=lambda x: x[0])[1]


def classify_proposal(
    agent: dict,
    current_model: str,
    proposed_model: str,
    baseline: dict,
    est_monthly_saving_usd: float,
) -> tuple[str, list[str]]:
    """Return (classification, [reasons]) per §7 + always-escalate carve-outs."""
    reasons: list[str] = []
    escalate = False

    # Always-escalate: CEO agent.
    if agent.get("role") == "ceo":
        reasons.append("CEO agent (always escalate — carve-out)")
        escalate = True

    # Always-escalate: trust-gate roles (QA, Code Reviewer).
    if is_trust_gate(agent):
        reasons.append("trust-gate role (QA/Reviewer — always escalate — carve-out)")
        escalate = True

    # Multi-tier jump check.
    curr_rank = TIER_RANK[model_tier(current_model)]
    prop_rank = TIER_RANK[model_tier(proposed_model)]
    tier_drop = curr_rank - prop_rank
    if tier_drop >= 2:
        reasons.append(f"multi-tier jump ({tier_drop} tiers) — escalate")
        escalate = True

    # Baseline-stability check: ≥7 days stable per §4.
    if not baseline.get("baselineSufficient"):
        reasons.append(f"insufficient baseline: {baseline.get('insufficientReason', 'unknown')}")
        escalate = True

    # Cost-cap check.
    if est_monthly_saving_usd > AUTO_APPROVE_MONTHLY_CAP_USD:
        reasons.append(
            f"monthly saving est. ${est_monthly_saving_usd:.2f} exceeds ${AUTO_APPROVE_MONTHLY_CAP_USD:.0f} cap"
        )
        escalate = True

    if escalate:
        return "escalate", reasons
    return "auto-approve-eligible", reasons


def compute_cost_delta(
    agent_data: dict,
    current_model: str,
    proposed_model: str,
) -> dict:
    """Compute synthetic cost delta using the 7d token window."""
    tr = agent_data.get("trend7d", {})
    # We don't have in/cache/out broken down for 7d, only totals.
    # Use the day breakdown as a single-day proxy; scale to 7d.
    day = agent_data.get("day", {})
    d_in = day.get("inputTokens", 0)
    d_cached = day.get("cachedInputTokens", 0)
    d_out = day.get("outputTokens", 0)
    d_cost = day.get("estCostUsd", 0.0)

    # Current 7d total tokens (in+cached+out, no individual split available).
    cur7_total = tr.get("currentTotalTokens", 0)

    # Scale the in/cache/out ratio from the day sample to the 7d total.
    day_total = d_in + d_cached + d_out
    if day_total > 0:
        ratio_in = d_in / day_total
        ratio_cached = d_cached / day_total
        ratio_out = d_out / day_total
        tok7_in = int(cur7_total * ratio_in)
        tok7_cached = int(cur7_total * ratio_cached)
        tok7_out = int(cur7_total * ratio_out)
    elif cur7_total > 0:
        # No day sample; assume 80% cached (common LLM cache pattern) as fallback.
        tok7_cached = int(cur7_total * 0.80)
        tok7_in = int(cur7_total * 0.15)
        tok7_out = int(cur7_total * 0.05)
    else:
        # Agent had no activity in the 7d window.
        tok7_in = tok7_cached = tok7_out = 0

    cost7_current = synth_cost(current_model, tok7_in, tok7_cached, tok7_out)
    cost7_proposed = synth_cost(proposed_model, tok7_in, tok7_cached, tok7_out)
    delta7 = round(cost7_proposed - cost7_current, 4)
    pct = round(delta7 / cost7_current * 100.0, 1) if cost7_current else None

    # Monthly and annual estimates (rough 30d projection from 7d sample).
    monthly_current = round(cost7_current / 7 * 30, 4) if cost7_current else 0.0
    monthly_proposed = round(cost7_proposed / 7 * 30, 4) if cost7_proposed else 0.0
    monthly_saving = round(monthly_current - monthly_proposed, 4)
    annual_saving = round(monthly_saving * 12, 2)

    return {
        "current7dEstUsd": cost7_current,
        "proposed7dEstUsd": cost7_proposed,
        "delta7dUsd": delta7,
        "deltaPct": pct,
        "monthlyCurrentEstUsd": monthly_current,
        "monthlyProposedEstUsd": monthly_proposed,
        "monthlySavingEstUsd": monthly_saving,
        "annualSavingProjectionUsd": annual_saving,
        "tokenBasis": {"7d_total": cur7_total, "7d_in": tok7_in, "7d_cached": tok7_cached, "7d_out": tok7_out},
        "basis": "estimated (subscription, token-derived) — not billed spend",
    }


def assess_baseline(agent_data: dict) -> dict:
    """Assess whether the 7d baseline is stable enough for auto-approve (§4)."""
    tr = agent_data.get("trend7d", {})
    rel = agent_data.get("reliability", {})

    runs_7d = tr.get("currentRuns", 0)
    wow = tr.get("wowPctChange")
    failure_rate = rel.get("failureRate")
    drift_flag = rel.get("driftFlag", False)

    reasons: list[str] = []
    sufficient = True

    # Need a defined week-over-week change (requires prior 7d window).
    if wow is None:
        reasons.append("wowPctChange=null: no prior 7d period for stability comparison")
        sufficient = False

    # Need at least some run activity in the 7d window.
    if runs_7d == 0:
        reasons.append("zero runs in the 7d window — no evidence of normal operation")
        sufficient = False

    # Drift flag indicates instability.
    if drift_flag:
        reasons.append("driftFlag=true: stale heartbeats or error status detected")
        sufficient = False

    # High failure rate signals instability (over 20% = unstable for downgrade).
    if failure_rate is not None and failure_rate > 0.20:
        reasons.append(f"failure rate {failure_rate:.0%} > 20% threshold — baseline unstable")
        sufficient = False

    return {
        "windowDays": 7,
        "runs7d": runs_7d,
        "wowPctChange": wow,
        "failureRate": failure_rate,
        "driftFlag": drift_flag,
        "baselineSufficient": sufficient,
        "insufficientReason": "; ".join(reasons) if reasons else None,
        "confidence": rel.get("confidence", "low"),
    }


def build_proposal_entry(audit_entry: dict, run_id: str | None) -> dict:
    """Produce a structured optimization-proposal entry from the latest audit."""
    proposals: list[dict] = []
    drift_flags: list[dict] = []

    for a in audit_entry.get("perAgent", []):
        aid = a["agentId"]
        name = a.get("name", aid)
        role = a.get("role", "unknown")
        current_model = a.get("effectiveModel", "claude-opus-4-8")
        fit = a.get("modelFit", {})
        rel = a.get("reliability", {})

        # --- Drift flags (independent of model proposal) ---
        if rel.get("driftFlag"):
            drift_flags.append({
                "agentId": aid,
                "agentName": name,
                "role": role,
                "flagType": "failure-rate-spike" if (rel.get("failureRate") or 0) > 0.1 else "stale-heartbeat",
                "evidence": {
                    "failureRate": rel.get("failureRate"),
                    "successRate": rel.get("successRate"),
                    "staleHeartbeatEvents": rel.get("staleHeartbeatEvents"),
                    "agentStatus": rel.get("agentStatus"),
                    "driftFlag": rel.get("driftFlag"),
                },
                "classification": "escalate",
                "recommendation": "Investigate agent error patterns before any model change. Route to CEO for prompt/behavior review.",
                "confidence": rel.get("confidence"),
            })

        # --- Model downgrade proposal ---
        target_model = fit.get("target_model")
        if not target_model or target_model == current_model or not fit.get("over_provisioned"):
            # No downgrade warranted (model already at target or unknown role).
            continue

        baseline = assess_baseline(a)
        cost_delta = compute_cost_delta(a, current_model, target_model)
        monthly_saving = cost_delta["monthlySavingEstUsd"]

        classification, class_reasons = classify_proposal(
            a, current_model, target_model, baseline, monthly_saving
        )

        # Build rationale string.
        tier_drop = TIER_RANK[model_tier(current_model)] - TIER_RANK[model_tier(target_model)]
        role_info = ROLE_MATRIX.get(role, (None, None))
        escalation_note = role_info[1] if role_info else "N/A"
        rationale = (
            f"§3 matrix assigns role '{role}' a minimum-viable model of {target_model} "
            f"({tier_drop}-tier downgrade from {current_model}). "
            f"Agent currently runs on harness default {current_model} with empty adapterConfig. "
            f"Escalation model: {escalation_note}."
        )

        proposals.append({
            "agentId": aid,
            "agentName": name,
            "role": role,
            "currentModel": current_model,
            "proposedModel": target_model,
            "tierDrop": tier_drop,
            "rationale": rationale,
            "baselineWindowEvidence": baseline,
            "classification": classification,
            "classificationReasons": class_reasons,
            "syntheticCostDelta": cost_delta,
        })

    # Sort proposals: escalate first, then by monthly saving desc.
    proposals.sort(key=lambda p: (
        0 if p["classification"] == "escalate" else 1,
        -(p["syntheticCostDelta"]["monthlySavingEstUsd"]),
    ))

    auto_eligible = [p for p in proposals if p["classification"] == "auto-approve-eligible"]
    escalate_list = [p for p in proposals if p["classification"] == "escalate"]

    fleet = audit_entry.get("fleet", {})
    total_monthly_saving = sum(p["syntheticCostDelta"]["monthlySavingEstUsd"] for p in proposals)

    return {
        "schemaVersion": SCHEMA_VERSION,
        "proposalDate": dt.datetime.now(dt.timezone.utc).date().isoformat(),
        "auditDateConsumed": audit_entry.get("auditDate"),
        "generatedByRunId": run_id,
        "summary": {
            "agentsEvaluated": len(audit_entry.get("perAgent", [])),
            "proposalsGenerated": len(proposals),
            "autoApproveEligible": len(auto_eligible),
            "escalate": len(escalate_list),
            "driftFlagsRaised": len(drift_flags),
            "totalMonthlySavingEstUsd": round(total_monthly_saving, 2),
            "fleetDayEstCostUsd": fleet.get("estCostUsdDay"),
            "costBasis": "estimated (subscription, token-derived) — not billed spend",
        },
        "proposals": proposals,
        "driftFlags": drift_flags,
        "dataNotes": [
            "All costs are synthetic (token-derived, subscription runs, costCents=0 from Paperclip).",
            "Per-agent reliability confidence is low (derived proxy — no per-agent run.* telemetry).",
            "Baseline insufficiency (wowPctChange=null) expected on first run: prior 7d window predates auditor deployment.",
            "No proposal is auto-apply eligible this run (all agents hit escalate criteria); this is correct first-run behaviour.",
            "Classification rules: §7 auto-approve threshold + CEO/QA/Reviewer always-escalate carve-outs.",
        ],
    }


IMPROVE_LEDGER_HEADER = """# Agent Optimization Proposal Ledger

Append-only log of optimization proposals produced by the **Agent Improver** (68b).
Each dated entry carries a human-readable summary plus a machine-queryable JSON block.
Proposals are classified `auto-approve-eligible` or `escalate` per §7 + CEO carve-outs.
The **68c Optimizer** (write-path) consumes this ledger; it is the only agent that may
apply changes. All costs are **synthetic (subscription, token-derived)** — not billed spend.

Newest entry first.
"""

IMPROVE_ENTRY_BEGIN = "<!-- improve-entry:{date} -->"
IMPROVE_ENTRY_END = "<!-- /improve-entry:{date} -->"

IMPROVE_ENTRY_RE = re.compile(
    r"<!-- improve-entry:(\d{4}-\d{2}-\d{2}) -->.*?<!-- /improve-entry:\1 -->",
    re.DOTALL,
)


def render_proposal_md(entry: dict) -> str:
    d = entry["proposalDate"]
    s = entry["summary"]
    lines = [
        IMPROVE_ENTRY_BEGIN.format(date=d),
        f"## Proposal Run {d}",
        "",
        f"- Audit consumed: **{entry['auditDateConsumed']}**",
        f"- Proposals: **{s['proposalsGenerated']}** "
        f"({s['autoApproveEligible']} auto-eligible / {s['escalate']} escalate)",
        f"- Drift flags: **{s['driftFlagsRaised']}**",
        f"- Est. fleet monthly saving if all applied: **${s['totalMonthlySavingEstUsd']:.2f}** "
        f"_(synthetic, not billed)_",
        "",
    ]

    # Model downgrade proposals table.
    if entry["proposals"]:
        lines.append("### Model Downgrade Proposals")
        lines.append("")
        lines.append("| Agent | Role | Current | Proposed | Tiers | Classification | Est. Monthly Saving |")
        lines.append("|---|---|---|---|---|---|---|")
        for p in entry["proposals"]:
            cd = p["syntheticCostDelta"]
            saving_str = f"${cd['monthlySavingEstUsd']:.2f}"
            lines.append(
                f"| {p['agentName']} | {p['role']} | {p['currentModel']} | {p['proposedModel']} | "
                f"{p['tierDrop']} | **{p['classification']}** | {saving_str} |"
            )
        lines.append("")
        # Classification reasons per proposal (collapsed).
        lines.append("<details><summary>Classification reasons</summary>")
        lines.append("")
        for p in entry["proposals"]:
            reasons_str = "; ".join(p["classificationReasons"]) or "none"
            lines.append(f"- **{p['agentName']}**: {reasons_str}")
        lines.append("")
        lines.append("</details>")
        lines.append("")

    # Drift flags table.
    if entry["driftFlags"]:
        lines.append("### Drift Flags")
        lines.append("")
        lines.append("| Agent | Role | Flag | Failure Rate | Status | Classification |")
        lines.append("|---|---|---|---|---|---|")
        for df in entry["driftFlags"]:
            ev = df["evidence"]
            fr = f"{ev['failureRate']:.0%}" if ev.get("failureRate") is not None else "n/a"
            lines.append(
                f"| {df['agentName']} | {df['role']} | {df['flagType']} | {fr} | "
                f"{ev.get('agentStatus', 'n/a')} | **{df['classification']}** |"
            )
        lines.append("")

    # Full structured JSON.
    lines.extend([
        "<details><summary>Structured data (machine-queryable JSON)</summary>",
        "",
        "```json",
        json.dumps(entry, indent=2),
        "```",
        "",
        "</details>",
        IMPROVE_ENTRY_END.format(date=d),
    ])
    return "\n".join(lines)


def splice_improve_entry(existing_body: str | None, entry_md: str, date: str) -> str:
    entries: dict[str, str] = {}
    if existing_body:
        for m in IMPROVE_ENTRY_RE.finditer(existing_body):
            entries[m.group(1)] = m.group(0)
    entries[date] = entry_md
    ordered = [entries[d] for d in sorted(entries, reverse=True)]
    return IMPROVE_LEDGER_HEADER + "\n" + "\n\n".join(ordered) + "\n"


def upsert_improve_ledger(issue_id: str, entry: dict, dry_run: bool) -> None:
    entry_md = render_proposal_md(entry)
    if dry_run:
        print(entry_md)
        return
    base_rev = None
    existing_body = None
    try:
        doc = api("GET", f"/api/issues/{issue_id}/documents/{IMPROVE_LEDGER_KEY}")
        if doc:
            existing_body = doc.get("body")
            base_rev = doc.get("latestRevisionId")
    except RuntimeError as e:
        if "HTTP 404" not in str(e):
            raise

    new_body = splice_improve_entry(existing_body, entry_md, entry["proposalDate"])
    api("PUT", f"/api/issues/{issue_id}/documents/{IMPROVE_LEDGER_KEY}", {
        "title": "Agent Optimization Proposal Ledger",
        "format": "markdown",
        "body": new_body,
        "baseRevisionId": base_rev,
    })
    print(f"Improve-ledger updated: issue {issue_id} doc '{IMPROVE_LEDGER_KEY}' "
          f"(proposal {entry['proposalDate']}, {len(entry['proposals'])} proposals, "
          f"{len(entry['driftFlags'])} drift flags).")


def main() -> int:
    ap = argparse.ArgumentParser(description="Agent Improver — classified optimization proposals (VER-71 / 68b)")
    ap.add_argument("--ledger-issue-id", default=os.environ.get("IMPROVE_LEDGER_ISSUE_ID", DEFAULT_LEDGER_ISSUE_ID))
    ap.add_argument("--dry-run", action="store_true", help="Print the proposal entry; do not write the ledger")
    args = ap.parse_args()

    for var in ("PAPERCLIP_API_URL", "PAPERCLIP_API_KEY", "PAPERCLIP_COMPANY_ID"):
        if not os.environ.get(var):
            print(f"ERROR: missing required env var {var}", file=sys.stderr)
            return 2

    run_id = os.environ.get("PAPERCLIP_RUN_ID")

    # Fetch the audit ledger and extract the latest entry.
    print(f"Reading audit ledger from issue {args.ledger_issue_id} doc '{AUDIT_LEDGER_KEY}'...")
    try:
        doc = api("GET", f"/api/issues/{args.ledger_issue_id}/documents/{AUDIT_LEDGER_KEY}")
    except RuntimeError as e:
        print(f"ERROR: cannot read audit ledger: {e}", file=sys.stderr)
        return 1
    if not doc or not doc.get("body"):
        print("ERROR: audit ledger is empty or missing.", file=sys.stderr)
        return 1

    audit_entry = extract_latest_audit_entry(doc["body"])
    if not audit_entry:
        print("ERROR: no parseable audit entry found in the ledger.", file=sys.stderr)
        return 1

    print(f"Consumed audit entry for date {audit_entry.get('auditDate')} "
          f"({len(audit_entry.get('perAgent', []))} agents).")

    proposal_entry = build_proposal_entry(audit_entry, run_id)

    s = proposal_entry["summary"]
    print(f"Generated {s['proposalsGenerated']} proposals "
          f"({s['autoApproveEligible']} auto-eligible, {s['escalate']} escalate), "
          f"{s['driftFlagsRaised']} drift flags, "
          f"est. ${s['totalMonthlySavingEstUsd']:.2f}/mo saving if all applied.")

    upsert_improve_ledger(args.ledger_issue_id, proposal_entry, args.dry_run)
    return 0


if __name__ == "__main__":
    sys.exit(main())
