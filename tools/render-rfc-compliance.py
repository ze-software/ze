#!/usr/bin/env -S uv run python3
"""Render the RFC compliance gate page from ../main RFC requirement data."""

import collections
import html
import importlib.util
import json
import pathlib
import re
import sys

import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN = GH_PAGES.parent / "main"
GENERATOR = MAIN / "scripts" / "dev" / "rfc_requirements.py"
STATUS_LEDGER = MAIN / "docs" / "features" / "rfc-status.md"
HOOK = MAIN / ".claude" / "hooks" / "pretool-writeedit.py"
VERIFY_RUN = MAIN / "scripts" / "status" / "verify_run.go"
MAKEFILE = MAIN / "Makefile"
DEST = GH_PAGES / "quality" / "rfc-compliance" / "index.html"
SNAPSHOT = GH_PAGES / "data" / "rfc-compliance.json"

SATISFACTION_LABELS = {
    "both_polarities": {
        "label": "Positive and negative tests",
        "short": "Test pair",
        "condition": "positive tag + negative tag",
    },
    "single_polarity": {
        "label": "One polarity plus reason",
        "short": "Single polarity",
        "condition": "{single-polarity} annotation + required tag",
    },
    "not_applicable": {
        "label": "Not applicable",
        "short": "Not applicable",
        "condition": "{not-applicable} annotation",
    },
    "gap": {
        "label": "Declared gap",
        "short": "Gap",
        "condition": "{gap} annotation + public ledger disclosure",
    },
    "one_polarity_unexcused": {
        "label": "One polarity, unexcused",
        "short": "Unexcused one side",
        "condition": "tag without annotation",
    },
    "missing_unexcused": {
        "label": "Missing, unexcused",
        "short": "Missing",
        "condition": "no tag, no annotation",
    },
}

SATISFACTION_ORDER = [
    "both_polarities",
    "single_polarity",
    "not_applicable",
    "gap",
    "one_polarity_unexcused",
    "missing_unexcused",
]

STATUS_ORDER = ["Partial", "Experimental", "Supported", "Not supported", "Unsupported"]


def esc(value):
    return html.escape(str(value), quote=True)


def fmt_int(value):
    return f"{int(value):,}"


def pct(count, total):
    return (100.0 * count / total) if total else 0.0


def fmt_pct(count, total):
    return f"{pct(count, total):.1f}%"


def _load_module():
    if not GENERATOR.exists():
        raise RuntimeError(f"missing {GENERATOR}")
    spec = importlib.util.spec_from_file_location("ze_site_rfc_requirements", GENERATOR)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


def _check_groups(module, enrolled, reqs, parse_errs, tags, parse_by_stem, status_rows):
    stems = module.summary_stems()
    baseline_enrolled = module._git_baseline_enrolment()
    baseline_ids = module._git_baseline_ids()
    baseline_stems = module._git_baseline_summary_stems()

    groups = []

    def add(name, description, errors):
        groups.append(
            {
                "name": name,
                "errors": list(errors),
                "error_count": len(errors),
            }
        )

    add(
        "Enrolment ratchet",
        "rfc/enrolled.txt can grow, and it cannot name a summary that does not exist.",
        module.check_enrolment(enrolled, baseline_enrolled, stems),
    )
    add(
        "New summary ratchet",
        "Adding an RFC summary must add checking instead of creating silent backlog.",
        module.check_new_summaries(stems, baseline_stems, enrolled, reqs, parse_by_stem),
    )
    if enrolled & baseline_enrolled:
        add(
            "Retired requirement ratchet",
            "An enrolled requirement ID cannot vanish from the current summary.",
            module.check_retired_requirements(
                reqs,
                enrolled,
                baseline_ids,
                baseline_enrolled,
                stems,
                baseline_stems,
                parse_by_stem,
            ),
        )
        add(
            "Coverage polarity ratchet",
            "A requirement proven at HEAD cannot lose a tagged polarity.",
            module.check_coverage_ratchet(
                reqs,
                tags,
                enrolled,
                module._git_baseline_tag_polarities(),
                baseline_enrolled,
            ),
        )
    add("Summary parse", "Malformed enrolled checklist lines fail closed.", parse_errs)
    add(
        "Requirement ID allocation",
        "Requirement IDs are section anchored and cannot be reused for different text.",
        module.check_id_allocation(reqs, baseline_ids),
    )
    add(
        "Requirement coverage",
        "Each enrolled MUST-level requirement needs both test polarities or an annotation.",
        module.evaluate(reqs, tags, enrolled),
    )
    add(
        "Public claim agreement",
        "A {gap} cannot hide behind a clean Supported row in docs/features/rfc-status.md.",
        module.check_status_agreement(reqs, status_rows, enrolled),
    )
    add(
        "Semantic audit freshness",
        "Recorded reader verdicts fail when the requirement text or tagged test changed.",
        module.check_audit_freshness(reqs, tags, enrolled),
    )
    add(
        "Generated ledger freshness",
        "ai/RFC-REQUIREMENTS.md must match the current summaries and test tags.",
        module.check_ledger_fresh(reqs, tags, enrolled),
    )
    return groups


