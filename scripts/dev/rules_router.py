#!/usr/bin/env python3
"""Report which rules a `**When:**` trigger index would surface, and which it drops.

`plan/spec-knowledge-3-rule-digest.md` replaces the eagerly-imported 97-section
digest with a trigger index plus a small always-on core. That trades tokens for
one risk: a blocking rule whose trigger never matches the work in hand stops
reaching the session that needed it (R-1). This report measures the trade
against real past work before the import is switched.

## The corpus

Past task descriptions, drawn from the two places this repo keeps them:

  plan/learned/*.md   `## Context` -- what problem a closed spec existed to solve
  plan/spec-*.md      `## Task`    -- the same section on specs still open

Both are the author's own statement of the work, written before the rules were
consulted, which is exactly the input a router would see.

## What "missed" means -- two readings, and the one that governs

Read literally, the digest carries all 97 rules for EVERY task, so any rule a
task does not surface is "missed" and the answer is a list the size of the rule
set. That reading measures nothing.

The reading that governs: a blocking rule is MISSED when NO task in the corpus
surfaces it. Such a rule would go dark across the whole corpus, and it is the
population the always-on core exists to protect. Rules already in the core are
excluded, because a core rule is never routed and so can never be missed by
routing.

## Scoring

A rule is surfaced for a task when the task text contains at least MIN_HITS of
the trigger's content words, counting only words that are DISTINCTIVE across the
97 triggers. "code", "test" and "any" appear in dozens of triggers and separate
nothing; "gokrazy", "nlri" and "qdisc" separate a great deal. The tokenizer is
`rules_condensed.significant_terms`, shared with the core derivation, so a rule
cannot be judged routable in one place and unroutable in the other.

This models a keyword reader, which is WEAKER than a model reading a trigger for
meaning. A miss here is therefore a floor on the risk, not a ceiling on it, and
that is the safe direction: the report over-reports misses rather than under-
reporting them.

Usage:
    python3 scripts/dev/rules_router.py            # text report
    python3 scripts/dev/rules_router.py --json     # machine-readable
    python3 scripts/dev/rules_router.py --verbose  # per-task surfaced rules

Exit: 0 for the measurement, which is something an operator reads and not a gate.
Non-zero only when the precedence ladder cannot be read, because the core this
report subtracts is derived from that ladder: an unreadable ladder makes every
number below wrong, so the report refuses rather than printing them.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import rules_condensed

# The scoring lives in `rules_condensed` because the core derivation depends on
# it: a rule no task surfaces is eager BECAUSE nothing surfaces it. Re-deriving
# it here would let this report and the core disagree about the same rule.
MAX_TRIGGER_DF = rules_condensed.MAX_TRIGGER_DF
MIN_HITS = rules_condensed.MIN_HITS
distinctive_terms = rules_condensed.distinctive_terms

SECTION = re.compile(r"^##\s+(.*)$")
# The section that states the work, whichever half of the lifecycle wrote it.
TASK_SECTIONS = ("context", "task")


def section_text(path, wanted=TASK_SECTIONS):
    """The named `##` section's prose, or "" when the document has none."""
    out, capturing = [], False
    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        m = SECTION.match(line)
        if m:
            capturing = m.group(1).strip().lower().split(" ")[0] in wanted
            continue
        if capturing:
            out.append(line)
    return "\n".join(out).strip()


def load_corpus(*dirs):
    """Task descriptions from every learned summary and open spec given."""
    corpus = []
    for d in dirs:
        d = Path(d)
        if not d.is_dir():
            continue
        for md in sorted(d.glob("*.md")):
            if md.stem.isupper() or md.name == "TEMPLATE.md":
                continue
            text = section_text(md)
            if text:
                corpus.append({"source": md.name, "text": text})
    return corpus


