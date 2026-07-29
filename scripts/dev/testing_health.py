#!/usr/bin/env python3
"""Render the project's testing state as one generated Markdown page.

Design: plan/spec-test-health-dashboard.md

The page answers "is our testing correct?", not "is our testing large?". Those
are different questions, and volume metrics answer only the second: 20k test
functions tell you nothing about whether a regression would be caught. Every
metric here belongs to one of three questions; a candidate belonging to none
is excluded as volume:

    Q1 sensitivity   if the code were wrong, would something go red?
    Q2 intent        are the things that matter checked, or only the happy path?
    Q3 integrity     when something goes red, does it stop the line?

What is gated, and what is merely published (see structural_facts):

  * STRUCTURAL facts -- which test files no `go test` target builds, which
    enrolled RFCs have no test pair, and every metric's status -- are gated by
    --check. Each one changing is an event.
  * VOLUME counters are published, not gated. A byte-exact gate over the whole
    report charged a regenerate-and-commit to ~60% of commits, since every added
    test moves a denominator. A check firing that often for cosmetic reasons is
    routed around rather than read, which is the "advisory gate permanently red"
    failure this page exists to expose.

The ratchets do not rest on any of this: ze-test-sensitivity-check enforces them
from the tree, reading only the baseline, and is unaffected by report staleness.

Which files count is decided by GIT'S INDEX (see tracked_files), never by a
working-tree listing. Two honest limits on "committed": file CONTENTS are read
from the working tree, so an uncommitted edit to a tracked test moves the
counts; and `git ls-files` reads the index, so `git add` of a new test moves
them before any commit.

Metrics that need a live test run never go straight onto the page: --record
appends them to test/health/history.ndjson (committed) and the page renders
trends from there, mirroring scripts/dev/mutation_history.py.

Usage:
    scripts/dev/testing_health.py --write     # regenerate page + latest.json + baseline
    scripts/dev/testing_health.py --check     # fail if a STRUCTURAL fact drifted
    scripts/dev/testing_health.py --record    # append one KPI row to history.ndjson
    scripts/dev/testing_health.py --json      # emit the metric record on stdout
Called by: make ze-test-health, ze-test-health-check, ze-test-health-record
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path

PAGE = "docs/features/test-health.md"
LATEST = "test/health/latest.json"
HISTORY = "test/health/history.ndjson"
BASELINE = "test/health/sensitivity-baseline.json"
# Committed "best so far" for the higher-is-better ratio metrics. A metric warns
# only when it drops BELOW its locked-in best, so the attention table lists
# regressions, not a permanent gap to an arbitrary target. Contrast BASELINE,
# which ratchets DOWN. See quality_status / tighten_quality.
QUALITY_BASELINE = "test/health/quality-baseline.json"

# Metrics whose status is a regression signal against QUALITY_BASELINE, keyed to
# the percent they ratchet on. The number they compare is read from each
# metric's own data at tighten time.
QUALITY_METRICS = ("rfc-proof-density", "mutation", "negative-tests")

RFC_LEDGER = "ai/RFC-REQUIREMENTS.md"
RFC_SUMMARIES = "rfc/short"
MUTATION_HISTORY = "test/mutation/history.ndjson"
SLEEP_BASELINE = "test/.ci-sleep-baseline"

# In-repo test trees. vendor/ and gokrazy/modcache/ are third-party module
# trees; counting them is exactly the error that let the published unit-test
# total reach six times the real one (see the Honest inventory section).
TEST_ROOTS = ("internal", "cmd", "pkg", "scripts", "test")

# The RFC ledger's coverage table. Pinned exactly: it is generated, and a column
# change must fail loudly rather than silently yield zero (spec AC-17).
# Tracks render_ledger's rollup header (scripts/dev/rfc_requirements.py _render_rollup).
# `Nightly-only` was added by plan/spec-rfcgate-2-evidence.md: requirements whose only
# evidence runs in the scheduled advisory workflow rather than in ze-verify. It is a
# SUBSET marker, not a partition member, so it is parsed but never summed with the others.
RFC_TABLE_HEADER = (
    "| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | "
    "Nightly-only | State |"
)
RFC_ROW = re.compile(
    r"^\| `([^`]+)` \| (\d+) \| (\d+) \| (\d+) \| (\d+) \| (\d+) \| (\d+) \| (\d+) \| (.*?) \|$"
)
# A requirement line in rfc/short/*.md: "- [ ] [RFC1234-1-1] [MUST] text {gap: ...}"
RFC_LEVEL = re.compile(r"^- \[[ x]\] \[[^\]]+\] \[([A-Z ]+)\]")
# Mirrors GATED_LEVELS in scripts/dev/rfc_requirements.py:69. The ledger's totals
# cover only these, so the page's split must too.
GATED_LEVELS = frozenset({"MUST", "MUST NOT", "SHALL", "SHALL NOT", "REQUIRED"})

TEST_FUNC = re.compile(r"^func (Test[A-Z_][A-Za-z0-9_]*)\(", re.MULTILINE)
FUZZ_FUNC = re.compile(r"^func (Fuzz[A-Z_][A-Za-z0-9_]*)\(", re.MULTILINE)
BENCH_FUNC = re.compile(r"^func (Benchmark[A-Z_][A-Za-z0-9_]*)\(", re.MULTILINE)
# Tokens that EXPECT an error, i.e. the test states that a specific failure must
# occur. Deliberately narrow.
#
# An earlier version also matched `err != nil.*t\.(Fatal|Error)`, which measured
# nearly the opposite of what it claimed: with no re.DOTALL it could not match the
# idiomatic multi-line form at all, while it did match one-line `if err != nil {
# t.Fatalf(...) }` -- which is overwhelmingly a SETUP GUARD, the happy path, not
# an error-path assertion. Comments are stripped before matching so prose
# mentioning wantErr does not count as coverage.
NEGATIVE_ASSERT = re.compile(
    # Helper-based expectations.
    r"\b(wantErr|wantError|expectErr|expectError|ErrorIs|ErrorAs|ErrorContains|"
    r"EqualError|ErrorAssertionFunc)\b|\b(assert|require)\.Error\b"
    # Plain Go, which is this project's house style: an error that MUST be
    # non-nil. Omitting these halved the figure -- 418 files instead of 828 --
    # and made ten subsystems read 0.0% when, for example,
    # internal/component/bfd has 13 of 31 files rejecting forged packets.
    # A metric that measures assertion-library adoption must not be labelled
    # "checks an error path".
    r"|err\s*==\s*nil\s*\{|!errors\.Is\(|!errors\.As\("
)
GO_LINE_COMMENT = re.compile(r"//[^\n]*")
GO_BLOCK_COMMENT = re.compile(r"/\*.*?\*/", re.DOTALL)

# Minimum samples before a series is drawn as a trend. Three points make a
# convincing line out of noise; saying "insufficient data" is the honest answer.
MIN_SAMPLES = 4

# Bound every subprocess: a hung `go run` or `git log` would otherwise hang
# the whole ze-verify run with no diagnostic.
SUBPROCESS_TIMEOUT = 600

OK, WARN, UNKNOWN = "ok", "warn", "unknown"


class Metric:
    """One row on the page.

    `action` is mandatory: a metric whose degradation implies no action is
    decoration, and decoration is what turns a dashboard into a green wall.
    """

    def __init__(
        self, key, question, label, status, value, detail="", action="", data=None
    ):
        self.key = key
        self.question = question
        self.label = label
        self.status = status
        self.value = value
        self.detail = detail
        self.action = action
        self.data = data or {}

    def as_dict(self):
        out = {
            "key": self.key,
            "question": self.question,
            "label": self.label,
            "status": self.status,
            "value": self.value,
            "detail": self.detail,
            "action": self.action,
        }
        out.update(self.data)
        return out


class CollectError(Exception):
    """A collector could not produce a trustworthy number.

    Raised rather than returning zero: a guard that reports a permissive value
    on a miss is worse than no guard (ai/rules/fail-closed-guards.md).
    """


# Cache of the repository's tracked-file list, keyed by root.
_TRACKED_CACHE: dict[str, set[str]] = {}


def tracked_files(root: Path) -> set[str]:
    """Repo-relative paths git has in its index.

    Every collector filters through this so the page is a function of COMMITTED
    state, not of whoever's working tree happens to be checked out. Without it,
    an untracked work-in-progress test moved the published counts, and the
    developer who regenerated the page then committed numbers that a clean CI
    checkout could not reproduce -- the byte-exact staleness gate went red for
    everyone.

    The ratchet is deliberately NOT filtered this way: inert_tests.go scans the
    working tree so `make ze-verify` catches an inert test before it is
    committed, rather than blaming the next unrelated change.
    """
    key = str(root)
    if key in _TRACKED_CACHE:
        return _TRACKED_CACHE[key]
    proc = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        capture_output=True,
        text=True,
        check=False,
        timeout=SUBPROCESS_TIMEOUT,
    )
    if proc.returncode != 0:
        raise CollectError(f"git ls-files failed in {root}: {proc.stderr.strip()}")
    files = {name for name in proc.stdout.split("\0") if name}
    if not files:
        raise CollectError(f"git ls-files listed nothing in {root}")
    _TRACKED_CACHE[key] = files
    return files


def tracked_matching(root: Path, tree: str, suffix: str) -> list[Path]:
    """Tracked files under `tree` ending in `suffix`, in a stable order.

    Reads git's INDEX, so a file deleted or moved in the working tree is still
    listed until that deletion is staged. Those entries are skipped: there is no
    content left to count, and on the clean checkout these counts are meant to
    describe, the deletion is committed and the entry is gone. Without the skip
    every developer mid-refactor gets a bare FileNotFoundError from the caller
    that reads the file, before they are able to commit.
    """
    out = []
    for name in tracked_files(root):
        if not name.startswith(tree + "/") or not name.endswith(suffix):
            continue
        parts = name.split("/")
        if any(p in ("vendor", "testdata", "node_modules") for p in parts):
            continue
        path = root / name
        if not path.exists():
            continue
        out.append(path)
    return sorted(out)


def repo_root() -> Path:
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True,
        text=True,
        check=False,
        timeout=SUBPROCESS_TIMEOUT,
    )
    if out.returncode != 0:
        raise CollectError(
            f"not inside a git repository: {out.stderr.strip()}. This tool derives "
            f"every count from git's index, so it needs one."
        )
    return Path(out.stdout.strip())


def read_text(root: Path, rel: str) -> str:
    path = root / rel
    if not path.exists():
        raise CollectError(f"{rel} does not exist")
    return path.read_text(encoding="utf-8", errors="ignore")


def ratio(num: int, den: int) -> dict:
    """Every ratio carries its parts.

    A percentage alone hides the improvement-by-shrinking-denominator failure:
    a score that rises because the hard packages stopped being sampled looks
    identical to real progress.
    """
    return {
        "numerator": num,
        "denominator": den,
        "percent": round(100.0 * num / den, 1) if den else None,
    }


# --------------------------------------------------------------------------
# Collectors
# --------------------------------------------------------------------------


def collect_rfc(root: Path) -> tuple[Metric, Metric]:
    """Proof density: MUST requirements proven by a test PAIR, over those gated.

    The ledger's own summary reports "0 outstanding", which is true and reads as
    100%. It merges four different states. This splits them back apart.
    """
    text = read_text(root, RFC_LEDGER)
    if RFC_TABLE_HEADER not in text:
        raise CollectError(
            f"{RFC_LEDGER} has no recognisable coverage table header. "
            f"The ledger format changed; update RFC_TABLE_HEADER rather than "
            f"letting this report a zero it did not measure."
        )
    rows = []
    for line in text.splitlines():
        m = RFC_ROW.match(line.strip())
        if m:
            rows.append(
                {
                    "rfc": m.group(1),
                    "gated": int(m.group(2)),
                    "both": int(m.group(3)),
                    "one": int(m.group(4)),
                    "annotated": int(m.group(5)),
                    "notest": int(m.group(6)),
                    "outstanding": int(m.group(7)),
                    "nightly_only": int(m.group(8)),
                }
            )
    if not rows:
        raise CollectError(f"{RFC_LEDGER} coverage table parsed to zero rows")

    gated = sum(r["gated"] for r in rows)
    both = sum(r["both"] for r in rows)
    if gated == 0:
        raise CollectError(f"{RFC_LEDGER} reports zero gated requirements")

    # The split MUST come from the same population as the ledger's totals, or it
    # is not a partition of the remainder. Counting `{kind}` across whole files
    # counted MAY/SHOULD/OPTIONAL requirements too, so the three parts summed to
    # 20 more than `gated - both` while being presented as its breakdown. One
    # file (rfc4577) supplied all 20: 8 MAY, 8 SHOULD, 3 OPTIONAL, 1 SHOULD NOT.
    #
    # Only enrolled summaries with a ledger row count, and only requirement lines
    # whose level is gated, matching GATED_LEVELS in rfc_requirements.py:69.
    kinds = defaultdict(int)
    known_rfcs = {r["rfc"] for r in rows}
    summary_files = tracked_matching(root, RFC_SUMMARIES.split("/")[0], ".md")
    summary_files = [
        p
        for p in summary_files
        if p.parent == root / RFC_SUMMARIES and p.stem in known_rfcs
    ]
    if not summary_files:
        raise CollectError(
            f"no tracked summaries under {RFC_SUMMARIES} match a ledger row: refusing "
            f"to report the annotation split as all-zero when nothing was measured"
        )
    for path in summary_files:
        for line in path.read_text(encoding="utf-8", errors="ignore").splitlines():
            level = RFC_LEVEL.search(line)
            if not level or level.group(1) not in GATED_LEVELS:
                continue
            for kind in ("not-applicable", "gap", "single-polarity"):
                if re.search(r"\{" + kind + r"[:}]", line):
                    kinds[kind] += 1
                    break

    annotated = gated - both
    split_total = sum(kinds.values())
    if split_total != annotated:
        raise CollectError(
            f"annotation split {dict(kinds)} sums to {split_total}, but the ledger's "
            f"remainder is {annotated} ({gated} gated - {both} proven). The page must "
            f"not present a non-partition as one; the two sources have diverged."
        )

    unproven = sorted(
        (r for r in rows if r["gated"] > 0 and r["both"] == 0),
        key=lambda r: -r["gated"],
    )
    density = ratio(both, gated)
    status = quality_status(root, "rfc-proof-density", density["percent"])

    headline = Metric(
        key="rfc-proof-density",
        question="Q2",
        label="RFC MUST requirements proven by a positive+negative test pair",
        status=status,
        value=f"{both} / {gated}",
        detail=(
            f"{density['percent']}% carry both polarities. Of the remaining {annotated}: "
            f"{kinds['not-applicable']} not-applicable (ze deliberately does not do it, so "
            f"no test is owed), {kinds['gap']} known gap (unimplemented, genuinely "
            f"untested), and {kinds['single-polarity']} single-polarity -- those DO have a "
            f"passing tagged test, just one side of the pair, and the RFC gate fails if "
            f"that test is missing. Only the gap column is untested work."
        ),
        action="Convert a {gap} or {single-polarity} annotation into a test pair. "
        "Not-applicable needs no test.",
        data={
            "proof_density": density,
            "annotations": dict(kinds),
            "rfcs_total": len(rows),
            "rfcs_without_any_proof": len(unproven),
            "worst": [{"rfc": r["rfc"], "gated": r["gated"]} for r in unproven[:10]],
        },
    )

    unproven_metric = Metric(
        key="rfc-unproven",
        question="Q2",
        label="Enrolled RFCs with zero test-proven requirements",
        status=OK if not unproven else WARN,
        value=f"{len(unproven)} / {len(rows)}",
        detail="Enrolled and gate-green, but no requirement is proven by BOTH polarities. "
        "Some of these do carry positive-only tests; none carries a pair.",
        action="Pick the largest and complete a pair, or accept it is a single-polarity claim.",
        data={"unproven": ratio(len(unproven), len(rows))},
    )
    return headline, unproven_metric


def _ratchet_status(actual: int, floor):
    """A ratcheted count is healthy while it honours its floor."""
    if floor is None:
        return WARN if actual else OK
    return OK if actual <= floor else WARN


def _floor_suffix(floor) -> str:
    return f" (floor {floor})" if floor is not None else ""


def collect_inert(root: Path) -> tuple[Metric, Metric, dict]:
    """Assert-nothing tests and tag-orphaned test files, from the Go AST gate.

    Status comes from the committed ratchet floor, not from an invented absolute
    threshold. A count sitting exactly at its agreed floor is recorded debt under
    an enforced contract, not a new problem, and colouring it red would make the
    page cry wolf on every run until the debt is fully paid.
    """
    proc = subprocess.run(
        ["go", "run", "scripts/checks/inert_tests.go", "--json", "--tracked-only"],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
        timeout=SUBPROCESS_TIMEOUT,
    )
    if proc.returncode != 0:
        raise CollectError(f"inert_tests.go --json failed: {proc.stderr.strip()}")
    try:
        raw = json.loads(proc.stdout)
    except ValueError as exc:
        raise CollectError(f"inert_tests.go emitted unparseable JSON: {exc}") from exc

    # Read through the same validation tighten_baseline uses. Reading it raw here
    # meant a corrupt baseline crashed with a JSONDecodeError/AttributeError
    # traceback from build(), long before those guards could report it.
    floors = read_baseline(root)

    assert_nothing = raw.get("assert-nothing") or []
    orphans = raw.get("tag-orphan") or []
    scanned = raw.get("tests-scanned", 0)
    if scanned == 0:
        raise CollectError("inert_tests.go scanned zero tests")

    inert = Metric(
        key="assert-nothing",
        question="Q1",
        label="Tests with no reachable failure call",
        status=_ratchet_status(len(assert_nothing), floors.get("assert-nothing")),
        value=f"{len(assert_nothing)} / {scanned}"
        + _floor_suffix(floors.get("assert-nothing")),
        detail="These execute code and pass unconditionally. Breaking the code under test "
        "would not turn them red.",
        action="Add a real assertion, or annotate with `// test-asserts-nothing: <why>` "
        "when the oracle is genuinely implicit (a must-not-panic smoke test).",
        data={
            "inert": ratio(len(assert_nothing), scanned),
            "worst": [
                {"file": f["file"], "test": f.get("test", "")}
                for f in assert_nothing[:10]
            ],
        },
    )
    orphan = Metric(
        key="tag-orphan",
        question="Q3",
        label="Test files no `go test` target can build",
        status=_ratchet_status(len(orphans), floors.get("tag-orphan")),
        value=str(len(orphans)) + _floor_suffix(floors.get("tag-orphan")),
        detail="Their build tags are supplied by no go test invocation in Makefile or mk/*.mk, "
        "so these tests exist but never run.",
        action="Add the tag to a go test invocation, or delete the file. Either way the "
        "false inventory shrinks.",
        data={
            "orphan_count": len(orphans),
            "orphans": [
                {"file": f["file"], "requires": f.get("detail", "")} for f in orphans
            ],
        },
    )
    return inert, orphan, raw


def collect_inventory(root: Path) -> Metric:
    """Honest in-repo test counts, with the counting boundary stated."""
    counts = {"test_funcs": 0, "fuzz_funcs": 0, "bench_funcs": 0, "test_files": 0}
    for tree in TEST_ROOTS:
        for path in tracked_matching(root, tree, "_test.go"):
            counts["test_files"] += 1
            body = path.read_text(encoding="utf-8", errors="ignore")
            counts["test_funcs"] += len(TEST_FUNC.findall(body))
            counts["fuzz_funcs"] += len(FUZZ_FUNC.findall(body))
            counts["bench_funcs"] += len(BENCH_FUNC.findall(body))
    if counts["test_funcs"] == 0:
        raise CollectError("counted zero test functions in the repository")

    counts["ci_files"] = len(tracked_matching(root, "test", ".ci"))
    counts["et_files"] = len(tracked_matching(root, "test", ".et"))

    return Metric(
        key="inventory",
        question="Q2",
        label="In-repo test inventory",
        status=OK,
        value=f"{counts['test_funcs']} test functions",
        detail=(
            f"{counts['test_files']} Go test files, {counts['fuzz_funcs']} fuzz targets, "
            f"{counts['bench_funcs']} benchmarks, {counts['ci_files']} .ci scenarios, "
            f"{counts['et_files']} .et editor tests. "
            f"Counts cover {', '.join(TEST_ROOTS)} only: vendor/ and gokrazy/modcache/ are "
            f"third-party module trees and are excluded."
        ),
        action="This is volume, not health. It is here to state the counting boundary, "
        "because a count that silently includes vendored tests inflates by ~6x.",
        data={"counts": counts},
    )


def collect_mutation(root: Path) -> Metric:
    """Mutation kill rate from the committed history.

    mutation_history.py is advisory and records nothing when gomu's report is
    missing, so an absent series means "not measured", never "score zero".
    """
    path = root / MUTATION_HISTORY
    if not path.exists():
        return Metric(
            key="mutation",
            question="Q1",
            label="Mutation kill rate",
            status=UNKNOWN,
            value="unknown",
            detail=f"{MUTATION_HISTORY} does not exist; no mutation run has been recorded.",
            action="Run `make ze-mutation-changed`, then `make ze-test-health-record`.",
        )
    rows = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except ValueError as exc:
            raise CollectError(
                f"{MUTATION_HISTORY} has an unparseable line: {exc}"
            ) from exc
    if not rows:
        return Metric(
            key="mutation",
            question="Q1",
            label="Mutation kill rate",
            status=UNKNOWN,
            value="unknown",
            detail=f"{MUTATION_HISTORY} is empty.",
            action="Run `make ze-mutation-changed`, then `make ze-test-health-record`.",
        )

    # Latest sample per package, so a package measured twice is not double counted.
    latest = {}
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise CollectError(f"{MUTATION_HISTORY} line {index + 1} is not an object")
        package = row.get("package")
        if not package:
            raise CollectError(
                f"{MUTATION_HISTORY} line {index + 1} has no 'package'; rows without one "
                f"would all collapse into a single bucket and silently overwrite each other"
            )
        latest[package] = row
    mutants = sum(r.get("mutants", 0) for r in latest.values())
    killed = sum(r.get("killed", 0) for r in latest.values())
    if mutants == 0:
        return Metric(
            key="mutation",
            question="Q1",
            label="Mutation kill rate",
            status=UNKNOWN,
            value="unknown",
            detail=f"{MUTATION_HISTORY} records {len(latest)} package(s) but zero mutants; "
            f"nothing was actually measured.",
            action="Run `make ze-mutation-changed`, then `make ze-test-health-record`.",
        )
    kill = ratio(killed, mutants)

    def _score(row):
        value = row.get("score")
        return value if isinstance(value, (int, float)) else 100.0

    weak = sorted(latest.values(), key=_score)[:10]

    return Metric(
        key="mutation",
        question="Q1",
        label="Mutants killed, latest sample per package",
        status=quality_status(root, "mutation", kill["percent"]),
        value=f"{killed} / {mutants}",
        detail=(
            f"{kill['percent']}% across {len(latest)} of the repository's packages. "
            f"Mutation operators are biased toward arithmetic, conditionals and returns, "
            f"and are nearly blind to concurrency and wire-format semantics."
        ),
        action="Take the lowest-scoring package and add tests until its survivors die.",
        data={
            "kill_rate": kill,
            "packages_measured": len(latest),
            "samples": len(rows),
            "worst": [
                {"package": r.get("package"), "score": r.get("score")} for r in weak
            ],
        },
    )


def collect_sleep_ratchet(root: Path) -> Metric:
    """.ci sleep ratchet headroom. Sleeps hide the races they paper over."""
    baseline_path = root / SLEEP_BASELINE
    if not baseline_path.exists():
        return Metric(
            key="ci-sleeps",
            question="Q1",
            label="time.sleep() in .ci tests",
            status=UNKNOWN,
            value="unknown",
            detail=f"{SLEEP_BASELINE} does not exist.",
            action="Restore the baseline file; the ratchet is unenforced without it.",
        )
    # The baseline is the composable delta form (full-line `#` comments plus
    # signed-integer lines that sum to the ceiling); verify_wiring_docs owns the
    # canonical parser. A file with no parseable integer line is malformed and
    # must fail closed -- a garbage baseline may not silently disable the ratchet
    # (ai/rules/fail-closed-guards.md). Imported lazily so a discovery_sources
    # import hiccup cannot break the whole module load.
    from verify_wiring_docs import parse_sleep_baseline

    raw = baseline_path.read_text(encoding="utf-8")
    baseline = parse_sleep_baseline(raw)
    if baseline is None:
        raise CollectError(
            f"{SLEEP_BASELINE} has no parseable ceiling"
            f" (delta form: `#` comments + signed-int lines): {raw.strip()!r}"
        )
    actual = 0
    for path in tracked_matching(root, "test", ".ci"):
        actual += path.read_text(encoding="utf-8", errors="ignore").count("time.sleep(")
    return Metric(
        key="ci-sleeps",
        question="Q1",
        label="time.sleep() calls in .ci tests",
        status=OK if actual <= baseline else WARN,
        value=f"{actual} (floor {baseline})",
        detail="A sleep is a guess about timing that hides the race it was added to mask. "
        "The ratchet allows the count to fall, never rise.",
        action="Replace a sleep with a payload-predicate wait (wait_until, dispatch_until), "
        "then lower the floor in the same change.",
        data={"actual": actual, "baseline": baseline, "headroom": baseline - actual},
    )


def collect_negative_tests(root: Path) -> Metric:
    """Share of test files that assert an error path, per subsystem."""
    per_area = defaultdict(lambda: {"files": 0, "negative": 0})
    for tree in TEST_ROOTS:
        for path in tracked_matching(root, tree, "_test.go"):
            rel = path.relative_to(root)
            # parts[:3] of a 3-component path is the FILE, not a subsystem, so
            # 117 of 318 "areas" were single files and every one was dropped by
            # the >=5 filter -- whole trees could never appear in the table the
            # metric's own action tells you to act on. Bucket by directory.
            dir_parts = rel.parent.parts
            area = "/".join(dir_parts[:3]) if dir_parts else str(rel.parent)
            body = path.read_text(encoding="utf-8", errors="ignore")
            per_area[area]["files"] += 1
            code = GO_BLOCK_COMMENT.sub("", GO_LINE_COMMENT.sub("", body))
            if NEGATIVE_ASSERT.search(code):
                per_area[area]["negative"] += 1

    total_files = sum(a["files"] for a in per_area.values())
    total_neg = sum(a["negative"] for a in per_area.values())
    if total_files == 0:
        raise CollectError("no test files found while measuring negative-test ratio")

    # Only areas with enough files for the ratio to mean anything.
    ranked = sorted(
        ((k, v) for k, v in per_area.items() if v["files"] >= 5),
        key=lambda kv: kv[1]["negative"] / kv[1]["files"],
    )
    overall = ratio(total_neg, total_files)
    return Metric(
        key="negative-tests",
        question="Q2",
        label="Test files that expect a specific error",
        status=quality_status(root, "negative-tests", overall["percent"]),
        value=f"{total_neg} / {total_files}",
        detail="Counts files using an error-expectation token (wantErr, ErrorIs, "
        "assert.Error, ...), with comments stripped. Setup guards of the form "
        "`if err != nil { t.Fatal(err) }` are deliberately NOT counted: those assert the "
        "happy path. Blind spot: expecting *an* error is weaker than pinning the right one.",
        action="Take the lowest-ranked subsystem and add malformed-input or fault-injection cases.",
        data={
            "overall": overall,
            "worst": [
                {
                    "area": k,
                    "negative": v["negative"],
                    "files": v["files"],
                    "percent": round(100.0 * v["negative"] / v["files"], 1),
                }
                for k, v in ranked[:10]
            ],
        },
    )


def collect_adoption(root: Path) -> Metric:
    """Age-bucketed technique adoption: the forward-only-adoption detector.

    A technique introduced in year N and never back-filled shows as a step: new
    packages carry it, older ones never do. Global counts cannot see this, which
    is why ai/rules/testing.md makes back-filling a BLOCKING rule.

    Package age uses each directory's first-commit date. Coarse year buckets keep
    the generated page stable: a new commit to an existing package does not move
    its first-commit year, so the page does not churn.
    """
    shallow = subprocess.run(
        ["git", "rev-parse", "--is-shallow-repository"],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
        timeout=SUBPROCESS_TIMEOUT,
    ).stdout.strip()
    if shallow == "true":
        # In a shallow clone git attributes every file to the graft commit, so
        # every package lands in one bucket. Rendering that would make the page
        # differ from the committed one and red ze-regen-check-readonly with no
        # escape: the diagnostic would say "regenerate and commit", which then
        # breaks every full clone. Unmeasured is the honest answer.
        return Metric(
            key="adoption",
            question="Q2",
            label="Technique adoption by package age",
            status=UNKNOWN,
            value="unknown",
            detail="This is a shallow clone, so git attributes every file to the graft "
            "commit and package age cannot be derived. Re-run in a full clone "
            "(`git fetch --unshallow`).",
            action="Nothing to do here; the metric needs full history.",
        )

    proc = subprocess.run(
        [
            "git",
            # Rename detection is controlled by the USER's diff.renames config
            # (and its default changed in git 2.9). With it on, a renamed file's
            # add is not reported, so 515 directories got a different
            # first-commit stamp and 258 moved year bucket -- the same commit
            # rendered two different pages on two machines. Pin both knobs so
            # the output depends on the repository, not on who ran it.
            "-c",
            "diff.renames=false",
            "-c",
            "core.quotePath=false",
            "log",
            "--reverse",
            "--diff-filter=A",
            "--format=%H %at",
            "--name-only",
            "--no-renames",
        ],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
        timeout=SUBPROCESS_TIMEOUT,
    )
    if proc.returncode != 0:
        raise CollectError(f"git log failed: {proc.stderr.strip()}")

    first_seen: dict[str, int] = {}
    stamp = 0
    for line in proc.stdout.splitlines():
        if not line.strip():
            continue
        parts = line.split()
        if len(parts) == 2 and len(parts[0]) == 40 and parts[1].isdigit():
            stamp = int(parts[1])
            continue
        directory = os.path.dirname(line)
        if directory and directory not in first_seen:
            first_seen[directory] = stamp

    # Track seen directories as a SET per bucket. A single-slot sentinel
    # under-counted nothing but over-counted plenty: sorted() orders by full path
    # string, so a subdirectory sorts between two files of its parent
    # (a/b_test.go, a/sub/c_test.go, a/z_test.go), the parent is re-entered, and
    # its package is counted again -- 70 re-entries on this repo, publishing 490
    # packages where 481 exist. The reset of the per-directory fuzz/rfc flags
    # inflated those two columns the same way.
    buckets: dict[str, dict] = defaultdict(
        lambda: {"packages": 0, "with_fuzz": 0, "with_rfc_tag": 0, "with_ci": 0}
    )
    seen: dict[tuple[str, str], dict] = {}
    undated: list[str] = []
    for tree in TEST_ROOTS:
        for path in tracked_matching(root, tree, "_test.go"):
            directory = str(path.parent.relative_to(root))
            stamp = first_seen.get(directory)
            if stamp is None:
                if directory not in undated:
                    undated.append(directory)
                continue
            year = _year_of(stamp)
            bucket = buckets[year]
            state = seen.get((year, directory))
            if state is None:
                bucket["packages"] += 1
                state = {"fuzz": False, "rfc": False}
                seen[(year, directory)] = state
            body = path.read_text(encoding="utf-8", errors="ignore")
            if not state["fuzz"] and FUZZ_FUNC.search(body):
                bucket["with_fuzz"] += 1
                state["fuzz"] = True
            if not state["rfc"] and "RFC requirement:" in body:
                bucket["with_rfc_tag"] += 1
                state["rfc"] = True

    # .ci functional tests live under test/, not beside the Go test files, so
    # they carry their own adoption story. AC-18 names all three techniques;
    # counting a directory as having .ci coverage lets the step-detector show
    # whether functional testing was back-filled to older subsystems too. Keyed
    # by the .ci file's OWN first-commit date so a package with an old unit test
    # and a new .ci correctly contributes to the year the .ci arrived.
    ci_seen: set[tuple[str, str]] = set()
    for path in tracked_matching(root, "test", ".ci"):
        directory = str(path.parent.relative_to(root))
        stamp = first_seen.get(directory)
        if stamp is None:
            if directory not in undated:
                undated.append(directory)
            continue
        year = _year_of(stamp)
        if (year, directory) in ci_seen:
            continue
        ci_seen.add((year, directory))
        buckets[year]["with_ci"] += 1

    clean = {year: dict(buckets[year]) for year in sorted(buckets)}

    if not clean:
        raise CollectError("could not bucket any package by first-commit date")

    detail = (
        "A technique adopted only forward from its introduction shows here as a step: "
        "recent buckets carry it, older ones never do."
    )
    if undated:
        # Silently dropping these would shrink the denominator with no signal,
        # which is the fail-open this file exists to avoid.
        detail += (
            f" {len(undated)} directory(ies) have no add-commit in this history and are "
            f"excluded; a shallow clone is the usual cause."
        )

    return Metric(
        key="adoption",
        question="Q2",
        label="Technique adoption by package age",
        status=OK if not undated else WARN,
        value=f"{len(clean)} age buckets",
        detail=detail,
        action="Back-fill the oldest bucket, or record the uncovered remainder as tracked "
        "backlog (ai/rules/testing.md, Back-Fill New Test Types).",
        data={"buckets": clean, "undated_directories": len(undated)},
    )


def _year_of(stamp: int) -> str:
    """UTC year of a unix timestamp, without importing a clock."""
    import datetime

    return datetime.datetime.fromtimestamp(stamp, datetime.timezone.utc).strftime("%Y")


def _shard_is_struck(shard: Path) -> bool:
    """True when a live shard's first `### ` heading is struck through (`~~...~~`).

    The sharded model never writes a struck live shard (a cleared red is moved to
    RESOLVED.md and its shard deleted), but a stray strike must not be able to
    inflate the live-debt figure, so it is treated as resolved.
    """
    for line in shard.read_text(encoding="utf-8", errors="ignore").splitlines():
        if line.startswith("### "):
            return line[4:].strip().startswith("~~")
    return False


def collect_known_failures(root: Path) -> Metric:
    """Tests logged as known-red. Debt that is tracked but still debt.

    The single `plan/known-failures.md` file was sharded into
    `plan/known-failures/` (spec-fixit-shared-plan-file-contention) so concurrent
    sessions never cross-commit each other's rows. One file per LIVE failure;
    RESOLVED.md archives the history verbatim; README.md holds the logging
    instructions. Neither of those two is a live failure. The aggregate is folded
    on read here, never stored.
    """
    directory = root / "plan/known-failures"
    if not directory.is_dir():
        # An absent input is unmeasured, never healthy. Reporting "0 known
        # failures" for a directory nobody could read is the sensor-rot failure
        # the page exists to expose. (Mirrors the old absent-file branch.)
        return Metric(
            key="known-failures",
            question="Q3",
            label="Logged known-failing tests",
            status=UNKNOWN,
            value="unknown",
            detail="plan/known-failures/ does not exist, so nothing was measured.",
            action="Restore the directory, or drop this metric if the log was retired.",
        )
    # LIVE = one shard file per live failure, excluding the two bookkeeping files.
    # Counting the RESOLVED archive would report the debt this project has already
    # paid off, the mirror of the inflated-test-count error this page corrects.
    non_shards = {"README.md", "RESOLVED.md"}
    live = 0
    struck = 0
    for shard in sorted(directory.glob("*.md")):
        if shard.name in non_shards:
            continue
        if _shard_is_struck(shard):
            struck += 1
            continue
        live += 1

    resolved = struck
    archive = directory / "RESOLVED.md"
    if archive.exists():
        for line in archive.read_text(encoding="utf-8", errors="ignore").splitlines():
            if line.startswith("### "):
                resolved += 1

    return Metric(
        key="known-failures",
        question="Q3",
        label="Logged known-failing tests",
        status=OK if live == 0 else WARN,
        value=str(live),
        detail=f"Reds logged rather than fixed, one shard file per live failure "
        f"({resolved} entries archived in plan/known-failures/RESOLVED.md are not "
        f"counted). Structural gates may never be logged here, but a live entry is "
        f"not necessarily flaky: some are deterministic product bugs awaiting a fix.",
        action="Fix or delete the oldest entry; a permanently logged failure is a deleted test with extra steps.",
        data={"live": live, "resolved": resolved},
    )


# --------------------------------------------------------------------------
# Rendering
# --------------------------------------------------------------------------

QUESTIONS = {
    "Q1": ("Sensitivity", "If the code were wrong, would something go red?"),
    "Q2": (
        "Intent coverage",
        "Are the things that matter checked, or only the happy path?",
    ),
    "Q3": ("Integrity", "When something goes red, does it stop the line?"),
}

STATUS_MARK = {OK: "ok", WARN: "attention", UNKNOWN: "unknown"}

# Status sort order: unknown first. A dead sensor outranks a known problem,
# because a number nobody is computing is worse than a number that looks bad.
STATUS_ORDER = {UNKNOWN: 0, WARN: 1, OK: 2}


def sparkline(values, width=240, height=40) -> str:
    """Inline SVG polyline. No chart library, no JavaScript.

    Renders as a chart in any Markdown viewer that passes block-level HTML
    through (GitHub, Codeberg, most editors). It is NOT what the website draws:
    ../gh-pages/tools/render-test-health.py builds the published page from
    latest.json with its own sparkline, and the mirrored quality/health/index.md
    is served raw for machine readers rather than rendered to HTML. So this tag
    is for readers of the repository, and it degrades to an inert tag elsewhere.
    """
    if len(values) < 2:
        return ""
    lo, hi = min(values), max(values)
    span = (hi - lo) or 1
    step = width / (len(values) - 1)
    points = " ".join(
        f"{i * step:.1f},{height - ((v - lo) / span) * (height - 4) - 2:.1f}"
        for i, v in enumerate(values)
    )
    return (
        f'<svg viewBox="0 0 {width} {height}" width="{width}" height="{height}" '
        f'role="img" aria-label="trend, {len(values)} samples, min {lo}, max {hi}">'
        f'<polyline points="{points}" fill="none" stroke="currentColor" stroke-width="2"/>'
        f"</svg>"
    )


def render_markdown(metrics, record) -> str:
    lines = []
    add = lines.append

    add("# Testing Health")
    add("")
    add(
        "GENERATED by `make ze-test-health` -- do not edit. Source: "
        "`scripts/dev/testing_health.py`."
    )
    add("")
    add(
        "**How current is this?** The structural facts -- which test files nothing "
        "runs, which RFCs have no test pair, and every metric's status -- are gated "
        "by `make ze-verify` and cannot lag the tree. The volume counters are as of "
        "the last `make ze-regen` and may lag by a few tests; they are deliberately "
        "not gated, because a check that fired on the ~60% of commits that add a "
        "test would be routed around rather than read. The ratchets are enforced "
        "from the tree itself, not from this page."
    )
    add("")
    add(
        "This page answers **is our testing correct**, not *is our testing large*. "
        "Those are different questions. A suite can grow forever while the share of "
        "behaviour it would actually catch a regression in falls, and no count of "
        "tests can show that. Every metric below belongs to one of three questions; "
        "anything belonging to none is volume and is deliberately absent."
    )
    add("")

    # Exceptions first. Green is the absence of information: if the reader has to
    # scroll past healthy rows to find the problems, the page is a trophy case.
    problems = [m for m in metrics if m.status != OK]
    healthy = [m for m in metrics if m.status == OK]

    add("## Needs attention")
    add("")
    if not problems:
        add("Nothing outstanding. Every metric below is within its threshold.")
    else:
        add("| Metric | Question | Value | What to do |")
        add("|---|---|---|---|")
        for m in sorted(problems, key=lambda m: STATUS_ORDER[m.status]):
            add(
                f"| {m.label} | {m.question} | **{m.value}** ({STATUS_MARK[m.status]}) | {m.action} |"
            )
    add("")

    if healthy:
        add(
            f"{len(healthy)} further metric(s) are within threshold and are listed in full below."
        )
        add("")

    for qkey in ("Q1", "Q2", "Q3"):
        title, question = QUESTIONS[qkey]
        group = [m for m in metrics if m.question == qkey]
        if not group:
            continue
        add(f"## {title}")
        add("")
        add(f"*{question}*")
        add("")
        for m in sorted(group, key=lambda m: STATUS_ORDER[m.status]):
            add(f"### {m.label}")
            add("")
            add(f"**{m.value}** ({STATUS_MARK[m.status]})")
            add("")
            if m.detail:
                add(m.detail)
                add("")
            if m.action:
                add(f"*Action if this degrades:* {m.action}")
                add("")
            for table in _detail_tables(m):
                add(table)
                add("")

    add("## Trends")
    add("")
    history = record.get("history", [])
    if len(history) < MIN_SAMPLES:
        add(
            f"Insufficient data: {len(history)} recorded sample(s), {MIN_SAMPLES} needed "
            f"before a trend is drawn. A line through three points is noise with a "
            f"direction. Append a sample with `make ze-test-health-record`."
        )
    else:
        add("| Series | Trend | Latest | Samples |")
        add("|---|---|---|---|")
        for key, label in (
            ("rfc_proof_percent", "RFC proof density %"),
            ("assert_nothing", "Assert-nothing tests"),
            ("tag_orphan", "Tag-orphaned files"),
            ("mutation_percent", "Mutation kill %"),
        ):
            series = [h[key] for h in history if h.get(key) is not None]
            if len(series) < MIN_SAMPLES:
                add(f"| {label} | insufficient data | - | {len(series)} |")
                continue
            add(f"| {label} | {sparkline(series)} | {series[-1]} | {len(series)} |")
    add("")

    add("## How to read this")
    add("")
    add(
        "- Every ratio shows its numerator and denominator. A percentage alone hides "
        "the case where a score improves because the denominator shrank."
    )
    add(
        "- `unknown` is not `ok`. A metric whose input is missing sorts above every "
        "other row, because a number nobody is computing is worse than a bad number."
    )
    add(
        "- Counts marked with a floor are ratchets: they may fall, never rise. "
        "`make ze-verify` enforces them."
    )
    add(
        "- Volume figures cover in-repo trees only. `vendor/` and `gokrazy/modcache/` "
        "are third-party module trees; including them inflates the test count roughly sixfold."
    )
    add("")
    return "\n".join(lines) + "\n"


def _cell(value) -> str:
    """A Markdown table cell: stringified, with the separator escaped."""
    return str(value).replace("|", "\\|")


def _detail_tables(m: Metric):
    """Per-metric supporting tables, ordered deterministically."""
    out = []
    worst = [w for w in (m.data.get("worst") or []) if isinstance(w, dict)]
    if worst:
        # Union of keys, and escape the cell separator. Taking keys from
        # worst[0] alone raised KeyError on a heterogeneous row, and an
        # unescaped "|" in a file name silently broke the table.
        keys = []
        for row in worst:
            for k in row:
                if k not in keys:
                    keys.append(k)
        head = "| " + " | ".join(k.replace("_", " ") for k in keys) + " |"
        sep = "|" + "---|" * len(keys)
        rows = [
            "| " + " | ".join(_cell(w.get(k, "")) for k in keys) + " |" for w in worst
        ]
        out.append("\n".join([head, sep] + rows))
    orphans = m.data.get("orphans")
    if orphans:
        head = "| file | requires |"
        sep = "|---|---|"
        rows = [
            f"| `{_cell(o.get('file', '?'))}` | `{_cell(o.get('requires', '?'))}` |"
            for o in orphans
            if isinstance(o, dict)
        ]
        out.append("\n".join([head, sep] + rows))
    buckets = m.data.get("buckets")
    if buckets:
        head = (
            "| package first commit | packages with tests | with a fuzz target "
            "| with an RFC-tagged test | with a .ci scenario |"
        )
        sep = "|---|---|---|---|---|"
        rows = [
            f"| {year} | {b['packages']} | {b['with_fuzz']} | {b['with_rfc_tag']} "
            f"| {b.get('with_ci', 0)} |"
            for year, b in sorted(buckets.items())
        ]
        out.append("\n".join([head, sep] + rows))
    return out


# --------------------------------------------------------------------------
# Modes
# --------------------------------------------------------------------------


def load_history(root: Path):
    path = root / HISTORY
    if not path.exists():
        return []
    rows = []
    for index, line in enumerate(path.read_text(encoding="utf-8").splitlines()):
        line = line.strip()
        if not line:
            continue
        try:
            rows.append(json.loads(line))
        except ValueError as exc:
            raise CollectError(
                f"{HISTORY} line {index + 1} is not valid JSON: {exc}"
            ) from exc
    return rows


def build(root: Path):
    metrics = []
    headline, unproven = collect_rfc(root)
    inert, orphan, _raw = collect_inert(root)
    metrics.extend(
        [
            headline,
            unproven,
            inert,
            orphan,
            collect_inventory(root),
            collect_mutation(root),
            collect_sleep_ratchet(root),
            collect_negative_tests(root),
            collect_adoption(root),
            collect_known_failures(root),
        ]
    )
    record = {
        "metrics": [m.as_dict() for m in metrics],
        "history": load_history(root),
    }
    return metrics, record


def kpi_row(metrics) -> dict:
    """The KPI subset worth storing per sample. Deliberately small."""
    by_key = {m.key: m for m in metrics}
    rfc = by_key["rfc-proof-density"].data["proof_density"]
    mut = by_key["mutation"].data.get("kill_rate") if by_key["mutation"].data else None
    return {
        "rfc_proof_numerator": rfc["numerator"],
        "rfc_proof_denominator": rfc["denominator"],
        "rfc_proof_percent": rfc["percent"],
        "assert_nothing": by_key["assert-nothing"].data["inert"]["numerator"],
        "tests_scanned": by_key["assert-nothing"].data["inert"]["denominator"],
        "tag_orphan": by_key["tag-orphan"].data["orphan_count"],
        "mutation_percent": mut["percent"] if mut else None,
        "ci_sleeps": by_key["ci-sleeps"].data.get("actual"),
    }


def read_quality_floors(root: Path) -> dict:
    """Locked-in best percentages, or {} when none recorded yet.

    Unlike the sensitivity baseline, a MISSING quality floor is not a laundering
    risk and defaults to "no regression yet": the low number it would flag is
    still published on the page regardless, and no exit code is tied to it, so
    deleting the file to silence a warning is self-defeating.
    """
    path = root / QUALITY_BASELINE
    if not path.exists():
        return {}
    try:
        floors = json.loads(path.read_text(encoding="utf-8"))
    except ValueError as exc:
        raise CollectError(f"{QUALITY_BASELINE} is not valid JSON: {exc}") from exc
    if not isinstance(floors, dict):
        raise CollectError(
            f"{QUALITY_BASELINE} must be a JSON object, found {type(floors).__name__}"
        )
    return floors


def quality_status(root: Path, key: str, percent) -> str:
    """OK unless `percent` has regressed below the committed best for `key`.

    A small tolerance absorbs float-rounding noise so a metric does not warn on
    a 0.05% wobble that is really the same value.
    """
    if percent is None:
        return UNKNOWN
    floor = read_quality_floors(root).get(key)
    if floor is None:
        return OK
    return OK if percent >= floor - 0.1 else WARN


def tighten_quality(root: Path, metrics) -> bool:
    """Raise each quality floor to the best value ever seen. Never lower it.

    Mirror image of tighten_baseline: improvement is locked in, so a later
    regression shows as WARN. Returns True when a floor moved.
    """
    by_key = {m.key: m for m in metrics}
    old = read_quality_floors(root)
    new = dict(old)
    for key in QUALITY_METRICS:
        m = by_key.get(key)
        pct = _metric_percent(m)
        if pct is None:
            continue
        new[key] = max(old.get(key, pct), pct)
    changed = new != old
    path = root / QUALITY_BASELINE
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(new, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return changed


def _metric_percent(metric):
    """The ratio percent a quality metric ratchets on, or None."""
    if metric is None:
        return None
    for key in ("proof_density", "kill_rate", "overall"):
        part = metric.data.get(key)
        if isinstance(part, dict) and part.get("percent") is not None:
            return part["percent"]
    return None


def read_baseline(root: Path) -> dict:
    """The committed ratchet floors, validated. Absent file -> {} (no floors known).

    Every consumer goes through this so a malformed baseline produces one
    diagnosed error rather than a traceback from whichever collector touched it
    first.
    """
    base_path = root / BASELINE
    if not base_path.exists():
        return {}
    try:
        old = json.loads(base_path.read_text(encoding="utf-8"))
    except ValueError as exc:
        raise CollectError(f"{BASELINE} is not valid JSON: {exc}") from exc
    if not isinstance(old, dict):
        raise CollectError(
            f"{BASELINE} must be a JSON object, found {type(old).__name__}"
        )
    for key, floor in old.items():
        if not isinstance(floor, int) or isinstance(floor, bool):
            raise CollectError(
                f"{BASELINE} floor {key!r} is {floor!r}; an integer is required."
            )
        if floor < 0:
            raise CollectError(f"{BASELINE} floor {key!r} is negative: {floor}")
    return old


def tighten_baseline(root: Path, metrics, bootstrap: bool = False) -> bool:
    """Lower the ratchet floors to the counts just measured. Never raise them.

    Returns True when a floor actually moved.

    A floor may only fall. Raising one here would let a regression be laundered
    into the baseline simply by running the generator, which is the opposite of
    what a ratchet is for. A MISSING KEY is therefore an error, not a default:
    defaulting it to the current count is exactly how a raise sneaks through.

    A MISSING FILE is the same hole one level up -- `rm sensitivity-baseline.json
    && make ze-test-health` would mint today's counts as the new floors and
    `make ze-verify` would then pass. Creating the file is a deliberate act,
    requested with --bootstrap-baseline, so the write path can tell bootstrap
    from laundering.
    """
    row = kpi_row(metrics)
    measured = {
        "assert-nothing": row["assert_nothing"],
        "tag-orphan": row["tag_orphan"],
    }
    base_path = root / BASELINE
    new = dict(measured)

    if not base_path.exists() and not bootstrap:
        raise CollectError(
            f"{BASELINE} does not exist. Restore it from git rather than letting this "
            f"run mint {measured} as the new floors: a deleted baseline would launder "
            f"any regression. To create it deliberately the first time, pass "
            f"--bootstrap-baseline."
        )

    if base_path.exists():
        old = read_baseline(root)
        for key in measured:
            if key not in old:
                raise CollectError(
                    f"{BASELINE} has no {key!r} floor. Refusing to invent one: a missing "
                    f"floor silently becomes whatever the count is today, which turns a "
                    f"regression into the new baseline. Restore the file from git."
                )
            new[key] = min(old[key], measured[key])
        changed = new != {k: old[k] for k in measured}
    else:
        changed = True

    base_path.parent.mkdir(parents=True, exist_ok=True)
    base_path.write_text(
        json.dumps(new, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    return changed


def latest_json(record) -> str:
    """The exact bytes of test/health/latest.json, so --write and --check agree."""
    return json.dumps({"metrics": record["metrics"]}, indent=2, sort_keys=True) + "\n"


def do_write(root: Path, bootstrap: bool = False) -> int:
    metrics, record = build(root)

    # Tighten FIRST, then render. The floor is part of the rendered value
    # ("136 / 19856 (floor 136)"), so rendering before tightening produced a page
    # that was stale the instant it was written -- and the ratchet's own advice
    # ("run make ze-test-health to tighten it") therefore broke the staleness
    # gate. Rebuild the metrics after either floor moves so the page and the
    # statuses reflect the new floors.
    moved = tighten_baseline(root, metrics, bootstrap=bootstrap)
    moved = tighten_quality(root, metrics) or moved
    if moved:
        metrics, record = build(root)

    page = render_markdown(metrics, record)

    (root / PAGE).parent.mkdir(parents=True, exist_ok=True)
    (root / PAGE).write_text(page, encoding="utf-8")

    (root / LATEST).parent.mkdir(parents=True, exist_ok=True)
    (root / LATEST).write_text(latest_json(record), encoding="utf-8")

    print(f"test-health: wrote {PAGE}, {LATEST}, {BASELINE}")
    return 0


def structural_facts(record) -> dict:
    """The claims worth gating: the ones whose change is an EVENT, not churn.

    A byte-exact gate over the whole page charged a regeneration-and-commit to
    ~60% of commits, because every added test moves a denominator. That is the
    "advisory gate permanently red" failure this very page is built to expose:
    a check that fires constantly for cosmetic reasons trains people to run
    `make ze-regen` without reading it.

    What is NOT here, deliberately: every volume counter. A stale test count is
    cosmetic. What IS here changes only when something happened:

      * which test files no `go test` target can build -- a new one means a
        build tag or a make target just stranded a file;
      * which enrolled RFCs have no test pair -- one leaving the list is a
        requirement newly proven;
      * every metric's status -- a flip to `warn`, and above all to `unknown`,
        means a collector stopped measuring. Sensor rot is the failure mode the
        page exists to make visible, so it must not be able to land silently.

    The anti-regression guarantee does not rest on this: `ze-test-sensitivity-check`
    enforces the ratchets from the tree at stage 10, reading only the baseline,
    and is untouched by page staleness.
    """
    by_key = {m.get("key"): m for m in record["metrics"]}
    orphans = by_key.get("tag-orphan", {}).get("orphans") or []
    unproven = by_key.get("rfc-unproven", {}).get("worst") or []
    return {
        "statuses": {m.get("key"): m.get("status") for m in record["metrics"]},
        "tag-orphans": sorted(
            (str(o.get("file", "")), str(o.get("requires", "")))
            for o in orphans
            if isinstance(o, dict)
        ),
        "rfc-unproven": sorted(
            str(r.get("rfc", "")) for r in unproven if isinstance(r, dict)
        ),
    }


def _describe(diff_a, diff_b, label) -> list[str]:
    """Name what moved, so the failure is diagnosable without a diff tool."""
    out = []
    if diff_a != diff_b:
        out.append(f"  {label}:")
        out.append(f"    committed: {diff_a}")
        out.append(f"    generated: {diff_b}")
    return out


def do_check(root: Path) -> int:
    metrics, record = build(root)

    for rel in (PAGE, LATEST):
        if not (root / rel).exists():
            print(
                f"test-health: {rel} does not exist. Run `make ze-test-health`.",
                file=sys.stderr,
            )
            return 1

    try:
        committed = json.loads((root / LATEST).read_text(encoding="utf-8"))
    except ValueError as exc:
        print(f"test-health: {LATEST} is not valid JSON: {exc}", file=sys.stderr)
        return 1
    if not isinstance(committed.get("metrics"), list):
        print(f"test-health: {LATEST} has no metrics list.", file=sys.stderr)
        return 1

    want = structural_facts(record)
    got = structural_facts(committed)
    if got != want:
        print(
            "test-health: a STRUCTURAL fact changed without the report being "
            "regenerated. These are gated because each one is an event, not churn.",
            file=sys.stderr,
        )
        for key in ("statuses", "tag-orphans", "rfc-unproven"):
            for line in _describe(got[key], want[key], key):
                print(line, file=sys.stderr)
        print(
            "  Run `make ze-test-health` and commit the result.",
            file=sys.stderr,
        )
        return 1

    print(
        f"test-health: structural facts in {LATEST} match the tree "
        f"(volume counters are not gated; `make ze-regen` refreshes them)"
    )
    return 0


def do_record(root: Path, bootstrap: bool = False) -> int:
    metrics, _record = build(root)

    # Validate everything the follow-up write needs BEFORE touching the
    # append-only history. Appending first meant a failing write left the sample
    # recorded, the page stale, and every retry adding another row -- the exact
    # staleness this command regenerates the page to avoid.
    if not (root / BASELINE).exists() and not bootstrap:
        raise CollectError(
            f"{BASELINE} does not exist, so the page cannot be regenerated after "
            f"recording. Restore it from git, or pass --bootstrap-baseline to create "
            f"it deliberately. Nothing was appended to {HISTORY}."
        )

    row = kpi_row(metrics)
    sha = (
        subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"],
            cwd=root,
            capture_output=True,
            text=True,
            check=False,
            timeout=SUBPROCESS_TIMEOUT,
        ).stdout.strip()
        or "unknown"
    )
    # Wall clock is allowed HERE and nowhere else: history is append-only, so a
    # timestamp cannot make the generated page churn.
    import datetime

    row = {
        "ts": datetime.datetime.now(datetime.timezone.utc).strftime(
            "%Y-%m-%dT%H:%M:%SZ"
        ),
        "sha": sha,
        **row,
    }
    # Skip a sample identical to the previous one at the same commit. Now that
    # --record runs from every mutation target, re-running at one sha would
    # otherwise stack duplicate points into the sparkline and overstate n --
    # and n is what the page prints beside every trend to keep it honest.
    previous = load_history(root)
    if previous:
        last = dict(previous[-1])
        candidate = dict(row)
        for key in ("ts",):
            last.pop(key, None)
            candidate.pop(key, None)
        if last == candidate:
            print(
                f"test-health: sample identical to the last one at {row['sha']}; "
                f"nothing appended to {HISTORY}"
            )
            return do_write(root, bootstrap=bootstrap)

    path = root / HISTORY
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as fh:
        fh.write(json.dumps(row, separators=(",", ":")) + "\n")
    print(f"test-health: recorded one sample in {HISTORY}")
    # The page renders trends from the history, so appending to it makes the
    # committed page stale. Regenerate in the same command or the very next
    # ze-verify fails on a staleness the recorder itself caused.
    return do_write(root, bootstrap=bootstrap)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--write", action="store_true", help="regenerate the page and artifacts"
    )
    parser.add_argument(
        "--check", action="store_true", help="fail if the committed page is stale"
    )
    parser.add_argument(
        "--record", action="store_true", help="append one KPI row to the history"
    )
    parser.add_argument(
        "--json", action="store_true", help="emit the metric record on stdout"
    )
    parser.add_argument(
        "--emit-page",
        action="store_true",
        help="print the page Markdown on stdout without writing any file or touching "
        "a baseline (read-only; used by the website build to publish current numbers)",
    )
    parser.add_argument(
        "--bootstrap-baseline",
        action="store_true",
        help="create test/health/sensitivity-baseline.json when it does not exist "
        "(deliberate first-time setup; without this a missing baseline is an error, "
        "so deleting it cannot launder a regression)",
    )
    parser.add_argument(
        "--root", default=None, help="repository root (defaults to git toplevel)"
    )
    args = parser.parse_args()

    root = Path(args.root).resolve() if args.root else repo_root()

    try:
        if args.write:
            return do_write(root, bootstrap=args.bootstrap_baseline)
        if args.check:
            return do_check(root)
        if args.record:
            return do_record(root, bootstrap=args.bootstrap_baseline)
        metrics, record = build(root)
        if args.emit_page:
            # Read-only twin of do_write's page: no file written, no baseline
            # tightened. render_markdown runs against the live metrics, so the
            # bytes match do_write's EXCEPT where a floor would have moved --
            # do_write tightens the baseline first, this does not, so a "(floor
            # N)" here shows the still-committed floor. That is the honest
            # current-state view the website build wants: current numbers with no
            # mutation of this repo or its ratchet floors.
            sys.stdout.write(render_markdown(metrics, record))
            return 0
        if args.json:
            print(json.dumps(record, indent=2, sort_keys=True))
        else:
            print(
                json.dumps({m["key"]: m["value"] for m in record["metrics"]}, indent=2)
            )
        return 0
    except CollectError as exc:
        print(f"test-health: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