def _audit_counts(module, gated_reqs, tags, enrolled):
    by_rid = collections.defaultdict(list)
    for tag in tags:
        by_rid[tag.rid].append(tag)
    audits = {stem: module.load_audit(stem) for stem in sorted(enrolled)}
    verdicts = fresh = stale = 0
    for req in gated_reqs:
        verdict = audits.get(req.rfc, {}).get(req.rid)
        if not verdict:
            continue
        verdicts += 1
        if module.verdict_is_fresh(
            verdict,
            module.requirement_sha(req.text),
            module.tagged_unit_shas(by_rid.get(req.rid, [])),
        ):
            fresh += 1
        else:
            stale += 1
    return {
        "verdicts": verdicts,
        "fresh": fresh,
        "stale": stale,
        "missing": max(len(gated_reqs) - verdicts, 0),
    }


def _agent_guard():
    hook_text = HOOK.read_text(encoding="utf-8", errors="replace") if HOOK.exists() else ""
    make_text = MAKEFILE.read_text(encoding="utf-8", errors="replace") if MAKEFILE.exists() else ""
    verify_text = VERIFY_RUN.read_text(encoding="utf-8", errors="replace") if VERIFY_RUN.exists() else ""
    return {
        "hook_path": ".claude/hooks/pretool-writeedit.py",
        "hook_present": HOOK.exists(),
        "blocks_unapproved": bool(
            "_rfc_tagged_change_err" in hook_text
            and "BLOCKED: RFC-tagged test" in hook_text
            and "rfc-test-change-approved" in hook_text
        ),
        "approval_token": "rfc-test-change-approved",
        "make_target_present": bool(re.search(r"^ze-rfc-check:", make_text, re.M)),
        "verify_stage_mentions": verify_text.count('mk("ze-rfc-check")'),
    }


