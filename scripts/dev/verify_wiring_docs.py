#!/usr/bin/env python3
"""Run changed-file-aware wiring, documentation, command, and inventory gates.

The script is intentionally a router. It keeps the expensive checks in their
existing direct targets, and only decides which ones apply to the current diff.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Iterable

from discovery_sources import is_discovery_source as discovery_is_source


@dataclass(frozen=True)
class Symbol:
    path: str
    line: int
    kind: str
    name: str


TARGET_ORDER = (
    "wiring",
    "ze-command-contract-check",
    "ze-command-ownership-check",
    "ze-doc-verify",
    "ze-doc-index-check",
    "ze-discovery-index-check",
    "ze-digest-check",
    "ze-inventory-json",
    "ze-command-list-json",
    "ze-plugin-imports-check",
    "ze-templ-output-check",
    "ze-functional-docker-exec-check",
    "ze-spec-citation-check",
)

MAKE_TARGETS = {
    "ze-command-contract-check",
    "ze-command-ownership-check",
    "ze-doc-verify",
    "ze-doc-index-check",
    "ze-discovery-index-check",
    "ze-digest-check",
    "ze-inventory-json",
    "ze-command-list-json",
    "ze-plugin-imports-check",
    "ze-templ-output-check",
    "ze-functional-docker-exec-check",
    "ze-spec-citation-check",
}

# Reviewed exceptions for exported symbols that are deliberately API surface.
# Keep this small. A new entry here should name the path and symbol exactly.
WIRING_ALLOWLIST: set[tuple[str, str]] = {
    # Cross-package test API: plugins (e.g. bgp/plugins/role) look up their
    # registered attr-mod handler in their own tests.
    ("internal/component/bgp/filterapi/filterapi.go", "AttrModHandlerFor"),
    # R9 sibling-collision (CheckSiblings) needs the sibling token names at one
    # tree level, so only the static grammar gate (scripts/checks/cli_grammar.go,
    # which walks the whole YANG command tree) can run it -- per-command
    # registration cannot see siblings. Gate-only by design; its only caller is
    # in scripts/, which has_production_reference does not count. See the grammar
    # package doc in checker.go.
    ("internal/component/command/grammar/checker.go", "CheckSiblings"),
    # Cross-package test seam: cliio's stdin/stdout are unexported package vars,
    # so command/mrt/analyze/doctor tests in OTHER packages inject in-memory
    # streams via cliio.SwapStreams to exercise the "-" (stdin/stdout) path. It is
    # test-only by design (a one-shot `ze` uses the real os.Stdin/os.Stdout); no
    # production caller. See internal/core/cliio/doc.go.
    ("internal/core/cliio/cliio.go", "SwapStreams"),
    # Same shape as CheckSiblings above: the config claim audit compares the
    # WHOLE config schema against the WHOLE claim union, which only a gate that
    # links internal/component/plugin/all can assemble. Its production caller is
    # scripts/checks/config_claims.go (make ze-config-claims-check, a verify
    # stage), which has_production_reference does not count. The daemon-side
    # entry point is AuditConfigured, called by the doctor check.
    ("internal/component/config/claims/claims.go", "Audit"),
    # grpc-go invokes these methods through stats.Handler. The concrete handler
    # is installed in NewGRPCServer, so no production source names the methods.
    ("internal/component/api/grpc/server.go", "TagRPC"),
    ("internal/component/api/grpc/server.go", "HandleRPC"),
    ("internal/component/api/grpc/server.go", "TagConn"),
    ("internal/component/api/grpc/server.go", "HandleConn"),
    # internal/test/golden is the golden-capture harness. Its callers are the
    # _test.go files of internal/component/web and internal/component/lg, which
    # is a different package, so every entry point it offers must be exported
    # and none of them can have a production caller. The package ships no
    # production code at all: `make ze-web-golden-check` is its only consumer.
    ("internal/test/golden/names.go", "AssertCoversDir"),
    ("internal/test/golden/names.go", "AssertCoversNames"),
    ("internal/test/golden/names.go", "AssertUniqueNames"),
    ("internal/test/golden/response.go", "VersionHeader"),
    ("internal/test/golden/routes.go", "RoutePatterns"),
    ("internal/test/golden/routes.go", "RepoFile"),
    ("internal/test/golden/response.go", "AssertResponseHasBody"),
    # The normalizer this file used to exempt is normalizeHTML now, and an
    # unexported name needs no entry. It had no caller in any other package, not
    # even a _test.go one, so the exemption was keeping a name exported that
    # only its own package spelled. AssertPortFidelity below is the entry point.
    # portcheck.go runs that comparison. `make ze-templ-port-check REF=<sha>`
    # reads the pre-port fixtures out of git at REF and compares them against
    # the ones on disk. Its callers are TestWebTemplPortFidelity and
    # TestLGTemplPortFidelity, one in each ported package.
    ("internal/test/golden/portcheck.go", "AssertPortFidelity"),
    ("internal/test/golden/portcheck.go", "PortRef"),
    ("internal/test/golden/portcheck.go", "PortResponse"),
    # internal/test/templcheck is the AC-8 guard of the same spec. It reads the
    # generated templ components of a package and refuses a parameter the
    # compiler cannot check a field name against. internal/component/lg calls it
    # from view_test.go and internal/component/web joins in phase 3, so its
    # entry points are exported and no production caller exists by design.
    # internal/test/markupcheck reads the CAPTURED pages of a package and reports
    # each htmx attribute whose asset that page's own head does not load. Its
    # callers are TestLGPageImportsCoverRenderedAttributes and
    # TestChaosPageImportsCoverRenderedAttributes, one in each capturing package,
    # and each one asserts the floor itself: the report returns findings rather
    # than taking a *testing.T, so the count a package expects stays with the
    # package that knows its own fixtures. No production caller exists by design.
    ("internal/test/markupcheck/head.go", "HeadCoverageFindings"),
    ("internal/test/templcheck/templcheck.go", "Report"),
    ("internal/test/templcheck/templcheck.go", "AssertTyped"),
}

# User-facing area -> functional suite directory expected to change with it
# (ai/rules/testing.md). Advisory, not blocking: a session that
# changed user-facing behavior with no test/ change gets a named pointer.
FUNCTIONAL_SUITE_BY_AREA = {
    "internal/component/cli/": "test/ui/ or test/editor/",
    "internal/component/web/": "test/web/",
    "internal/component/config/": "test/parse/",
    "internal/component/cmd/": "test/ui/",
}

FUNC_RE = re.compile(r"^func\s+(?:\([^)]*\)\s*)?([A-Z][A-Za-z0-9_]*)\s*\(")
TYPE_RE = re.compile(r"^type\s+([A-Z][A-Za-z0-9_]*)\b")
CONST_RE = re.compile(r"^const\s+(.+)$")
VAR_RE = re.compile(r"^var\s+(.+)$")
BLOCK_RE = re.compile(r"^(type|const|var)\s*\(")
IDENT_LIST_RE = re.compile(
    r"^([A-Za-z_][A-Za-z0-9_]*(?:\s*,\s*[A-Za-z_][A-Za-z0-9_]*)*)\b"
)
TOKEN_TEMPLATE = r"\b{}\b"


class GateFailure(Exception):
    pass


FAILURE_GROUP_PREFIX = "VERIFY FAILURE GROUP:"
FAILURE_GROUPS_COMPLETE_PREFIX = "VERIFY FAILURE GROUPS COMPLETE:"

# The two kinds a group of this gate can carry. group_related_paths
# (scripts/dev/commit_helper.py) reads paths out of a group only when its kind is
# inside PATH_BEARING_GROUP_KINDS, which holds "files" and not "subcheck". So a failure
# that names files is attributable, and a failure that names none is charged to
# the session that is committing. Charging is the deliberate half: guessing which
# files a check name covers would let a real red go uncharged.
PATH_BEARING_KIND = "files"
UNATTRIBUTABLE_KIND = "subcheck"

# splitLines (scripts/status/verify_run.go) reads the stage log with a scanner
# whose token limit is maxLogLineBytes, 4 MiB. A line above it does not just
# vanish: the scan ENDS there, so that line and every line after it go
# unclassified, and the reader charges the stage for the part it could not read.
# A tree-wide check can name hundreds of files, so one group carries at most this
# many paths and the rest go to sibling groups. Every group line then stays three
# orders of magnitude below the limit, and nothing is dropped: a truncated
# `related` would hide the very file that makes the red this session's.
RELATED_PER_GROUP = 50

# Every group id this run declared, in the order it declared them. The count is
# what declare_groups_complete publishes, and the consumer refuses to prefer
# declared groups unless that count matches what it read.
DECLARED_GROUPS: list[str] = []


def declare_failure_group(
    name: str, related: list[str], summary: str, rerun: str
) -> None:
    """Declare the failure group for the failure being reported, right here.

    The producer knows which files its failure is about; the verify runner
    reading its prose is guessing (classifyWiringDocs, scripts/status/verify_run.go).
    So each failure prints its own group, and `related` names the repository
    paths it is about, or is empty when the check judges a population rather than
    a file.

    MUST be called at the point of failure, never accumulated for an end-of-run
    dump: `main` returns early when the wiring check fails, and `run_make_target`
    raises past the remaining targets, so a dump would print the groups of the
    failures that happen to come last and none of the others.

    The line is built with json.dumps, so a path holding a quote, a newline, or
    the prefix itself is escaped into the JSON string it belongs to and cannot
    forge a second group.
    """
    kind = PATH_BEARING_KIND if related else UNATTRIBUTABLE_KIND
    chunks = [
        related[i : i + RELATED_PER_GROUP]
        for i in range(0, len(related), RELATED_PER_GROUP)
    ] or [[]]
    for number, chunk in enumerate(chunks, start=1):
        group_id = f"{kind}:{name}" if number == 1 else f"{kind}:{name}#{number}"
        payload = {
            "group-id": group_id,
            "kind": kind,
            "related": chunk,
            "summary": summary,
            "rerun": rerun,
        }
        DECLARED_GROUPS.append(group_id)
        print(f"{FAILURE_GROUP_PREFIX} {json.dumps(payload)}")


def declare_groups_complete() -> None:
    """State how many groups this run declared, which is what lets them be used.

    The verify runner prefers declared groups over its own classifier only when
    this count matches the number of group lines it read (parseDeclaredGroups,
    scripts/status/verify_run.go).

    What the count detects is a group set that reached the reader DAMAGED, and
    all three cases are failures of the channel rather than of a check:

    | The reader sees | Because |
    |-----------------|---------|
    | fewer group lines than the count | the stage log was truncated, or the run died between a group and its count |
    | more group lines than the count | a child process relayed a group line of its own into this stage's log |
    | no count at all | the run died before it got here, so nothing is trusted |

    It does NOT detect a failure that declared no group. This number is
    `len(DECLARED_GROUPS)`, so it counts the declarations that were MADE, and a
    missed failure raises neither side of the comparison. `run_check` is what
    closes that hole, at the producer: a check that fails having declared nothing
    gets an unattributable group there, so by the time this line is printed every
    failure of this run has a group and the count is honest by construction.
    """
    print(f"{FAILURE_GROUPS_COMPLETE_PREFIX} {len(DECLARED_GROUPS)}")


def run_check(name: str, rerun: str, check: Callable[[], int | None]) -> int:
    """Run one sub-check, and let no failure of it leave the failure index.

    A check that fails without declaring a group would publish a count that
    agrees with itself: `declare_groups_complete` counts the declarations that
    were made, so a missed failure raises neither the count nor the number of
    group lines the reader parses. The reader would then read `complete=true`,
    prefer a group set that says nothing about that failure, and drop the red.

    So "declared nothing" is turned into "declared one no-file group" here. Its
    kind is UNATTRIBUTABLE_KIND, which `structural_gate_reds`
    (scripts/dev/commit_helper.py) CHARGES to the session that is committing.
    Every route out of a check is covered: a non-zero return, and a GateFailure
    raised past the remaining work, which `run_make_target` uses for an unknown
    target before it has declared anything.

    A check that declared its own group keeps it and gets no second one.
    """
    before = len(DECLARED_GROUPS)
    try:
        rc = check() or 0
    except GateFailure:
        _declare_silent_failure(name, rerun, before)
        raise
    if rc:
        _declare_silent_failure(name, rerun, before)
    return rc


def _declare_silent_failure(name: str, rerun: str, before: int) -> None:
    """Declare an unattributable group for a failure that declared none."""
    if len(DECLARED_GROUPS) != before:
        return
    declare_failure_group(
        name,
        [],
        f"{name} failed without naming the files it is about",
        rerun,
    )


# The `<path>:<line>:` prefix every finding of check_doc_links.py carries. The
# path names the file holding the broken `// Design:` reference, which is the
# file that has to be edited, and so the file whose owner earns the red.
CHILD_FINDING_RE = re.compile(r"^([^\s:]+):\d+: ")


def child_finding_paths(root: Path, text: str) -> list[str]:
    """The repository paths a captured child's findings name, sorted and deduped.

    A token is kept only when it is a relative path the checkout holds, which is
    the same test related_repo_path (scripts/dev/commit_helper.py) applies later.
    A finding whose prefix names nothing contributes no path, and a check whose
    findings contribute none declares an unattributable group instead of a false
    one.
    """
    found: set[str] = set()
    for line in text.splitlines():
        match = CHILD_FINDING_RE.match(line)
        if not match:
            continue
        candidate = match.group(1)
        if candidate.startswith("/") or ".." in Path(candidate).parts:
            continue  # absolute or escaping: not a path in this checkout
        if (root / candidate).exists():
            found.add(candidate)
    return sorted(found)


def check_design_refs(root: Path) -> int:
    """Run the Design-ref existence gate (check_doc_links --design-only).

    Unconditional: closure debt is non-local -- deleting/closing a spec
    orphans `// Design:` refs in any source file, so this scans the whole
    tree on every verify, not only when related files change. Enforces
    ai/rules/planning.md "Design references survive closure" (the rule had a
    checker but no gate, so 201 dangling refs once accumulated silently).
    """
    checker = root / "scripts" / "dev" / "check_doc_links.py"
    if not checker.exists():
        # Isolated test root or minimal checkout without the checker: skip
        # rather than fail. The real verify always runs with the repo as root.
        return 0
    # Captured, not inherited. The paths this check is complaining about live in
    # the child's own output, and this gate has to name them: it is unconditional
    # and tree-wide, so on a shared checkout it is among the likeliest reds, and a
    # red naming no file is charged to whoever is committing. Capturing also ends
    # the interleave -- this script never flushes and its recipe passes no -u, so
    # a child writing to the inherited descriptor could print ahead of a parent
    # line produced before it.
    proc = subprocess.run(
        ["python3", "scripts/dev/check_doc_links.py", "--design-only"],
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.stdout:
        print(proc.stdout, end="")
    if proc.stderr:
        print(proc.stderr, end="", file=sys.stderr)
    if not proc.returncode:
        return 0
    declare_failure_group(
        "design-refs",
        child_finding_paths(root, proc.stdout),
        "a `// Design:` reference does not resolve to a durable document",
        "python3 scripts/dev/check_doc_links.py --design-only",
    )
    return 1


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="repository root, defaults to cwd")
    parser.add_argument(
        "--changed-file",
        action="append",
        default=[],
        help="changed file to evaluate; repeatable",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="print selected gates instead of running them",
    )
    parser.add_argument(
        "--make", default="make", help="make executable used for delegated targets"
    )
    parser.add_argument(
        "--check-plugin-imports",
        action="store_true",
        help="verify internal/component/plugin/all/all.go without rewriting it",
    )
    args = parser.parse_args()

    root = find_repo_root(Path(args.root).resolve())
    if args.check_plugin_imports:
        # Not part of the gate: the ze-doc-wiring-check recipe (mk/check-docs.mk)
        # never passes this flag, so this branch declares no group and states no
        # count. A caller that passes it gets the check and nothing else.
        check_plugin_imports(root)
        return 0

    try:
        rc = run_gate(root, args)
    except GateFailure as exc:
        print(str(exc), file=sys.stderr)
        rc = 1
    if rc:
        declare_groups_complete()
    return rc


def run_gate(root: Path, args: argparse.Namespace) -> int:
    """Run the selected gates. Every CHECK that fails here declares its group.

    Every check runs through `run_check`, and that is what makes the property
    structural rather than a convention a new check can forget: a check that
    fails having declared nothing is given an unattributable group there, so it
    is charged instead of leaving the failure index. A check that knows its own
    files still declares them itself, at the point of failure, and keeps them.

    One failure is the RUN's rather than a check's: `changed_files` raises when
    git is unusable, before any check has run. It declares nothing, so the count
    is zero, and zero declared groups is what sends the stage back to its
    classifier.
    """
    gate_rerun = "make ze-doc-wiring-check"
    changed = [normalize_changed_path(root, p) for p in args.changed_file]

    if not changed:
        changed = changed_files(root)

    targets = selected_targets(root, changed)
    advisory = functional_test_advisory(changed)
    if args.dry_run:
        if targets:
            print("\n".join(targets))
        else:
            print("No wiring/doc/inventory checks needed")
        if advisory:
            print(advisory)
        return 0

    gate_rc = 0
    for name, check in (
        ("ci-sleep-ratchet", lambda: check_ci_sleep_ratchet(root, changed)),
        ("ci-sleep-justification", lambda: check_ci_sleep_justification(root, changed)),
        (
            "known-failure-load-excuse",
            lambda: check_known_failure_load_excuses(root, changed),
        ),
        ("ci-log-subsystem-key", lambda: check_ci_log_subsystem_keys(root, changed)),
        ("design-refs", lambda: check_design_refs(root)),
    ):
        # Every check runs, whatever an earlier one returned, and gate_rc keeps
        # the first non-zero exactly as the chain of `or`s it replaces did.
        rc = run_check(name, gate_rerun, check)
        gate_rc = gate_rc or rc

    if not targets:
        print("No wiring/doc/inventory checks needed")
        if advisory:
            print(advisory)
        return gate_rc

    if "wiring" in targets:
        if run_check("wiring", gate_rerun, lambda: report_wiring(root, changed)):
            return 1

    for target in targets:
        if target == "wiring":
            continue
        run_check(
            target,
            f"make {target}",
            lambda t=target: run_make_target(args.make, t, root),
        )

    if advisory:
        print(advisory)
    if gate_rc:
        return gate_rc
    print("Wiring/doc/inventory gates passed")
    return 0


SLEEP_BASELINE = "test/.ci-sleep-baseline"
SLEEP_RE = re.compile(r"time\.sleep\(")
_SIGNED_INT_RE = re.compile(r"^[+-]?\d+$")

# Functional tests under development live here. Gitignored and invisible to every
# repo-wide gate, so a draft cannot move this ratchet or redden any other check
# (test/draft/README.md, internal/test/runner/draft_dir.go).
DRAFT_DIR = "draft"


def real_ci_files(root: Path):
    """Every .ci test under test/, EXCLUDING the test/draft/ incubator.

    A draft carrying an unjustified `time.sleep(` would otherwise raise the count
    over the committed ceiling and fail the ratchet for whoever ran verify next --
    including a session that had nothing to do with the draft.
    """
    for ci in (root / "test").rglob("*.ci"):
        if ci.relative_to(root / "test").parts[:1] == (DRAFT_DIR,):
            continue
        yield ci


def parse_sleep_baseline(text: str) -> int | None:
    """Ceiling from the composable delta baseline: the SUM of every signed-integer
    line (comments `#` and blanks ignored). Returns None when no integer line is
    present (ratchet inactive for this tree).

    The delta form replaces the old single absolute integer so two independent
    sleep-removals append distinct `-N` lines instead of both editing one number,
    which used to guarantee a merge conflict on the second land. A plain single
    `125` still parses (backward compatible: one line, one summand). A `+N` line
    is the explicit-approval knob that raises the ceiling (as editing the old
    absolute integer upward once did).
    """
    total = 0
    seen = False
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if not _SIGNED_INT_RE.match(stripped):
            continue
        total += int(stripped)
        seen = True
    return total if seen else None


def check_ci_sleep_ratchet(root: Path, changed: Iterable[str]) -> int:
    """Ratchet: the time.sleep count in .ci files may only go down.

    Sleeps in embedded observers hide real races; ze_api provides
    wait_for_event / wait_for_shutdown. Legacy sleeps are tolerated at the
    committed baseline; new ones fail the gate. The baseline is a composable
    delta ledger (see parse_sleep_baseline) so parallel removals do not conflict.
    Returns the process exit contribution (0 ok, 1 failed).
    """
    if not any(p.startswith("test/") and p.endswith(".ci") for p in changed):
        return 0
    baseline_path = root / SLEEP_BASELINE
    try:
        ceiling = parse_sleep_baseline(baseline_path.read_text())
    except OSError:
        return 0  # no baseline committed: ratchet not active for this tree
    if ceiling is None:
        return 0
    count = 0
    for ci in real_ci_files(root):
        try:
            count += len(SLEEP_RE.findall(ci.read_text(encoding="utf-8")))
        except OSError:
            continue
    if count > ceiling:
        print("ci-sleep ratchet FAILED:")
        print(
            f"  test/**/*.ci now contains {count} time.sleep( calls; the"
            f" committed delta baseline ({SLEEP_BASELINE}) allows {ceiling}."
        )
        print(
            "  Replace the new sleep with ze_api wait_for_event /"
            " wait_for_shutdown (test/scripts/ze_api.py), or raise the"
            " ceiling only with explicit user approval (append a `+N` line)."
        )
        # No file to name: `count` is a sum over every .ci in the tree measured
        # against one committed ceiling, so no single file is the offender. The
        # group is declared anyway, naming nothing, so the commit helper charges
        # this gate to the session that is committing instead of dropping the red.
        declare_failure_group(
            "ci-sleep-ratchet",
            [],
            f"test/**/*.ci holds {count} time.sleep( calls against a ceiling of {ceiling}",
            "make ze-doc-wiring-check",
        )
        return 1
    if count < ceiling:
        print(
            f"ci-sleep ratchet: count dropped to {count} (ceiling {ceiling});"
            f" append a `-{ceiling - count}` delta line to {SLEEP_BASELINE}"
            " in this change to tighten it."
        )
    else:
        print(f"ci-sleep ratchet OK ({count} <= ceiling {ceiling})")
    return 0


def _sleep_is_justified(lines: list[str], idx: int) -> bool:
    """True when the time.sleep on line `idx` (0-based) carries an explanatory
    comment: either a `#` trailing the call on the same line, or a `#` comment on
    the nearest preceding non-blank line. Mirrors the placement the annotation
    playbook enforces so a reader can see why the sleep was not converted to a
    deterministic wait (ai/rules/testing.md)."""
    after = lines[idx].split("time.sleep(", 1)[1]
    if "#" in after:
        return True
    j = idx - 1
    while j >= 0:
        stripped = lines[j].strip()
        if not stripped:
            j -= 1
            continue
        return stripped.startswith("#")
    return False


def check_ci_sleep_justification(root: Path, changed: Iterable[str]) -> int:
    """Every time.sleep( in a CHANGED .ci test must be justified by a comment.

    The ratchet caps how MANY sleeps exist; this caps how many are unexplained.
    A blind sleep hides why it was left un-converted (deliberate timer, needs-linux
    QEMU-only effect, no queryable readiness signal). Requiring a comment on/above
    each sleep makes that reason auditable. Scoped to changed files: a session is
    responsible for the sleeps in the .ci files it touches, not the whole tree.
    Returns the process exit contribution (0 ok, 1 failed).
    """
    ci_changed = [p for p in changed if p.startswith("test/") and p.endswith(".ci")]
    if not ci_changed:
        return 0
    violations: list[str] = []
    offenders: set[str] = set()
    checked = 0
    for rel in ci_changed:
        try:
            lines = (root / rel).read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        for i, line in enumerate(lines):
            if not SLEEP_RE.search(line):
                continue
            if line.strip().startswith("#"):
                continue  # the sleep is itself commented out; nothing to justify
            checked += 1
            if not _sleep_is_justified(lines, i):
                violations.append(f"{rel}:{i + 1}: {line.strip()}")
                offenders.add(rel)
    if violations:
        print("ci-sleep justification FAILED:")
        print("  Every time.sleep( in a changed .ci test must carry a comment on the")
        print("  line directly above it (or trailing it) explaining why it is there /")
        print("  why it was not converted to a deterministic wait. Unjustified sleeps:")
        for v in violations:
            print("    " + v)
        print(
            "  Add a `#` comment (poll interval, deliberate timer, needs-linux effect,"
        )
        print("  or no queryable readiness signal). See ai/rules/testing.md.")
        declare_failure_group(
            "ci-sleep-justification",
            sorted(offenders),
            "a changed .ci test holds a time.sleep( with no comment saying why",
            "make ze-doc-wiring-check",
        )
        return 1
    if checked:
        print(f"ci-sleep justification OK ({checked} sleeps, all commented)")
    return 0


LOAD_EXCUSE_RE = re.compile(
    r"under load|loaded host|load average|load[- ]sensitive"
    r"|pass(?:es|ed)? in isolation|resource contention|contended host",
    re.IGNORECASE,
)

KNOWN_FAILURES_DIR = "plan/known-failures/"
KNOWN_FAILURES_EXEMPT = {"README.md", "RESOLVED.md"}


def check_known_failure_load_excuses(root: Path, changed: Iterable[str]) -> int:
    """A CHANGED known-failures shard may not blame host load.

    Load is a mechanism, not a mystery: once a shard can say "it fails when the
    machine is busy", the diagnosis is that the test asserts on elapsed time
    instead of on state, and the deliverable is the fix (ai/rules/completion.md,
    owner directive 2026-07-26). The shard directory stays open for a red whose
    mechanism is genuinely unknown, which is why this is a phrase check rather than
    a ban on shards.

    Scoped to changed files, like check_ci_sleep_justification: a session owns the
    shards it writes, not the whole backlog. README.md and RESOLVED.md are exempt --
    the first states this policy and the second is a verbatim archive of history
    that must not be edited to satisfy a present-day gate.
    Returns the process exit contribution (0 ok, 1 failed).
    """
    shards = [
        p
        for p in changed
        if p.startswith(KNOWN_FAILURES_DIR)
        and p.endswith(".md")
        and Path(p).name not in KNOWN_FAILURES_EXEMPT
    ]
    if not shards:
        return 0
    violations: list[str] = []
    offenders: set[str] = set()
    for rel in shards:
        try:
            lines = (root / rel).read_text(encoding="utf-8").splitlines()
        except OSError:
            continue  # deleting a shard is the intended outcome, not a violation
        for i, line in enumerate(lines):
            if LOAD_EXCUSE_RE.search(line):
                violations.append(f"{rel}:{i + 1}: {line.strip()}")
                offenders.add(rel)
    if violations:
        print("known-failure load excuse FAILED:")
        print("  A shard may not attribute a red to host load. That attribution IS")
        print("  the diagnosis: the test asserts on elapsed time instead of on state.")
        for v in violations:
            print("    " + v)
        print("  Fix the test to wait on the condition (ze_api wait_until /")
        print("  dispatch_until / wait_for_event), then delete the shard. Raising a")
        print("  timeout is not a fix. See ai/rules/completion.md.")
        declare_failure_group(
            "known-failure-load-excuse",
            sorted(offenders),
            "a changed known-failures shard blames host load for its red",
            "make ze-doc-wiring-check",
        )
        return 1
    print(f"known-failure load excuse OK ({len(shards)} shard(s) checked)")
    return 0


CI_LOG_KEY_RE = re.compile(r"ze\.log\.([A-Za-z0-9._-]+)")
GO_SOURCE_ROOTS = ("internal", "pkg", "cmd")


def _hyphenated_subsystems_in_go(root: Path) -> set[str]:
    """Every hyphen-bearing double-quoted Go string literal under the source roots.

    A `ze.log.<subsystem>` key is only honoured if <subsystem> is a real slog
    subsystem name, and a subsystem name reaches the code as a literal in exactly
    two forms: `slogutil.Logger("bgp.filter.aspath-length")` / `LazyLogger(...)`
    for an engine logger, or a plugin `Name: "bgp-adj-rib-in"` that
    PluginLogger uses verbatim on the forked path. Collecting quoted literals
    covers both without parsing call shapes; anything not present in source
    cannot be a subsystem name at all.
    """
    found: set[str] = set()
    literal = re.compile(r'"([a-z0-9]+(?:[.-][a-z0-9]+)*-[a-z0-9]+(?:[.-][a-z0-9]+)*)"')
    for sub in GO_SOURCE_ROOTS:
        base = root / sub
        if not base.is_dir():
            continue
        for path in base.rglob("*.go"):
            try:
                text = path.read_text(encoding="utf-8", errors="ignore")
            except OSError:
                continue
            found.update(literal.findall(text))
    return found


def check_ci_log_subsystem_keys(root: Path, changed: Iterable[str]) -> int:
    """A `ze.log.<subsystem>` key in a .ci test must name a real slog subsystem.

    getLogEnv (internal/core/slogutil/slogutil.go) splits the subsystem on "."
    only, and an internal plugin's logger name is CanonicalSubsystemName of its
    registry name (internal/component/plugin/inprocess.go), which turns every
    hyphen into a dot. So `ze.log.bgp.adj-rib-in` matches no lookup for a plugin
    registered as "bgp-adj-rib-in": the real key is `ze.log.bgp.adj.rib.in`. The
    key sets nothing, the level silently stays at the WARN default, and the test
    quietly loses the log lines it was written to observe -- there is no error,
    which is why this has now recurred three times.

    Only hyphen-bearing subsystems are checked, because that is the whole failure
    mode and it keeps the check free of false positives: a hyphenated subsystem is
    legitimate only when it is declared literally in Go source (e.g.
    `slogutil.LazyLogger("bgp.filter.aspath-length")`), so an absent literal is
    proof the key is inert. Comment lines are skipped: policy-boot-apply.ci documents
    the wrong form on purpose.

    Scoped to changed .ci files like the sleep gates, but the scan is tree-wide so
    an inert key elsewhere is not laundered by an unrelated edit.
    Returns the process exit contribution (0 ok, 1 failed).
    """
    if not any(p.startswith("test/") and p.endswith(".ci") for p in changed):
        return 0
    test_dir = root / "test"
    if not test_dir.is_dir():
        return 0

    suspects: list[tuple[str, int, str, str]] = []
    for path in sorted(test_dir.rglob("*.ci")):
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError:
            continue
        rel = path.relative_to(root).as_posix()
        for i, line in enumerate(lines):
            if line.lstrip().startswith("#"):
                continue
            for match in CI_LOG_KEY_RE.finditer(line):
                subsystem = match.group(1).strip(".")
                if "-" not in subsystem:
                    continue
                suspects.append((rel, i + 1, subsystem, line.strip()))

    if not suspects:
        return 0

    declared = _hyphenated_subsystems_in_go(root)
    violations = [s for s in suspects if s[2] not in declared]
    if violations:
        print("ci log-subsystem key FAILED:")
        print("  A hyphenated ze.log.<subsystem> key only works when that exact")
        print("  subsystem is declared literally in Go. These match nothing, so the")
        print("  level silently stays at the WARN default:")
        for rel, lineno, subsystem, text in violations:
            dotted = subsystem.replace("-", ".")
            print(f"    {rel}:{lineno}: ze.log.{subsystem}  (did you mean {dotted}?)")
            print(f"      {text}")
        print("  An internal plugin's logger name is CanonicalSubsystemName of its")
        print("  registry name (plugin/inprocess.go): every hyphen becomes a dot.")
        declare_failure_group(
            "ci-log-subsystem-key",
            sorted({rel for rel, _lineno, _subsystem, _text in violations}),
            "a .ci test sets a ze.log.<subsystem> key that matches no slog subsystem",
            "make ze-doc-wiring-check",
        )
        return 1
    print(f"ci log-subsystem key OK ({len(suspects)} hyphenated key(s) declared)")
    return 0


def functional_test_advisory(changed: Iterable[str]) -> str | None:
    """Warn when user-facing code changed but no functional test did."""
    changed_list = list(changed)
    if any(p.startswith("test/") for p in changed_list):
        return None
    suites: dict[str, list[str]] = {}
    for path in changed_list:
        if not path.endswith(".go") or path.endswith("_test.go"):
            continue
        for prefix, suite in FUNCTIONAL_SUITE_BY_AREA.items():
            if path.startswith(prefix):
                suites.setdefault(suite, []).append(path)
                break
    if not suites:
        return None
    lines = ["ADVISORY: user-facing code changed without a functional-test change"]
    for suite, paths in sorted(suites.items()):
        lines.append(f"  expected coverage in {suite} for: {', '.join(sorted(paths))}")
    lines.append("  see ai/rules/testing.md")
    return "\n".join(lines)


def find_repo_root(start: Path) -> Path:
    cur = start
    while True:
        if (cur / "go.mod").exists():
            return cur
        if cur.parent == cur:
            raise SystemExit(f"could not find go.mod above {start}")
        cur = cur.parent


def normalize_changed_path(root: Path, path: str) -> str:
    p = Path(path)
    if p.is_absolute():
        try:
            return p.resolve().relative_to(root).as_posix()
        except ValueError:
            return p.as_posix()
    return p.as_posix()


def changed_files(root: Path) -> list[str]:
    files: set[str] = set()
    commands = (
        ("git", "diff", "--name-only"),
        ("git", "diff", "--cached", "--name-only"),
        ("git", "ls-files", "--others", "--exclude-standard"),
    )
    for cmd in commands:
        proc = subprocess.run(
            cmd, cwd=root, text=True, capture_output=True, check=False
        )
        if proc.returncode != 0:
            raise GateFailure(proc.stderr.strip() or f"{' '.join(cmd)} failed")
        for line in proc.stdout.splitlines():
            line = line.strip()
            if line:
                files.add(line)
    return sorted(files)


def selected_targets(root: Path, changed: Iterable[str]) -> list[str]:
    selected: set[str] = set()
    changed_list = list(changed)
    for path in changed_list:
        if is_wiring_source(path):
            selected.add("wiring")
        if is_command_source(root, path):
            selected.add("ze-command-contract-check")
        if is_command_ownership_source(path):
            selected.add("ze-command-ownership-check")
        if is_doc_source(root, path):
            selected.add("ze-doc-verify")
            selected.add("ze-doc-index-check")
        if is_discovery_source(root, path):
            selected.add("ze-discovery-index-check")
        if is_digest_source(root, path):
            selected.add("ze-digest-check")
        if is_inventory_source(root, path):
            selected.add("ze-inventory-json")
            selected.add("ze-command-list-json")
            selected.add("ze-plugin-imports-check")
        if is_templ_source(path):
            selected.add("ze-templ-output-check")
        if is_docker_exec_source(path):
            selected.add("ze-functional-docker-exec-check")
        if is_plan_source(path):
            selected.add("ze-spec-citation-check")
    return [target for target in TARGET_ORDER if target in selected]


def is_templ_source(path: str) -> bool:
    """Changed files that must re-run the templ generated-output freshness gate.

    A .templ is the source and a *_templ.go is the output, so either side moving
    can leave the pair stale. The Makefile is routed too, because the generate
    recipe and the check share one scope, and a scope edit is the change that
    would leave a whole directory ungenerated. The orphan check is routed
    because it runs inside the gate: with -keep-orphaned-files, templ reports no
    orphan, so that script is the only report of one.
    """
    return path in {
        "Makefile",
        "scripts/dev/templ_orphan_check.py",
    } or path.endswith((".templ", "_templ.go"))


def is_plan_source(path: str) -> bool:
    """Changed files that must re-run the spec citation freshness gate: any
    plan/spec-*.md (a citer or a now-removed target), any plan/learned/*.md (the
    token-drift WARN pass reads them), the checker itself, or the citation
    baseline. A spec closure removes a plan/spec-*.md, which lands here, so the
    gate fires exactly when a sibling's citation could have gone dangling."""
    if path in {
        "scripts/dev/spec-citation-check.py",
        "plan/.citation-baseline",
    }:
        return True
    if not path.endswith(".md"):
        return False
    return path.startswith("plan/spec-") or path.startswith("plan/learned/")


def is_docker_exec_source(path: str) -> bool:
    """Changed files that must re-run the fail-open call-site ratchet: any
    Python under test/ (a scenario check.py, an interop lab, the harness itself),
    the checker, or the committed floor.

    Any of them can move the count. A scenario adds a call site; a lab adds a
    WRAPPER, which enlarges the derived fail-open set and reclassifies sites in
    files nobody touched. Both land here. test/draft/ is excluded to match the
    checker's own skip: a draft is gitignored and must not redden the ratchet
    for a session that had nothing to do with it.
    """
    if path in {
        "scripts/dev/docker_exec_checked.py",
        "test/health/docker-exec-baseline.json",
    }:
        return True
    if not (path.startswith("test/") and path.endswith(".py")):
        return False
    return not path.startswith("test/draft/")



def is_wiring_source(path: str) -> bool:
    return (
        path.endswith(".go")
        and not path.endswith("_test.go")
        and (path.startswith("internal/") or path.startswith("cmd/"))
    )


def is_command_ownership_source(path: str) -> bool:
    """Changed files that must re-run the command-surface-ownership gate:
    the checker, the command registry + shim, any owner command package
    register.go, and the cmd/ze dispatch + central-registration files."""
    if path in {
        "scripts/checks/command_ownership.go",
        "cmd/ze/main.go",
    }:
        return True
    if path.startswith("internal/component/command/registry/"):
        return True
    if path.startswith("cmd/ze/internal/cmdregistry/"):
        return True
    if path.endswith("register.go") and (
        "/cli/" in path or "/client/" in path or path.startswith("cmd/ze/")
    ):
        return True
    return False


def is_command_source(root: Path, path: str) -> bool:
    if path in {
        "scripts/docvalid/commands.go",
        "scripts/inventory/commands.go",
        "internal/component/config/yang/command.go",
        "internal/component/plugin/server/command.go",
    }:
        return True
    if path.endswith("-cmd.yang"):
        return True
    if path.endswith(".yang") and file_or_head_contains(root, path, "ze:command"):
        return True
    if path.endswith(".go") and file_or_head_contains_any(
        root,
        path,
        (
            "cmdregistry.MustRegisterLocal",
            "cmdregistry.MustRegisterLocalMeta",
            "pluginserver.RegisterRPCs",
            "ze:command",
        ),
    ):
        return True
    return False


def is_doc_source(root: Path, path: str) -> bool:
    if path in {
        "scripts/docvalid/doc_drift.go",
        "scripts/docvalid/commands.go",
        "scripts/dev/code_to_docs.py",
        "ai/CODE-TO-DOCS.md",
    }:
        return True
    if path == "Makefile" or path.startswith("mk/"):
        return True
    if (path.startswith("docs/") or path == "README.md") and path.endswith(".md"):
        return file_or_head_contains(root, path, "<!-- source:")
    return False


def is_discovery_source(root: Path, path: str) -> bool:
    """Changed files that can drift a generated discovery index
    (ai/PACKAGE-MAP.md, ai/DOCS-TO-CODE.md): the generators themselves, their
    outputs, the Makefile wiring, any register.go (its Description), and any .go
    whose header carries a `// Package` or `// Design:` line that the indexes
    derive from.

    The path rules are shared with the commit gate
    (commit_helper.feeds_discovery_index) via discovery_sources.py; here the
    `// Package` / `// Design:` markers are matched against the working tree plus
    HEAD (a change either adds or removes such a header)."""
    header = ""
    if path.endswith(".go") and not path.endswith("_test.go"):
        header = (
            read_current_or_empty(root, path) + "\n" + read_head_or_empty(root, path)
        )
    return discovery_is_source(path, header)


_DIGEST_BASE_RE = re.compile(r"<!--\s*digest-base:\s*(.+?)\s*-->")
_digest_bases_cache: dict[str, list[str]] = {}


def digest_bases(root: Path) -> list[str]:
    """Subtrees the digests anchor into, read from their `digest-base` headers.
    A `.go` edit under one of these can shift the line numbers a digest cites."""
    key = str(root)
    if key in _digest_bases_cache:
        return _digest_bases_cache[key]
    bases: set[str] = set()
    ddir = root / "ai" / "digests"
    if ddir.is_dir():
        for md in ddir.glob("*.md"):
            try:
                text = md.read_text(encoding="utf-8")
            except OSError:
                continue
            for m in _DIGEST_BASE_RE.finditer(text):
                for tok in re.split(r"[,\s]+", m.group(1).strip()):
                    if tok:
                        bases.add(tok)
    result = sorted(bases)
    _digest_bases_cache[key] = result
    return result


def is_digest_source(root: Path, path: str) -> bool:
    """Changed files that must re-validate the digest anchors: a digest itself,
    the checker, or a non-test `.go` under a subtree some digest anchors into."""
    if path.startswith("ai/digests/") and path.endswith(".md"):
        return True
    if path == "scripts/dev/digest_check.py":
        return True
    if path.endswith(".go") and not path.endswith("_test.go"):
        return any(path == b or path.startswith(b + "/") for b in digest_bases(root))
    return False


def is_inventory_source(root: Path, path: str) -> bool:
    if path in {
        "Makefile",
        "mk/report-inventory.mk",
        "scripts/codegen/plugin_imports.go",
        "internal/component/plugin/all/all.go",
    }:
        return True
    if path.startswith("scripts/inventory/"):
        return True
    if path.endswith(".yang") and path.startswith("internal/"):
        return True
    if path.endswith("register.go") and path.startswith("internal/"):
        return file_or_head_contains_any(
            root,
            path,
            (
                "registry.Register",
                "MustRegister",
                "RegisterNamespace",
                "RegisterBackend",
                "yang.MustRegister",
            ),
        )
    return False


def file_or_head_contains(root: Path, path: str, needle: str) -> bool:
    return needle in read_current_or_empty(root, path) or needle in read_head_or_empty(
        root, path
    )


def file_or_head_contains_any(root: Path, path: str, needles: Iterable[str]) -> bool:
    text = read_current_or_empty(root, path)
    head = read_head_or_empty(root, path)
    return any(needle in text or needle in head for needle in needles)


def read_current_or_empty(root: Path, path: str) -> str:
    try:
        return (root / path).read_text(encoding="utf-8")
    except (FileNotFoundError, UnicodeDecodeError):
        return ""


def read_head_or_empty(root: Path, path: str) -> str:
    proc = subprocess.run(
        ("git", "show", f"HEAD:{path}"),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        return ""
    return proc.stdout


def report_wiring(root: Path, changed: Iterable[str]) -> int:
    """Print the wiring check's verdict, declare its group, and say whether it failed.

    `check_wiring` stays a pure function returning text, because `run_gate`
    decides the exit and the check's own tests read that text. Reporting lives
    here so the wiring check reaches the failure index through `run_check` like
    every other check, rather than through a path of its own.
    """
    issues = check_wiring(root, changed)
    if not issues:
        print("Wiring check PASSED")
        return 0
    print("Wiring check FAILED:")
    for issue in issues:
        print("  - " + issue)
    declare_wiring_group(root, issues)
    return 1


def declare_wiring_group(root: Path, issues: list[str]) -> None:
    """Declare the wiring failure's group, naming every file it reports.

    `check_wiring` stays a pure function returning text, because `main` decides
    the exit and its own tests read the text. Each issue is formatted below as
    `{sym.path}:{sym.line}: exported ...`, so the same `<path>:<line>:` prefix
    check_doc_links.py uses reads them back, and
    `test_the_wiring_group_names_the_file_check_wiring_reported` holds the two
    formats together.
    """
    declare_failure_group(
        "wiring",
        child_finding_paths(root, "\n".join(issues)),
        "an exported symbol added by this change has no non-test reference",
        "make ze-doc-wiring-check",
    )


def check_wiring(
    root: Path,
    changed: Iterable[str],
    baseline_reader: Callable[[str], str] | None = None,
) -> list[str]:
    if baseline_reader is None:

        def baseline_reader(path: str) -> str:
            return read_head_or_empty(root, path)

    changed = list(changed)

    # A pure relocation (rename / tier move, e.g. internal/plugins/<x> ->
    # internal/component/<x>) deletes a file and re-adds its exported symbols at a
    # new path. Collect every exported symbol REMOVED in this change -- a wiring
    # source that no longer exists on disk but did at the baseline. Those names are
    # pre-existing API, not new, so a behaviour-preserving move contributes zero
    # "added" symbols. Without this, relocating a package surfaces every unwired
    # helper it carries (consts, test-only funcs) as a false "added" symbol,
    # because the baseline at the NEW path is empty.
    relocated: set[str] = set()
    for path in changed:
        if not is_wiring_source(path):
            continue
        if read_current_or_empty(root, path):
            continue  # still on disk -> not a deletion
        for sym in exported_symbols(path, baseline_reader(path)):
            relocated.add(sym.name)

    added: list[Symbol] = []
    for path in changed:
        if not is_wiring_source(path):
            continue
        current = read_current_or_empty(root, path)
        if not current:
            continue
        old_names = {sym.name for sym in exported_symbols(path, baseline_reader(path))}
        for sym in exported_symbols(path, current):
            if sym.name in old_names or sym.name in relocated:
                continue
            if (sym.path, sym.name) in WIRING_ALLOWLIST:
                continue
            added.append(sym)

    if not added:
        return []

    issues: list[str] = []
    for sym in added:
        # The repository-wide wiring gate uses the same explicit convention:
        # an exported *ForTest helper exists for a cross-package test, not for a
        # production caller. Do not require a fake production reference here.
        if sym.name.endswith("ForTest"):
            continue
        if not has_production_reference(root, sym):
            issues.append(
                f"{sym.path}:{sym.line}: exported {sym.kind} {sym.name} has no non-test reference in internal/ or cmd/"
            )
    return issues


def exported_symbols(path: str, content: str) -> list[Symbol]:
    symbols: list[Symbol] = []
    block_kind = ""
    for line_no, line in enumerate(content.splitlines(), start=1):
        code = line.split("//", 1)[0].strip()
        if not code:
            continue

        if block_kind:
            if code.startswith(")"):
                block_kind = ""
                continue
            for name in leading_exported_idents(code):
                symbols.append(
                    Symbol(path=path, line=line_no, kind=block_kind, name=name)
                )
            continue

        block_match = BLOCK_RE.match(code)
        if block_match:
            block_kind = block_match.group(1)
            continue

        func_match = FUNC_RE.match(code)
        if func_match:
            symbols.append(
                Symbol(path=path, line=line_no, kind="func", name=func_match.group(1))
            )
            continue

        type_match = TYPE_RE.match(code)
        if type_match:
            symbols.append(
                Symbol(path=path, line=line_no, kind="type", name=type_match.group(1))
            )
            continue

        const_match = CONST_RE.match(code)
        if const_match:
            for name in leading_exported_idents(const_match.group(1)):
                symbols.append(Symbol(path=path, line=line_no, kind="const", name=name))
            continue

        var_match = VAR_RE.match(code)
        if var_match:
            for name in leading_exported_idents(var_match.group(1)):
                symbols.append(Symbol(path=path, line=line_no, kind="var", name=name))
            continue

    return symbols


def leading_exported_idents(code: str) -> list[str]:
    match = IDENT_LIST_RE.match(code)
    if not match:
        return []
    names = [part.strip() for part in match.group(1).split(",")]
    return [name for name in names if name and name[0].isupper()]


def has_production_reference(root: Path, sym: Symbol) -> bool:
    token_re = re.compile(TOKEN_TEMPLATE.format(re.escape(sym.name)))
    for base in ("internal", "cmd"):
        base_path = root / base
        if not base_path.exists():
            continue
        for path in base_path.rglob("*.go"):
            rel = path.relative_to(root).as_posix()
            if rel.endswith("_test.go"):
                continue
            try:
                lines = path.read_text(encoding="utf-8").splitlines()
            except UnicodeDecodeError:
                continue
            for line_no, line in enumerate(lines, start=1):
                if rel == sym.path and line_no == sym.line:
                    continue
                code = line.split("//", 1)[0]
                if token_re.search(code):
                    return True
    return False


def check_plugin_imports(root: Path) -> None:
    env = os.environ.copy()
    env["CGO_ENABLED"] = "0"
    proc = subprocess.run(
        ("go", "run", "scripts/codegen/plugin_imports.go", "--check"),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
        env=env,
    )
    if proc.stdout:
        print(proc.stdout, end="")
    if proc.stderr:
        print(proc.stderr, end="", file=sys.stderr)
    if proc.returncode != 0:
        raise GateFailure("plugin import check failed")


def run_make_target(make: str, target: str, root: Path) -> None:
    if target not in MAKE_TARGETS:
        raise GateFailure(f"unknown make target {target}")
    print(f"Running {target}...")
    proc = subprocess.run(
        (make, "--no-print-directory", target),
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    if proc.returncode != 0:
        if proc.stdout:
            print(proc.stdout, end="")
        if proc.stderr:
            print(proc.stderr, end="", file=sys.stderr)
        # This gate names no file here, deliberately. Every path in the text
        # above was produced by another program, and reading a delegated
        # target's output as this gate's attribution would let a red it does not
        # understand look like somebody else's. The group is declared so the red
        # is CHARGED rather than dropped; a target that wants its own files named
        # declares its own groups, which every stage may now do.
        declare_failure_group(
            target,
            [],
            f"delegated target {target} failed",
            f"make {target}",
        )
        raise GateFailure(f"{target} failed")
    print(f"{target} PASSED")


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except GateFailure as exc:
        print(str(exc), file=sys.stderr)
        raise SystemExit(1)