def build_report(rules, corpus):
    """Which rules each task surfaces, and which blocking rules nothing surfaces.

    The core is computed WITHOUT the corpus on purpose. Passing it would make
    every corpus-derived core member vanish from `missed-blocking`, and the
    report would print "MISSED: none" precisely because it had already been
    acted on -- a gauge wired to its own output.
    """
    core = {r["name"] for r in rules_condensed.core_members(rules)}
    terms = distinctive_terms(rules)
    routed = [r for r in rules if r["name"] not in core]

    tasks, surfaced_any = [], set()
    for entry in corpus:
        task_terms = rules_condensed.significant_terms(entry["text"])
        hits = []
        for rule in routed:
            overlap = terms[rule["name"]] & task_terms
            if len(overlap) >= MIN_HITS:
                hits.append(rule["name"])
        surfaced_any.update(hits)
        tasks.append(
            {
                "source": entry["source"],
                "surfaced": sorted(hits),
                "surfaced-blocking": sorted(
                    n for n in hits if _severity(rules, n) == "blocking"
                ),
            }
        )

    missed = sorted(
        r["name"]
        for r in routed
        if r["severity"] == "blocking" and r["name"] not in surfaced_any
    )
    blocking_routed = [r for r in routed if r["severity"] == "blocking"]
    return {
        "tasks": tasks,
        "corpus-size": len(corpus),
        "rules-total": len(rules),
        "core": sorted(core),
        "routed": len(routed),
        "blocking-routed": len(blocking_routed),
        "surfaced-any": sorted(surfaced_any),
        "missed-blocking": missed,
        "unroutable-terms": sorted(r["name"] for r in routed if not terms[r["name"]]),
    }


def _severity(rules, name):
    for rule in rules:
        if rule["name"] == name:
            return rule["severity"]
    return ""


def format_text(report, verbose=False):
    out = [
        f"corpus: {report['corpus-size']} past task descriptions",
        f"rules: {report['rules-total']} "
        f"({len(report['core'])} always-on core, {report['routed']} routed, "
        f"of which {report['blocking-routed']} blocking)",
        "",
    ]
    if verbose:
        for task in report["tasks"]:
            out.append(f"{task['source']}: {len(task['surfaced'])} rule(s) surfaced")
            for name in task["surfaced"]:
                out.append(f"    ai/rules/{name}")
        out.append("")

    per_task = [len(t["surfaced"]) for t in report["tasks"]] or [0]
    out.append(
        f"surfaced per task: min {min(per_task)}, max {max(per_task)}, "
        f"mean {sum(per_task) / len(per_task):.1f}"
    )
    out.append(
        f"blocking rules surfaced by at least one task: "
        f"{report['blocking-routed'] - len(report['missed-blocking'])} "
        f"of {report['blocking-routed']}"
    )
    out.append("")
    if report["missed-blocking"]:
        out.append(
            f"MISSED: {len(report['missed-blocking'])} blocking rule(s) no task in "
            "the corpus surfaces. Each is a candidate for the always-on core:"
        )
        for name in report["missed-blocking"]:
            out.append(f"    ai/rules/{name}")
    else:
        out.append("MISSED: none. Every routed blocking rule is surfaced by some task")
    if report["unroutable-terms"]:
        out.append("")
        out.append(
            "triggers with no distinctive term (they share every word with other "
            f"triggers): {len(report['unroutable-terms'])}"
        )
        for name in report["unroutable-terms"]:
            out.append(f"    ai/rules/{name}")
    return "\n".join(out)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--json", action="store_true", help="emit JSON instead of text")
    ap.add_argument("--verbose", action="store_true", help="list rules per task")
    ap.add_argument("--rules-dir", default=None)
    args = ap.parse_args(argv)

    root = Path(
        os.environ.get("CLAUDE_PROJECT_DIR") or Path(__file__).resolve().parents[2]
    )
    rules_dir = Path(args.rules_dir) if args.rules_dir else root / "ai" / "rules"
    rules = rules_condensed.load_rules(rules_dir)
    corpus = load_corpus(root / "plan" / "learned", root / "plan")
    try:
        report = build_report(rules, corpus)
    except rules_condensed.LadderError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    print(
        json.dumps(report, indent=2) if args.json else format_text(report, args.verbose)
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