def collect_from_main():
    module = _load_module()
    enrolled, reqs, parse_errs, tags, parse_by_stem = module._collect_for_check()
    status_rows = module.parse_status_ledger(STATUS_LEDGER.read_text(encoding="utf-8"))
    check_groups = _check_groups(module, enrolled, reqs, parse_errs, tags, parse_by_stem, status_rows)

    gated_reqs = [req for req in reqs if req.gated and req.rfc in enrolled]
    by_rid = collections.defaultdict(set)
    for tag in tags:
        by_rid[tag.rid].add(tag.polarity)

    satisfaction_counts = collections.Counter()
    gap_counts = collections.Counter()
    gap_rfcs = set()
    for req in gated_reqs:
        ann = req.annotation
        polarities = by_rid.get(req.rid, set())
        if ann and ann.kind == "not-applicable":
            bucket = "not_applicable"
        elif ann and ann.kind == "gap":
            bucket = "gap"
            gap_counts[req.rfc] += 1
            gap_rfcs.add(req.rfc)
        elif ann and ann.kind == "single-polarity":
            bucket = "single_polarity"
        elif polarities == module.POLARITIES:
            bucket = "both_polarities"
        elif polarities:
            bucket = "one_polarity_unexcused"
        else:
            bucket = "missing_unexcused"
        satisfaction_counts[bucket] += 1

    satisfaction = []
    for key in SATISFACTION_ORDER:
        count = satisfaction_counts.get(key, 0)
        if count == 0 and key not in ("one_polarity_unexcused", "missing_unexcused"):
            continue
        entry = dict(SATISFACTION_LABELS[key])
        entry.update({"key": key, "count": count, "share": round(pct(count, len(gated_reqs)), 1)})
        satisfaction.append(entry)

    gap_status_counts = collections.Counter(
        status_rows.get(stem, {}).get("status", "Missing public row") for stem in gap_rfcs
    )
    gap_status = []
    for status in STATUS_ORDER:
        if gap_status_counts.get(status, 0):
            gap_status.append({"status": status, "count": gap_status_counts[status]})
    for status, count in sorted(gap_status_counts.items()):
        if status not in STATUS_ORDER:
            gap_status.append({"status": status, "count": count})

    supported_with_remaining = []
    for stem in sorted(gap_rfcs):
        row = status_rows.get(stem, {})
        if not row.get("status", "").startswith("Supported"):
            continue
        supported_with_remaining.append(
            {
                "rfc": stem.upper().replace("RFC", "RFC ", 1),
                "remaining": row.get("remaining", "").strip(),
            }
        )

    top_gap_rfcs = []
    for stem, count in gap_counts.most_common(12):
        row = status_rows.get(stem, {})
        top_gap_rfcs.append(
            {
                "rfc": stem.upper().replace("RFC", "RFC ", 1),
                "count": count,
                "status": row.get("status", "Missing public row"),
            }
        )

    gate_error_count = sum(group["error_count"] for group in check_groups)
    return {
        "gate": {
            "ok": gate_error_count == 0,
            "error_count": gate_error_count,
            "gated_must": len(gated_reqs),
            "enrolled_rfcs": len(enrolled),
            "test_tags": len(tags),
            "message": (
                "rfc-requirements OK: %d gated MUST-level requirement(s) across %d enrolled RFC(s); %d test tag(s) resolved."
                % (len(gated_reqs), len(enrolled), len(tags))
            ),
        },
        "satisfaction": satisfaction,
        "gaps": {
            "requirements": satisfaction_counts.get("gap", 0),
            "rfcs": len(gap_rfcs),
            "status_counts": gap_status,
            "supported_with_remaining": supported_with_remaining,
            "top_rfcs": top_gap_rfcs,
        },
        "audit": _audit_counts(module, gated_reqs, tags, enrolled),
        "checks": check_groups,
        "agent_guard": _agent_guard(),
        "sources": {
            "requirements": "rfc/short/*.md",
            "enrolment": "rfc/enrolled.txt",
            "test_tags": "internal/, pkg/, test/",
            "status_ledger": "docs/features/rfc-status.md",
            "audit_verdicts": "rfc/audit/*.json",
            "gate_script": "scripts/dev/rfc_requirements.py",
        },
    }


def load():
    try:
        snapshot = collect_from_main()
    except Exception as exc:
        if SNAPSHOT.exists():
            sitelib.warn(
                "rfc-compliance: could not derive current RFC gate data (%s); using data/rfc-compliance.json"
                % exc
            )
            return json.loads(SNAPSHOT.read_text(encoding="utf-8")), False
        sitelib.warn("rfc-compliance: could not derive RFC gate data (%s)" % exc)
        return None, False
    SNAPSHOT.parent.mkdir(parents=True, exist_ok=True)
    SNAPSHOT.write_text(json.dumps(snapshot, indent=2, ensure_ascii=False) + "\n")
    return snapshot, True


def render_cards(snapshot):
    gate = snapshot["gate"]
    gaps = snapshot["gaps"]
    audit = snapshot["audit"]
    guard = snapshot["agent_guard"]
    cards = [
        (
            "Gate verdict",
            "OK" if gate["ok"] else "RED",
            "%s open gate issue%s" % (fmt_int(gate["error_count"]), "" if gate["error_count"] == 1 else "s"),
            "ok" if gate["ok"] else "bad",
        ),
        (
            "Gated MUSTs",
            fmt_int(gate["gated_must"]),
            "%s enrolled RFCs, %s resolved test tags" % (fmt_int(gate["enrolled_rfcs"]), fmt_int(gate["test_tags"])),
            "info",
        ),
        (
            "Declared gaps",
            fmt_int(gaps["requirements"]),
            "Across %s RFCs, all forced into the public ledger" % fmt_int(gaps["rfcs"]),
            "warn",
        ),
        (
            "AI test guard",
            "ON" if guard["blocks_unapproved"] else "OFF",
            "%s verify stage mentions, approval token required" % fmt_int(guard["verify_stage_mentions"]),
            "ok" if guard["blocks_unapproved"] else "bad",
        ),
        (
            "Semantic verdicts",
            fmt_int(audit["fresh"]),
            "%s stale, %s missing and therefore not claimed" % (fmt_int(audit["stale"]), fmt_int(audit["missing"])),
            "ok" if audit["stale"] == 0 else "bad",
        ),
    ]
    out = ['<div class="rfc-card-grid reveal">']
    for label, value, note, state in cards:
        out.append(
            '<article class="rfc-card rfc-%s"><span>%s</span><strong>%s</strong><p>%s</p></article>'
            % (esc(state), esc(label), esc(value), esc(note))
        )
    out.append("</div>")
    return "\n".join(out)


def render_satisfaction(snapshot):
    total = snapshot["gate"]["gated_must"]
    rows = [row for row in snapshot["satisfaction"] if row["count"]]
    tape = ['<div class="rfc-tape" role="img" aria-label="RFC requirement satisfaction split">']
    for row in rows:
        tape.append(
            '<span class="rfc-tape-%s" style="--w: %.3f%%"><b>%s</b><em>%s</em></span>'
            % (esc(row["key"]), pct(row["count"], total), esc(row["short"]), esc(fmt_int(row["count"])))
        )
    tape.append("</div>")

    table = [
        "<table>",
        "<thead><tr><th>Bucket</th><th>Count</th><th>Share</th><th>Source condition</th></tr></thead>",
        "<tbody>",
    ]
    for row in rows:
        table.append(
            "<tr><td>%s</td><td><strong>%s</strong></td><td>%s</td><td><code>%s</code></td></tr>"
            % (esc(row["label"]), esc(fmt_int(row["count"])), esc(fmt_pct(row["count"], total)), esc(row["condition"]))
        )
    table.extend(["</tbody>", "</table>"])
    return "\n".join(tape + table)


def render_status(snapshot):
    gaps = snapshot["gaps"]
    out = [
        "<table>",
        "<thead><tr><th>Public status for RFCs with gaps</th><th>RFCs</th></tr></thead>",
        "<tbody>",
    ]
    for row in gaps["status_counts"]:
        out.append(
            "<tr><td>%s</td><td><strong>%s</strong></td></tr>"
            % (esc(row["status"]), esc(fmt_int(row["count"])))
        )
    out.extend(["</tbody>", "</table>"])
    if gaps["supported_with_remaining"]:
        out.append('<div class="rfc-note-box">')
        out.append("<h3>Supported rows that still disclose a gap</h3>")
        out.append("<ul>")
        for row in gaps["supported_with_remaining"]:
            out.append("<li><strong>%s</strong>: %s</li>" % (esc(row["rfc"]), esc(row["remaining"])))
        out.append("</ul></div>")
    return "\n".join(out)


def render_layers(snapshot):
    gate = snapshot["gate"]
    gaps = snapshot["gaps"]
    audit = snapshot["audit"]
    guard = snapshot["agent_guard"]
    supported_count = sum(
        row["count"] for row in gaps["status_counts"] if row["status"].startswith("Supported")
    )
    layers = [
        ("Requirement source", "rfc/short/*.md", "%s gated MUST-level requirements" % fmt_int(gate["gated_must"])),
        ("Enrollment", "rfc/enrolled.txt", "%s enrolled RFCs" % fmt_int(gate["enrolled_rfcs"])),
        ("Test tags", "internal/, pkg/, test/", "%s resolved tags" % fmt_int(gate["test_tags"])),
        ("Public ledger", "docs/features/rfc-status.md", "%s RFCs with gaps, %s Supported with Remaining" % (fmt_int(gaps["rfcs"]), fmt_int(supported_count))),
        ("Semantic audits", "rfc/audit/*.json", "%s fresh, %s stale, %s missing" % (fmt_int(audit["fresh"]), fmt_int(audit["stale"]), fmt_int(audit["missing"]))),
        ("AI write/edit guard", guard["hook_path"], "ON" if guard["blocks_unapproved"] else "OFF"),
        ("Verify integration", "Makefile + scripts/status/verify_run.go", "%s verify stages, make target %s" % (fmt_int(guard["verify_stage_mentions"]), "present" if guard["make_target_present"] else "missing")),
    ]
    out = [
        "<table>",
        "<thead><tr><th>Input</th><th>Producer</th><th>Observed value</th></tr></thead>",
        "<tbody>",
    ]
    for layer, producer, observed in layers:
        out.append(
            "<tr><td>%s</td><td><code>%s</code></td><td>%s</td></tr>"
            % (esc(layer), esc(producer), esc(observed))
        )
    out.extend(["</tbody>", "</table>"])
    return "\n".join(out)


def render_check_table(snapshot):
    out = [
        "<table>",
        "<thead><tr><th>Check</th><th>Open issues</th></tr></thead>",
        "<tbody>",
    ]
    for group in snapshot["checks"]:
        cls = "rfc-check-ok" if group["error_count"] == 0 else "rfc-check-bad"
        out.append(
            '<tr class="%s"><td>%s</td><td><strong>%s</strong></td></tr>'
            % (esc(cls), esc(group["name"]), esc(fmt_int(group["error_count"])))
        )
    out.extend(["</tbody>", "</table>"])
    return "\n".join(out)


def render_top_gap_rfcs(snapshot):
    rows = snapshot["gaps"]["top_rfcs"]
    if not rows:
        return ""
    out = [
        "<table>",
        "<thead><tr><th>RFC</th><th>Declared gaps</th><th>Public status</th></tr></thead>",
        "<tbody>",
    ]
    for row in rows:
        out.append(
            "<tr><td><code>%s</code></td><td><strong>%s</strong></td><td>%s</td></tr>"
            % (esc(row["rfc"]), esc(fmt_int(row["count"])), esc(row["status"]))
        )
    out.extend(["</tbody>", "</table>"])
    return "\n".join(out)


STYLE = """
<style>
.rfc-card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr)); gap: 1rem; margin: 1.25rem 0 1.6rem; }
.rfc-card { border-radius: 18px; padding: 1rem 1.1rem; background: var(--panel-strong); border: 1px solid var(--line); box-shadow: 0 1rem 2rem -1.6rem var(--shadow); }
.rfc-card span { display: block; color: var(--muted); font-size: .78rem; font-weight: 800; letter-spacing: .06em; text-transform: uppercase; }
.rfc-card strong { display: block; margin: .3rem 0; font-size: clamp(1.75rem, 4vw, 2.55rem); line-height: 1; color: var(--text); }
.rfc-card p { margin: 0; color: var(--muted); font-size: .92rem; }
.rfc-ok { border-left: 7px solid var(--teal-base); }
.rfc-info { border-left: 7px solid var(--sky-base); }
.rfc-warn { border-left: 7px solid var(--gold-base); }
.rfc-bad { border-left: 7px solid var(--danger-deep); }
.rfc-tape { display: flex; min-height: 3.4rem; margin: 1rem 0 1.2rem; overflow: hidden; border-radius: 999px; border: 1px solid var(--line-strong); background: var(--panel); box-shadow: inset 0 0 0 1px rgba(255,255,255,.75); }
.rfc-tape span { width: var(--w); min-width: 5.2rem; display: flex; flex-direction: column; justify-content: center; gap: .12rem; padding: .45rem .8rem; color: #241431; }
.rfc-tape b { font-size: .82rem; line-height: 1; }
.rfc-tape em { font-style: normal; font-size: .78rem; opacity: .78; }
.rfc-tape-both_polarities { background: var(--teal-chip); }
.rfc-tape-single_polarity { background: var(--sky-chip); }
.rfc-tape-not_applicable { background: var(--grape-chip); }
.rfc-tape-gap { background: var(--gold-chip); }
.rfc-note-box { margin: 1rem 0; padding: 1rem 1.2rem; border-radius: 16px; background: var(--gold-tint); border: 1px solid var(--gold-chip); }
.rfc-note-box h3 { margin-top: 0; }
.rfc-note-box ul { margin-bottom: 0; }
.rfc-check-ok strong { color: var(--teal-deep); }
.rfc-check-bad strong { color: var(--danger-deep); }
.rfc-command { padding: .85rem 1rem; border-radius: 14px; background: var(--term-bg); color: var(--term-text); overflow-x: auto; }
.rfc-command code { color: var(--term-text); }
@media (max-width: 760px) {
  .rfc-tape { flex-direction: column; border-radius: 18px; }
  .rfc-tape span { width: 100% !important; }
}
</style>
"""


def render(snapshot):
    root = "../../"
    title = "RFC Compliance Gate Report - Ze"
    desc = "Generated RFC gate report from requirement summaries, test tags, status ledger, audits, and agent guard state."
    out = [
        sitelib.page_head(title, desc, root, og_title=title, og_desc=desc, page_key="quality/rfc-compliance/")
    ]
    out.append('            <section aria-labelledby="rfc-compliance-title" class="md-content reveal cat-observe">')
    out.append(
        sitelib.page_hero(
            "RFC Compliance Gate Report",
            "Source: <code>scripts/dev/rfc_requirements.py</code>, <code>rfc/short/*.md</code>, <code>docs/features/rfc-status.md</code>, <code>rfc/audit/*.json</code>, and <code>.claude/hooks/pretool-writeedit.py</code>.",
            "Quality",
            h1_id="rfc-compliance-title",
            lead_html=True,
        )
    )
    out.append(STYLE)
    out.append(render_cards(snapshot))
    out.append('<div class="section-note reveal"><p><strong>Current gate output:</strong></p><pre class="rfc-command"><code>%s</code></pre></div>' % esc(snapshot["gate"]["message"]))

    out.append("<section><h2>Requirement buckets</h2>")
    out.append(render_satisfaction(snapshot))
    out.append("</section>")

    out.append("<section><h2>Gap disclosure</h2>")
    out.append(render_status(snapshot))
    out.append("</section>")

    out.append("<section><h2>Top gap clusters</h2>")
    out.append(render_top_gap_rfcs(snapshot))
    out.append("</section>")

    out.append("<section><h2>AI guard and gate inputs</h2>")
    out.append(render_layers(snapshot))
    out.append("</section>")

    out.append("<section><h2>Check results</h2>")
    out.append(
        "<p>Generated artifacts: <a href=\"../../data/rfc-compliance.json\"><code>data/rfc-compliance.json</code></a>, <code>quality/rfc-compliance/index.html</code>, and <code>quality/rfc-compliance/index.md</code>.</p>"
    )
    out.append(render_check_table(snapshot))
    out.append("</section>")
    out.append("            </section>")

    body = "\n".join(out)
    DEST.parent.mkdir(parents=True, exist_ok=True)
    DEST.write_text(body + "\n" + sitelib.page_foot(root))
    sitelib.write_markdown_sibling(DEST, render_markdown(snapshot))
    print(
        "rendered RFC compliance gate: %s gated MUSTs, %s tags, %s gaps -> %s (+ index.md, data/rfc-compliance.json)"
        % (
            fmt_int(snapshot["gate"]["gated_must"]),
            fmt_int(snapshot["gate"]["test_tags"]),
            fmt_int(snapshot["gaps"]["requirements"]),
            DEST,
        )
    )


def render_markdown(snapshot):
    gate = snapshot["gate"]
    gaps = snapshot["gaps"]
    audit = snapshot["audit"]
    lines = [
        "# RFC Compliance Gate Report",
        "",
        "Source: `scripts/dev/rfc_requirements.py`, `rfc/short/*.md`, `docs/features/rfc-status.md`, `rfc/audit/*.json`, and `.claude/hooks/pretool-writeedit.py`.",
        "",
        "## Current gate output",
        "",
        "```",
        gate["message"],
        "```",
        "",
        "| Metric | Value |",
        "|---|---:|",
        "| Gate issues | %s |" % fmt_int(gate["error_count"]),
        "| Gated MUST-level requirements | %s |" % fmt_int(gate["gated_must"]),
        "| Enrolled RFCs | %s |" % fmt_int(gate["enrolled_rfcs"]),
        "| Resolved test tags | %s |" % fmt_int(gate["test_tags"]),
        "| Declared gaps | %s |" % fmt_int(gaps["requirements"]),
        "| RFCs with declared gaps | %s |" % fmt_int(gaps["rfcs"]),
        "| Fresh semantic audit verdicts | %s |" % fmt_int(audit["fresh"]),
        "| Stale semantic audit verdicts | %s |" % fmt_int(audit["stale"]),
        "",
        "## Requirement buckets",
        "",
        "| Bucket | Count | Share | Source condition |",
        "|---|---:|---:|---|",
    ]
    for row in [item for item in snapshot["satisfaction"] if item["count"]]:
        lines.append(
            "| %s | %s | %s | `%s` |"
            % (row["label"], fmt_int(row["count"]), fmt_pct(row["count"], gate["gated_must"]), row["condition"])
        )
    lines.extend(
        [
            "",
            "## Gap disclosure",
            "",
            "| Public status for RFCs with gaps | RFCs |",
            "|---|---:|",
        ]
    )
    for row in gaps["status_counts"]:
        lines.append("| %s | %s |" % (row["status"], fmt_int(row["count"])))
    if gaps["supported_with_remaining"]:
        lines.extend(["", "### Supported rows that still disclose a gap", ""])
        for row in gaps["supported_with_remaining"]:
            lines.append("- **%s:** %s" % (row["rfc"], row["remaining"]))
    lines.extend(
        [
            "",
            "## Top gap clusters",
            "",
            "| RFC | Declared gaps | Public status |",
            "|---|---:|---|",
        ]
    )
    for row in gaps["top_rfcs"]:
        lines.append("| `%s` | %s | %s |" % (row["rfc"], fmt_int(row["count"]), row["status"]))
    lines.extend(
        [
            "",
            "## AI guard and gate inputs",
            "",
            "| Input | Producer | Observed value |",
            "|---|---|---|",
            "| Requirement source | `rfc/short/*.md` | %s gated MUST-level requirements |" % fmt_int(gate["gated_must"]),
            "| Enrollment | `rfc/enrolled.txt` | %s enrolled RFCs |" % fmt_int(gate["enrolled_rfcs"]),
            "| Test tags | `internal/, pkg/, test/` | %s resolved tags |" % fmt_int(gate["test_tags"]),
            "| Public ledger | `docs/features/rfc-status.md` | %s RFCs with gaps |" % fmt_int(gaps["rfcs"]),
            "| Semantic audits | `rfc/audit/*.json` | %s fresh, %s stale, %s missing |" % (fmt_int(audit["fresh"]), fmt_int(audit["stale"]), fmt_int(audit["missing"])),
            "| AI write/edit guard | `.claude/hooks/pretool-writeedit.py` | %s |" % ("ON" if snapshot["agent_guard"]["blocks_unapproved"] else "OFF"),
            "| Verify integration | `Makefile` and `scripts/status/verify_run.go` | %s verify stages |" % fmt_int(snapshot["agent_guard"]["verify_stage_mentions"]),
            "",
            "## Check results",
            "",
            "| Check | Open issues |",
            "|---|---:|",
        ]
    )
    for group in snapshot["checks"]:
        lines.append("| %s | %s |" % (group["name"], fmt_int(group["error_count"])))
    lines.append("")
    return "\n".join(lines)


def main():
    snapshot, fresh = load()
    if snapshot is None:
        return 1
    render(snapshot)
    return 0 if snapshot.get("gate", {}).get("ok") else 1


if __name__ == "__main__":
    raise SystemExit(main())
