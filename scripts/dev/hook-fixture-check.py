#!/usr/bin/env python3
"""Behavioural fixture tests for the three agent-guard hook fixes.

hook-parity-check.py locks the WHOLE-dispatcher exit code in a non-git temp
dir. That harness cannot exercise the three hooks this runner covers:

  * c_format_alloc (pretool-writeedit.py) is dominated by c_pre_write_go /
    c_require_design_ref on any internal/*.go file (both return 2 in a fresh
    dir), so a whole-dispatcher exit code can never isolate it. This runner
    imports c_format_alloc and asserts its return value directly.
  * validate-spec.sh only validates a plan/spec-*.md path; the assertion is
    "does not abort under set -e on ASCII-arrow specs", which needs the script
    driven over crafted spec files.
  * the commit-time gates (deferral / wiring / doc-drift / spec-audit) live in
    commit_helper.py and need a real git repository, which the parity harness
    never provides.
  * session id resolution spans TWO languages -- lib/session-id.sh writes the
    marker files pretool-writeedit.py reads -- so the invariant is agreement
    between two programs under a controlled environment, which a single-dispatcher
    exit code cannot express.

Sections come from the SECTIONS registry at the bottom of this file, and --help
derives the list from it. A hardcoded copy here drifted twice and missed half the
sections, so this file keeps no second list (ai/rules/derive-not-hardcode.md).

    python3 scripts/dev/hook-fixture-check.py                 # all sections
    python3 scripts/dev/hook-fixture-check.py --help          # list the sections
    python3 scripts/dev/hook-fixture-check.py --only validate-spec

Exit 0 = every fixture matched its expectation, 1 = a hook regressed.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

ROOT = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..")
)
HOOKS = os.path.join(ROOT, ".claude", "hooks")
DEV = os.path.abspath(os.path.dirname(__file__))

# A UUID in any version (the minted fallback is v4). Used to prove the no-source
# path resolves a per-session id, never the old shared constant.
_UUID_RE = re.compile(r"\A[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}\Z")


def _fixture_root() -> str:
    # Fixture dirs live outside /tmp and outside the repo tree (same rationale as
    # hook-parity-check.py): a /tmp path trips c_system_tmp_we / c_throwaway_tests
    # and a path inside the repo pulls fixture .go into the Go module. A dir under
    # XDG_CACHE_HOME / ~/.cache dodges both. rmtree'd per fixture after each run.
    base = os.environ.get("XDG_CACHE_HOME") or os.path.join(
        os.path.expanduser("~"), ".cache"
    )
    root = os.path.join(base, "ze-hook-fixture")
    os.makedirs(root, exist_ok=True)
    return root


class Results:
    def __init__(self) -> None:
        self.passed = 0
        self.failed = 0

    def check(self, name: str, ok: bool, detail: str = "") -> None:
        if ok:
            self.passed += 1
            print(f"  PASS  {name}")
        else:
            self.failed += 1
            print(f"  FAIL  {name}  {detail}")


# --------------------------------------------------------------------------- #
# format-alloc: import c_format_alloc and call it directly
# --------------------------------------------------------------------------- #


def _load_pretool_writeedit():
    path = os.path.join(HOOKS, "pretool-writeedit.py")
    spec = importlib.util.spec_from_file_location("pretool_writeedit", path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def run_format_alloc(results: Results) -> None:
    print("format-alloc:")
    mod = _load_pretool_writeedit()
    cfa = mod.c_format_alloc
    base = "/repo/internal/component/bgp/format/"

    def call(fp: str, content: str, tool: str = "Write"):
        return cfa({"tool": tool, "ti": {}, "fp": fp, "content": content})

    join_code = 'package format\nfunc f() string { return strings.Join(a, ",") }\n'
    builder_code = "package format\nfunc f() { var b strings.Builder; _ = b }\n"
    comment_only = (
        "package format\n"
        "// All formatters append into a caller-provided []byte. No fmt.Sprintf,\n"
        "// no strings.Builder, no strings.Join, no strings.ReplaceAll here.\n"
        "var x = 1\n"
    )

    r = call(base + "text_json.go", join_code)
    results.check("format-alloc-live-join", r is not None and r[0] == 2, repr(r))

    r = call(base + "text.go", builder_code)
    results.check("format-alloc-live-builder", r is not None and r[0] == 2, repr(r))

    r = call(base + "text.go", comment_only)
    results.check("format-alloc-comment-exempt", r is None, repr(r))

    # json.go was added to the guarded list (spec AC-1 decision).
    r = call(base + "json.go", builder_code)
    results.check("format-alloc-json-guarded", r is not None and r[0] == 2, repr(r))

    # A .go file in the same package that is NOT in the guarded list is ignored.
    r = call(base + "other.go", join_code)
    results.check("format-alloc-unguarded-file", r is None, repr(r))

    # bgp/attribute/text.go was removed from the list (package deleted in 3e66070f8).
    r = call("/repo/internal/component/bgp/attribute/text.go", join_code)
    results.check("format-alloc-stale-attribute-path", r is None, repr(r))

    # filter_format.go under reactor/ is guarded even though it is not in format/.
    r = call("/repo/internal/component/bgp/reactor/filter_format.go", join_code)
    results.check("format-alloc-reactor-filter", r is not None and r[0] == 2, repr(r))

    # Test files are never guarded.
    r = call(base + "text_json_test.go", join_code)
    results.check("format-alloc-test-file-skip", r is None, repr(r))


# --------------------------------------------------------------------------- #
# validate-spec: drive validate-spec.sh over crafted spec files
# --------------------------------------------------------------------------- #

# Sentinel: distinguishes "caller passed no payload" from "caller passed None".
_UNSET = object()

_VALID_SPEC = """# Spec: fixture

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-07-09 |

## Task

Fixture spec exercising validate-spec.sh arrow handling.

## Required Reading

- [ ] `internal/x/y.go`
  @ARROW@ Constraint: fixture only.

## Current Behavior

- [ ] `internal/x/y.go`

**Behavior to preserve:** the existing y.go output stays byte-identical.

## Data Flow

### Entry Point
- CLI command foo enters through the y.go handler.

### Transformation Path
1. Parse the input.
2. Emit the output.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| cli @ARROW@ handler | call | [ ] |

### Integration Points
- internal/x/y.go

## Wiring Test

| Entry Point | @ARROW@ | Feature Code | Test |
|-------------|---|--------------|------|
| CLI foo @ARROW@ runs | @ARROW@ | y.go handler | test/x/foo.ci |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestFoo | internal/x/y_test.go | AC-1 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| foo.ci | test/x/ | user runs foo | |

## Files to Modify

- `internal/x/y.go` - fixture feature file

## Implementation Steps

1. Implement the handler.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] make ze-test passes
"""


def _run_validate_spec(script: str, spec_text: str, *, argv=None, payload=_UNSET):
    """Drive validate-spec.sh over a crafted spec.

    argv/payload let a case bypass the normal JSON-stdin call to assert the
    absent-tool-name refusal: argv=[spec] sends NO stdin, mimicking the
    `validate-spec.sh plan/spec-foo.md` invocation that used to exit 0 without
    running a single check.
    """
    work = tempfile.mkdtemp(prefix="validate-spec-", dir=_fixture_root())
    try:
        plan = os.path.join(work, "plan")
        os.makedirs(plan, exist_ok=True)
        fp = os.path.join(plan, "spec-fixture.md")
        with open(fp, "w", encoding="utf-8") as fh:
            fh.write(spec_text)
        if payload is _UNSET:
            payload = {"tool_name": "Write", "tool_input": {"file_path": fp}}
        stdin = "" if argv else json.dumps(payload)
        env = dict(os.environ, CLAUDE_PROJECT_DIR=work)
        proc = subprocess.run(
            ["bash", script] + [a.replace("@SPEC@", fp) for a in (argv or [])],
            input=stdin,
            text=True,
            capture_output=True,
            env=env,
            timeout=30,
        )
        return proc.returncode, proc.stderr
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_validate_spec(results: Results) -> None:
    print("validate-spec:")
    script = os.path.join(HOOKS, "validate-spec.sh")

    rc, err = _run_validate_spec(script, _VALID_SPEC.replace("@ARROW@", "->"))
    results.check("validate-spec-ascii-arrows", rc == 0, f"rc={rc} err={err[:120]!r}")

    rc, err = _run_validate_spec(script, _VALID_SPEC.replace("@ARROW@", "→"))
    results.check("validate-spec-unicode-arrows", rc == 0, f"rc={rc} err={err[:120]!r}")

    malformed = _VALID_SPEC.replace("@ARROW@", "->").replace(
        "## Data Flow", "## Not Data Flow"
    )
    rc, err = _run_validate_spec(script, malformed)
    results.check("validate-spec-missing-section-blocks", rc == 2, f"rc={rc}")

    # An ABSENT tool name must not take the same path as a legitimately
    # different one. Called via argv the hook gets no stdin, so TOOL_NAME is
    # empty and the pre-fix script exited 0 -- reporting "valid" for a spec it
    # had not read. Drive a spec that is structurally INVALID: a pass here can
    # only mean no check ran. See ai/rules/fail-closed-guards.md.
    rc, err = _run_validate_spec(script, malformed, argv=["@SPEC@"])
    results.check(
        "validate-spec-argv-no-stdin-refuses",
        rc != 0 and "NOTHING WAS CHECKED" in err,
        f"rc={rc} err={err[:160]!r}",
    )

    rc, err = _run_validate_spec(script, malformed, payload={"tool_input": {}})
    results.check(
        "validate-spec-absent-tool-name-refuses",
        rc != 0 and "NOTHING WAS CHECKED" in err,
        f"rc={rc} err={err[:160]!r}",
    )

    # ...while a tool this hook does not handle stays a quiet no-op.
    rc, err = _run_validate_spec(
        script,
        malformed,
        payload={"tool_name": "Bash", "tool_input": {"command": "ls"}},
    )
    results.check(
        "validate-spec-other-tool-quiet-pass",
        rc == 0 and err == "",
        f"rc={rc} err={err[:160]!r}",
    )

    base = _VALID_SPEC.replace("@ARROW@", "->")
    _CB = "## Current Behavior\n\n- [ ] `internal/x/y.go`"

    # T-1 (AC-1): a citation carrying a line number is the form no-fabrication.md
    # mandates; the old regex required the backtick to END in the extension, so a
    # trailing :line defeated the match. Must now be ACCEPTED.
    rc, err = _run_validate_spec(
        script,
        base.replace(_CB, "## Current Behavior\n\n- [ ] `scripts/dev/foo.py:42`"),
    )
    results.check(
        "validate-spec-line-numbered-citation-accepted",
        rc == 0,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-1 (AC-2): a shell path and a Makefile path must be citable (a spec about a
    # shell hook could not cite the hook it is about under the old extension set).
    rc, err = _run_validate_spec(
        script,
        base.replace(
            _CB,
            "## Current Behavior\n\n- [ ] `.claude/hooks/foo.sh`\n- [ ] `Makefile:12`",
        ),
    )
    results.check(
        "validate-spec-shell-and-makefile-citable",
        rc == 0,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-1 (AC-3): the whole Current Behavior section is read, not a 30-line window.
    # A citation sitting past line 30 of the section was invisible to `head -30`,
    # so the check wrongly demanded source files that were in fact listed.
    long_preamble = "\n".join(f"prose context line {i}" for i in range(35))
    rc, err = _run_validate_spec(
        script,
        base.replace(
            _CB,
            "## Current Behavior\n\n" + long_preamble + "\n\n- [ ] `internal/x/y.go`",
        ),
    )
    results.check(
        "validate-spec-whole-current-behavior-read",
        rc == 0,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-1 MUST-NOT-FIRE (AC-7): prose alone is STILL rejected. Widening to accept a
    # line-numbered/shell citation must not accept a sentence with no source path.
    rc, err = _run_validate_spec(
        script,
        base.replace(
            _CB,
            "## Current Behavior\n\nWe looked at the shell hooks and reasoned about them.",
        ),
    )
    results.check(
        "validate-spec-prose-citation-still-rejected",
        rc == 2 and "list source files read" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-5 (AC-8): a tooling-only spec (no daemon Go in Files to Modify) may name a
    # concrete .py driving surface instead of a .ci, and pass. No opt-out keyword
    # is used, so this exercises the daemon-scoping path, not the keyword escape.
    tooling = (
        base.replace(
            "- `internal/x/y.go` - fixture feature file",
            "- `scripts/dev/foo.py` - fixture tooling file",
        )
        .replace(
            "| foo.ci | test/x/ | user runs foo | |",
            "| hook fixtures | `scripts/dev/hook-fixture-check.py` | fixtures drive the hook | |",
        )
        .replace(_CB, "## Current Behavior\n\n- [ ] `scripts/dev/foo.py`")
    )
    rc, err = _run_validate_spec(script, tooling)
    results.check(
        "validate-spec-tooling-surface-accepted",
        rc == 0,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-5 MUST-NOT-FIRE (AC-8): a spec that DOES touch daemon Go still owes a .ci;
    # naming only a Go unit test (no .ci, no opt-out keyword) must be REJECTED.
    daemon = base.replace(
        "| foo.ci | test/x/ | user runs foo | |",
        "| foo unit | `internal/x/y_test.go` | user runs foo | |",
    )
    rc, err = _run_validate_spec(script, daemon)
    results.check(
        "validate-spec-daemon-still-needs-ci",
        rc == 2 and "must reference .ci" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-5 MUST-NOT-FIRE (AC-8, review NIT-2): a daemon spec must NOT be able to
    # take the tooling escape by naming a .py surface -- the TOUCHES_DAEMON guard
    # blocks it. internal/x/y.go stays in Files to Modify, so it is a daemon spec.
    daemon_py = base.replace(
        "| foo.ci | test/x/ | user runs foo | |",
        "| py surface | `scripts/dev/foo.py` | user runs foo | |",
    )
    rc, err = _run_validate_spec(script, daemon_py)
    results.check(
        "validate-spec-daemon-py-surface-still-rejected",
        rc == 2 and "must reference .ci" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-1 MUST-NOT-FIRE (AC-7, review ISSUE-1): an empty-basename citation `.go`
    # is garbage, not a real source path, and must be REJECTED. The basename `+`
    # (not `*`) closes the zero-value-looks-valid hole (fail-closed-guards.md).
    rc, err = _run_validate_spec(
        script, base.replace(_CB, "## Current Behavior\n\n- [ ] `.go`")
    )
    results.check(
        "validate-spec-empty-basename-citation-rejected",
        rc == 2 and "list source files read" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # --- verification command: ONE gate, `make ze-verify` -------------------
    # The template used to ship three spellings at once and this hook demanded
    # the fuzz-inclusive `ze-test` target, which is NOT the pre-commit gate
    # (ai/rules/git-safety.md). `ze-verify` is clean; the legacy string still
    # passes (50 specs predate the change) but warns; neither is an error.
    _LEGACY_LINE = "- [ ] make ze-" + "test passes"
    _VERIFY_LINE = "- [ ] `make ze-" + "verify` passes"

    def _warn_count(stderr: str) -> int:
        """Warnings are printed as a COUNT, not a list, so the two spellings are
        told apart by the delta: the fixture carries unrelated warnings of its
        own, which is why 'no warnings at all' is the wrong discriminator."""
        m = re.search(r"Spec: (\d+) warnings", stderr)
        return int(m.group(1)) if m else 0

    rc_v, err_v = _run_validate_spec(script, base.replace(_LEGACY_LINE, _VERIFY_LINE))
    rc_l, err_l = _run_validate_spec(script, base)
    results.check(
        "validate-spec-verify-command-accepted",
        rc_v == 0 and _warn_count(err_v) == _warn_count(err_l) - 1,
        f"rc={rc_v} verify_warns={_warn_count(err_v)} legacy_warns={_warn_count(err_l)}",
    )
    results.check(
        "validate-spec-legacy-test-command-warns",
        rc_l == 0 and _warn_count(err_l) > _warn_count(err_v),
        f"rc={rc_l} err={err_l[:200]!r}",
    )
    rc, err = rc_l, err_l

    rc, err = _run_validate_spec(script, base.replace(_LEGACY_LINE, "- [ ] it builds"))
    results.check(
        "validate-spec-no-verification-command-rejected",
        rc == 2 and "verification checklist item" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # --- placeholder guards are status-aware --------------------------------
    # A `skeleton` spec is the documented shape of a deferral holder: fill Task,
    # leave the rest (ai/rules/deferral-tracking.md). Blocking its placeholders
    # made a correctly-authored skeleton un-editable. From `design` onward the
    # author IS claiming the section is written, so the same text must block.
    _placeholder = base.replace(
        "- CLI command foo enters through the y.go handler.",
        "- [Where data enters: wire bytes, API command, config, plugin message]",
    )
    rc, err = _run_validate_spec(
        script,
        _placeholder.replace("| Status | in-progress |", "| Status | skeleton |"),
    )
    results.check(
        "validate-spec-skeleton-placeholder-warns",
        rc == 0 and "warning" in err.lower(),
        f"rc={rc} err={err[:200]!r}",
    )

    rc, err = _run_validate_spec(
        script, _placeholder.replace("| Status | in-progress |", "| Status | design |")
    )
    results.check(
        "validate-spec-design-placeholder-blocks",
        rc == 2 and "Entry Point contains placeholder" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # MUST-NOT-FIRE: the guard used to match ONLY `[Format at entry]`, so the
    # template's real placeholder `[Where data enters: ...]` passed on its own.
    # This fixture carries that text alone -- it must still be caught.
    results.check(
        "validate-spec-where-data-enters-alone-caught",
        rc == 2 and "Entry Point contains placeholder" in err,
        f"rc={rc} err={err[:200]!r}",
    )


# --------------------------------------------------------------------------- #
# commit-gate: commit_helper.py creation-time gates in git fixtures
# --------------------------------------------------------------------------- #


def _load_commit_helper():
    if DEV not in sys.path:
        sys.path.insert(0, DEV)
    import commit_helper  # noqa: E402  (path set above)

    return commit_helper


def _git(repo: str, *args: str) -> None:
    subprocess.run(
        ["git", "-C", repo, *args],
        check=True,
        capture_output=True,
        text=True,
    )


def _init_repo() -> str:
    repo = tempfile.mkdtemp(prefix="commit-gate-", dir=_fixture_root())
    _git(repo, "init", "-q")
    _git(repo, "config", "user.email", "fixture@example.com")
    _git(repo, "config", "user.name", "fixture")
    _git(repo, "config", "commit.gpgsign", "false")
    with open(os.path.join(repo, "seed.txt"), "w", encoding="utf-8") as fh:
        fh.write("seed\n")
    _git(repo, "add", "seed.txt")
    _git(repo, "commit", "-q", "-m", "seed")
    return repo


def _write(repo: str, rel: str, text: str) -> None:
    full = os.path.join(repo, rel)
    os.makedirs(os.path.dirname(full), exist_ok=True)
    with open(full, "w", encoding="utf-8") as fh:
        fh.write(text)


# Six-column layout matching a real plan/deferrals/<source>.md shard (Date |
# Source | What | Reason | Destination | Status); the gate reads Destination and
# Status by index and folds over every shard under plan/deferrals/.
_DEFERRALS_HEADER = (
    "# Deferrals\n\n"
    "| Date | Source | What | Reason | Destination | Status |\n"
    "|------|--------|------|--------|-------------|--------|\n"
)


def _seed_learned_repo(repo: str, extra_generators: tuple[str, ...] = ()) -> None:
    """A fixture repo the discovery-index gate can actually run in.

    By default only ONE generator is copied (learned_index.py, plus
    discovery_sources.py which it imports): every consumer skips a generator that
    is not present, so PACKAGE-MAP and DOCS-TO-CODE stay out and the fixture needs
    no Go tree. Pass `extra_generators` when a case must distinguish "verify every
    index" from "verify the ones this commit feeds" -- one generator cannot.
    """
    gens = (
        "scripts/dev/learned_index.py",
        "scripts/dev/discovery_sources.py",
    ) + extra_generators
    for rel in gens:
        dst = os.path.join(repo, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copyfile(os.path.join(DEV, os.path.basename(rel)), dst)
    _write(repo, "plan/learned/0001-a.md", "# 0001 -- a\n")
    os.makedirs(os.path.join(repo, "ai"), exist_ok=True)
    _regen_learned_index(repo)
    _git(repo, "add", "scripts", "plan", "ai")
    _git(repo, "commit", "-q", "-m", "seed learned index")


def _regen(repo: str, generator: str) -> None:
    subprocess.run(
        [sys.executable, os.path.join(repo, "scripts/dev", generator)],
        check=True,
        capture_output=True,
        text=True,
    )


def _regen_learned_index(repo: str) -> None:
    _regen(repo, "learned_index.py")


def run_commit_gate(results: Results) -> None:
    print("commit-gate:")
    import contextlib
    import io
    from pathlib import Path

    ch = _load_commit_helper()

    # --- deferral-unassigned (block) --- the gate folds over plan/deferrals/*.md
    repo = _init_repo()
    try:
        _write(
            repo,
            "plan/deferrals/abc.md",
            _DEFERRALS_HEADER + "| 2026-07-09 | abc | thing | reason |  | open |\n",
        )
        problems = ch.deferral_unassigned_problems(Path(repo))
        results.check("commit-gate-deferral-unassigned", bool(problems), repr(problems))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # An assigned destination passes only when the spec it names EXISTS. Both
    # spellings resolve to the same file, across shards (ai/rules/deferral-tracking.md).
    repo = _init_repo()
    try:
        _write(repo, "plan/spec-foo.md", "# Spec: foo\n")
        _write(
            repo,
            "plan/deferrals/foo.md",
            _DEFERRALS_HEADER
            + "| 2026-07-09 | abc | thing | reason | spec-foo.md | open |\n"
            + "| 2026-07-09 | abc | thing2 | reason | `plan/spec-foo.md` | open |\n",
        )
        problems = ch.deferral_unassigned_problems(Path(repo))
        results.check("commit-gate-deferral-assigned-ok", not problems, repr(problems))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # A destination naming a spec nobody created loses the work exactly as a
    # prose destination does, so it must block too -- even when it lives in a shard.
    repo = _init_repo()
    try:
        _write(
            repo,
            "plan/deferrals/orphan.md",
            _DEFERRALS_HEADER
            + "| 2026-07-09 | abc | thing | reason | spec-never-written.md | open |\n",
        )
        problems = ch.deferral_unassigned_problems(Path(repo))
        results.check(
            "commit-gate-deferral-assigned-missing-blocks",
            bool(problems),
            repr(problems),
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- deferral-in-diff (block) ---
    repo = _init_repo()
    try:
        _write(repo, "docs/notes.md", "# notes\n\nThis is out of scope for now.\n")
        problems = ch.deferral_in_diff_problems(Path(repo), ("docs/notes.md",), ())
        results.check("commit-gate-deferral-in-diff", bool(problems), repr(problems))
        # a plan/deferrals/ shard included in the commit clears it
        _write(repo, "plan/deferrals/notes.md", _DEFERRALS_HEADER)
        problems = ch.deferral_in_diff_problems(
            Path(repo), ("docs/notes.md", "plan/deferrals/notes.md"), ()
        )
        results.check(
            "commit-gate-deferral-in-diff-logged-ok", not problems, repr(problems)
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # A bare quoted-string literal (the DEFERRAL_PATTERNS definition shape) is
    # exempt, so committing the gate's own list / rule docs does not self-trip;
    # prose in the same file still trips.
    repo = _init_repo()
    try:
        _write(
            repo,
            "scripts/dev/patterns.py",
            'PATTERNS = (\n    "out of scope",\n    "future work",\n)\n',
        )
        problems = ch.deferral_in_diff_problems(
            Path(repo), ("scripts/dev/patterns.py",), ()
        )
        results.check(
            "commit-gate-deferral-in-diff-code-literal-exempt",
            not problems,
            repr(problems),
        )
        _write(
            repo,
            "scripts/dev/patterns.py",
            'PATTERNS = ("out of scope",)\n# we will handle later in prose\n',
        )
        problems = ch.deferral_in_diff_problems(
            Path(repo), ("scripts/dev/patterns.py",), ()
        )
        results.check(
            "commit-gate-deferral-in-diff-prose-still-caught",
            bool(problems),
            repr(problems),
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- wiring-at-commit (warn) ---
    repo = _init_repo()
    try:
        warns = ch.wiring_warnings(("internal/plugins/foo/foo.go",))
        results.check("commit-gate-wiring-warn", bool(warns), repr(warns))
        warns = ch.wiring_warnings(("internal/plugins/foo/foo.go", "test/foo/foo.ci"))
        results.check("commit-gate-wiring-with-ci-ok", not warns, repr(warns))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- doc-drift (warn) ---
    repo = _init_repo()
    try:
        warns = ch.doc_drift_warnings(Path(repo))
        results.check("commit-gate-doc-drift-absent-skips", not warns, repr(warns))
        if shutil.which("go"):
            _write(repo, "go.mod", "module fixture\n\ngo 1.21\n")
            _write(
                repo,
                "scripts/docvalid/doc_drift.go",
                'package main\n\nimport (\n\t"fmt"\n\t"os"\n)\n\n'
                'func main() {\n\tfmt.Println("docs drifted: foo")\n\tos.Exit(1)\n}\n',
            )
            warns = ch.doc_drift_warnings(Path(repo))
            results.check("commit-gate-doc-drift-warns", bool(warns), repr(warns))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- spec-audit (block on unfilled Pre-Commit Verification) ---
    empty_pcv = (
        "# Spec: fixture\n\n"
        "## Pre-Commit Verification\n\n"
        "### Files Exist (ls)\n"
        "| File | Exists | Evidence |\n"
        "|------|--------|----------|\n\n"
        "## Checklist\n"
    )
    filled_pcv = (
        "# Spec: fixture\n\n"
        "## Pre-Commit Verification\n\n"
        "### Files Exist (ls)\n"
        "| File | Exists | Evidence |\n"
        "|------|--------|----------|\n"
        "| internal/x/y.go | yes | ls output |\n\n"
        "## Checklist\n"
    )
    repo = _init_repo()
    try:
        _write(repo, "plan/spec-fixture.md", empty_pcv)
        _write(repo, "plan/learned/099-fixture.md", "# fixture\n")
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/learned/099-fixture.md",), "spec-fixture.md"
        )
        results.check("commit-gate-spec-audit-blocks", bool(problems), repr(problems))

        _write(repo, "plan/spec-fixture.md", filled_pcv)
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/learned/099-fixture.md",), "spec-fixture.md"
        )
        results.check("commit-gate-spec-audit-filled-ok", not problems, repr(problems))

        # No spec claimed -> the gate skips entirely.
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/learned/099-fixture.md",), ""
        )
        results.check(
            "commit-gate-spec-audit-no-claim-skips", not problems, repr(problems)
        )

        # A commit that does NOT add this spec's learned summary is not a closure
        # commit, so the gate does not fire even with an unfilled section.
        _write(repo, "plan/spec-fixture.md", empty_pcv)
        problems = ch.spec_audit_problems(
            Path(repo), ("internal/x/y.go",), "spec-fixture.md"
        )
        results.check(
            "commit-gate-spec-audit-non-closure-skips", not problems, repr(problems)
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- real path: commit_helper create blocks on a deferral diff (exit 2) ---
    repo = _init_repo()
    try:
        _write(
            repo, "docs/notes.md", "# notes\n\nWe will handle later, out of scope.\n"
        )
        # create() prints its UsageError to stderr; capture it so a passing run
        # is not polluted by the (expected) block message.
        with contextlib.redirect_stderr(io.StringIO()):
            rc = ch.main(
                [
                    "--repo",
                    repo,
                    "create",
                    "--session",
                    "abcd1234",
                    "--subject",
                    "fixture commit",
                    "--file",
                    "docs/notes.md",
                    "--lesson-not-needed",
                    "fixture integration test for the deferral gate",
                ]
            )
        script_exists = os.path.isfile(os.path.join(repo, "tmp", "commit-abcd1234.sh"))
        results.check(
            "commit-gate-create-blocks-deferral",
            rc == 2 and not script_exists,
            f"rc={rc} script={script_exists}",
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: own staleness blocks, a concurrent session's does not ---
    # The gate judges the tree the commit PRODUCES (HEAD + adds - removes), not the
    # working tree, so an untracked summary belonging to another session cannot
    # force a commit to either block or cross-commit that session's index row.
    repo = _init_repo()
    try:
        _seed_learned_repo(repo)
        # The cases below run in sequence against ONE repo and each depends on the
        # state the previous one left. The order is load-bearing; inserting a case
        # changes what the ones after it test.
        #
        # A: this commit adds a summary and omits the regenerated index -> block.
        # Asserts the MESSAGE, not merely that something blocked: the pre-change
        # implementation also blocked here, by a different branch.
        _write(repo, "plan/learned/0002-b.md", "# 0002 -- b\n")
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), ("plan/learned/0002-b.md",)
            )
        results.check(
            "commit-gate-index-own-staleness-blocks",
            bool(problems) and "omitted:" in "".join(problems),
            repr(problems),
        )

        # D (runs here deliberately): at THIS state the working tree is stale from
        # the summary A left, so an unrelated commit is the case that proves a
        # concurrent session's staleness does not block. Run after B or C, where
        # the tree is fresh again, it would assert on a branch that returns early
        # and would pass with the whole change reverted.
        _write(repo, "docs/unrelated.md", "# unrelated\n")
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(Path(repo), ("docs/unrelated.md",))
        results.check(
            "commit-gate-index-unrelated-commit-passes", not problems, repr(problems)
        )

        # B: same commit, index regenerated to match HEAD + its own summary, while a
        # concurrent session leaves an UNTRACKED summary in the tree -> no block.
        _regen_learned_index(repo)
        _write(repo, "plan/learned/0003-foreign.md", "# 0003 -- foreign\n")
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo),
                ("plan/learned/0002-b.md", "ai/LEARNED-FULL-INDEX.md"),
            )
        results.check(
            "commit-gate-index-foreign-staleness-passes", not problems, repr(problems)
        )

        # C: the index is regenerated WITH the concurrent session's untracked
        # summary and then committed -> it would publish a row for a file absent
        # from HEAD. This is how plan/learned/1282-*.md reached HEAD's committed
        # index, and a working-tree check calls this state "fresh".
        _regen_learned_index(repo)
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo),
                ("plan/learned/0002-b.md", "ai/LEARNED-FULL-INDEX.md"),
            )
        results.check(
            "commit-gate-index-foreign-row-included-blocks",
            bool(problems) and "included but wrong" in "".join(problems),
            repr(problems),
        )

    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: an index the commit does not visibly FEED is still
    # verified. `indexes_fed_by` recognises a PACKAGE-MAP source by a `// Package`
    # header or a register.go name, but package_map keys its rows on DIRECTORY
    # existence, so a new .go carrying only `// Design:` drifts PACKAGE-MAP while
    # feeding DOCS-TO-CODE alone. Needs TWO generators: with one, "verify every
    # index" and "verify the fed ones" are indistinguishable.
    repo = _init_repo()
    try:
        _seed_learned_repo(repo, extra_generators=("scripts/dev/package_map.py",))
        _write(
            repo,
            "internal/existing/a.go",
            "// Package existing does a thing.\npackage existing\n",
        )
        _regen(repo, "package_map.py")
        _git(repo, "add", "internal", "ai")
        _git(repo, "commit", "-q", "-m", "seed package map")

        # The new file feeds DOCS-TO-CODE only (no `// Package`), yet it adds a
        # PACKAGE-MAP row. The author regenerated PACKAGE-MAP but did not --file it,
        # so the working tree is FRESH and only the commit view can see the drift.
        _write(
            repo,
            "internal/newpkg/thing.go",
            "// Design: docs/x.md -- thing\npackage newpkg\n",
        )
        _regen(repo, "package_map.py")
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), ("internal/newpkg/thing.go",)
            )
        results.check(
            "commit-gate-index-unfed-index-still-verified",
            bool(problems) and "PACKAGE-MAP" in "".join(problems),
            repr(problems),
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: a REMOVAL drifts the index too (fresh repo) ---
    # `--remove` is how a spec closes (commit B) and how a package is deleted. If
    # the view does not apply removals it keeps a file HEAD will not have, the
    # generator calls the index coherent, and a stale index ships.
    repo = _init_repo()
    try:
        _seed_learned_repo(repo)
        _write(repo, "plan/learned/0002-b.md", "# 0002 -- b\n")
        _regen_learned_index(repo)
        _git(repo, "add", "plan", "ai")
        _git(repo, "commit", "-q", "-m", "add 0002")
        os.remove(os.path.join(repo, "plan/learned/0002-b.md"))

        # E1: removal committed, index left listing the removed summary -> block.
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), (), ("plan/learned/0002-b.md",)
            )
        results.check(
            "commit-gate-index-removal-stale-blocks", bool(problems), repr(problems)
        )

        # E2: same removal with the regenerated index riding along -> passes.
        _regen_learned_index(repo)
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), ("ai/LEARNED-FULL-INDEX.md",), ("plan/learned/0002-b.md",)
            )
        results.check(
            "commit-gate-index-removal-regenerated-passes", not problems, repr(problems)
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: the ENTRY POINT passes remove_paths through ---
    # E1/E2 above call discovery_index_problems directly, so dropping the argument
    # at create()'s call site leaves them green. Drive the guard from where a user
    # reaches it (ai/rules/fail-closed-guards.md).
    repo = _init_repo()
    try:
        _seed_learned_repo(repo)
        _write(repo, "plan/learned/0002-b.md", "# 0002 -- b\n")
        _regen_learned_index(repo)
        _git(repo, "add", "plan", "ai")
        _git(repo, "commit", "-q", "-m", "add 0002")
        os.remove(os.path.join(repo, "plan/learned/0002-b.md"))
        with contextlib.redirect_stderr(io.StringIO()):
            rc = ch.main(
                [
                    "--repo",
                    repo,
                    "create",
                    "--session",
                    "beef1234",
                    "--subject",
                    "remove a summary without refreshing the index",
                    "--remove",
                    "plan/learned/0002-b.md",
                    "--lesson-not-needed",
                    "fixture for the removal path of the discovery-index gate",
                ]
            )
        script_exists = os.path.isfile(os.path.join(repo, "tmp", "commit-beef1234.sh"))
        results.check(
            "commit-gate-index-removal-blocks-via-create",
            rc == 2 and not script_exists,
            f"rc={rc} script={script_exists}",
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)


# --------------------------------------------------------------------------- #
# session-id: lib/session-id.sh (writer) vs pretool-writeedit.py (reader) parity
# --------------------------------------------------------------------------- #


def _sid_bash(env: dict) -> str:
    """_session_id from lib/session-id.sh under a given environment."""
    r = subprocess.run(
        ["bash", "-c", "source .claude/hooks/lib/session-id.sh; _session_id"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env=env,
    )
    return r.stdout.strip()


def _sid_python(env: dict) -> str:
    """session_id() from pretool-writeedit.py under a given environment.

    Run in a subprocess, not via _load_pretool_writeedit(): session_id() reads
    os.environ and walks the process tree, so an in-process import would see THIS
    runner's environment and pid instead of the fixture's.
    """
    code = (
        "import importlib.util,sys;"
        "spec=importlib.util.spec_from_file_location('m',sys.argv[1]);"
        "m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);"
        "print(m.session_id())"
    )
    r = subprocess.run(
        [sys.executable, "-c", code, os.path.join(HOOKS, "pretool-writeedit.py")],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env=env,
    )
    return r.stdout.strip()


def _sid_commit_helper(env: dict) -> str:
    """commit_helper.claude_session_fingerprint() under a given environment.

    Registered in sys.modules before exec so its frozen dataclasses can resolve
    their own module during introspection; scripts/dev on the path so its sibling
    imports (discovery_sources) resolve.
    """
    code = (
        "import sys, importlib.util;"
        "sys.path.insert(0, sys.argv[1]);"
        "spec=importlib.util.spec_from_file_location('commit_helper', sys.argv[2]);"
        "m=importlib.util.module_from_spec(spec);"
        "sys.modules['commit_helper']=m;"
        "spec.loader.exec_module(m);"
        "print(m.claude_session_fingerprint())"
    )
    r = subprocess.run(
        [sys.executable, "-c", code, DEV, os.path.join(DEV, "commit_helper.py")],
        cwd=ROOT,
        capture_output=True,
        text=True,
        env=env,
    )
    return r.stdout.strip()


def _load_session_id_module():
    """Import lib/session_id.py in-process, for testing the minting internals."""
    spec = importlib.util.spec_from_file_location(
        "ze_session_id_test", os.path.join(HOOKS, "lib", "session_id.py")
    )
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    return m


def _grep_lines(pattern: str, *paths: str) -> list[str]:
    r = subprocess.run(
        # Exclude this harness: it names the very symbols it greps for (as string
        # literals in the checks below), which would self-match every scan.
        [
            "grep",
            "-rn",
            "--include=*.py",
            "--include=*.sh",
            "--exclude=hook-fixture-check.py",
            "--",
            pattern,
            *paths,
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    return [ln for ln in r.stdout.splitlines() if ln.strip()]


def _run_session_id_mint(results: Results) -> None:
    """AC-10/AC-11: the minted fallback is per-key UNIQUE and per-key STABLE.

    Tested at the mint primitive (_mint_cached) with explicit cache dir + cache key,
    so the real uniqueness axis (same dir, different CLI-ancestor PID) is exercised
    without process-tree games -- the axis a project-dir-only test would mask."""
    mod = _load_session_id_module()
    cache = tempfile.mkdtemp(prefix="ze-sid-mint-", dir=_fixture_root())
    try:
        a1 = mod._mint_cached(cache, 111)
        a2 = mod._mint_cached(cache, 111)  # same key -> stable
        b1 = mod._mint_cached(cache, 222)  # distinct key -> unique
        results.check(
            "session-id-mint-stable",
            a1 == a2 and _UUID_RE.match(a1) is not None,
            f"a1={a1!r} a2={a2!r}",
        )
        results.check(
            "session-id-mint-unique",
            a1 != b1 and _UUID_RE.match(b1) is not None,
            f"a1={a1!r} b1={b1!r}",
        )
        results.check(
            "session-id-mint-not-constant",
            "claude-session-fallback" not in (a1, b1),
            f"a1={a1!r} b1={b1!r}",
        )
        # ISSUE-1 regression: a POISONED (empty) cache file -- what a crash between an
        # O_EXCL create and the separate write leaves behind -- MUST be treated as a
        # miss and overwritten with a full id, then stay stable. The pre-fix code read
        # "" from the empty file, hit FileExistsError on the O_EXCL create, and returned
        # a FRESH uuid on every call (never healed until 24h cleanup): the session
        # stopped matching its own markers and gates re-blocked already-done work.
        empty = os.path.join(cache, ".sid-by-pid-333")
        with open(empty, "w"):
            pass  # zero bytes, exactly what a crashed O_EXCL create leaves
        c1 = mod._mint_cached(cache, 333)
        c2 = mod._mint_cached(cache, 333)
        results.check(
            "session-id-mint-heals-empty-cache",
            c1 == c2
            and _UUID_RE.match(c1) is not None
            and c1 != "claude-session-fallback",
            f"c1={c1!r} c2={c2!r}",
        )
    finally:
        shutil.rmtree(cache, ignore_errors=True)


def _run_session_id_cleanup_reparse(results: Results) -> None:
    """R-11: _cleanup_stale_markers must recover a FULL UUID sid, not its last group.

    A live session's state file (its .session-<sid> marker exists) that happens to be
    older than the 24h threshold must SURVIVE. The pre-fix `${fname##*-}` mangled the
    UUID, looked for a marker that never existed, called the live file an orphan, and
    deleted it."""
    work = tempfile.mkdtemp(prefix="ze-sid-cleanup-", dir=_fixture_root())
    try:
        sess = os.path.join(work, "tmp", "session")
        os.makedirs(sess)
        sid = "8d3d7c6b-fbad-4077-8f06-4678828041d0"
        state = os.path.join(sess, f"session-state-spec-vrrp-4-transport-{sid}.md")
        with open(state, "w") as fh:
            fh.write("spec-vrrp-4-transport\n")
        with open(os.path.join(sess, f".session-{sid}"), "w") as fh:
            fh.write("spec-vrrp-4-transport\n")
        old = time.time() - 26 * 3600  # older than the 1440-min cleanup threshold
        os.utime(state, (old, old))
        subprocess.run(
            [
                "bash",
                "-c",
                'source "$1"; _cleanup_stale_markers',
                "_",
                os.path.join(HOOKS, "lib", "state-file.sh"),
            ],
            cwd=work,
            capture_output=True,
            text=True,
        )
        results.check(
            "session-id-cleanup-keeps-live-uuid-state",
            os.path.isfile(state),
            "live UUID state file wrongly deleted (sid mis-parse)",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_session_id(results: Results) -> None:
    """Lock the shell writer and the Python reader to ONE session id.

    These two ends key the same marker files (.lsp-invoked-<sid>,
    .source-read-<sid>, .session-<sid>, session-state-<stem>-<sid>.md): the shell
    hooks WRITE them, pretool-writeedit.py READS them. Any disagreement fails
    CLOSED -- the reader looks for a file nothing wrote and blocks work that was in
    fact done (real incident, 2026-07-16; see session_id.__doc__).

    Only pretool-writeedit.py's docstring asserted the invariant, and prose does
    not fail a build. This is the executable form.
    """
    print("session-id:")
    base = {k: v for k, v in os.environ.items() if k != "CLAUDE_CODE_SESSION_ID"}
    base.pop("CLAUDE_CODE_SESSION_ACCESS_TOKEN", None)

    def env_with(sid):
        e = dict(base)
        if sid is not None:
            e["CLAUDE_CODE_SESSION_ID"] = sid
        return e

    # The exported session UUID wins, and both ends read it identically.
    e = env_with("11111111-2222-3333-4444-555555555555")
    b, p = _sid_bash(e), _sid_python(e)
    results.check(
        "session-id-env-parity",
        b == p == "11111111-2222-3333-4444-555555555555",
        f"bash={b!r} py={p!r}",
    )

    # Distinct sessions MUST NOT collide -- this is the bug the env lookup fixes:
    # with no id source, every concurrent session shared one marker set and
    # `spec-session.sh claim` silently overwrote another session's claim.
    e2 = env_with("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
    b2 = _sid_bash(e2)
    results.check(
        "session-id-distinct-sessions-differ",
        b2 != b and b2 == "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        f"first={b!r} second={b2!r}",
    )

    # An id unusable as a filename component is REJECTED, not rewritten: both ends
    # must fall through together, or they would disagree on the marker path. A
    # dedicated project dir keeps the fall-through's minted cache out of the live
    # tmp/session/.
    reject_proj = tempfile.mkdtemp(prefix="ze-sid-reject-", dir=_fixture_root())
    try:
        for label, bad in (
            ("traversal", "../../../etc/passwd"),
            ("slash", "a/b"),
            ("space", "has space"),
            ("empty", ""),
        ):
            e3 = env_with(bad)
            e3["CLAUDE_PROJECT_DIR"] = reject_proj
            b3, p3 = _sid_bash(e3), _sid_python(e3)
            results.check(
                f"session-id-rejects-{label}",
                b3 == p3 and "/" not in b3 and b3 != "" and b3 != bad,
                f"bash={b3!r} py={p3!r}",
            )
    finally:
        shutil.rmtree(reject_proj, ignore_errors=True)

    # With no id source at all, the resolver MUST NOT collapse onto a shared
    # constant (the collision this spec fixes, AC-10/AC-11): it mints a per-session
    # UUID cached by the CLI-ancestor PID, stable across hook subprocesses. Both ends
    # resolve the SAME id -- one CLI ancestor, one project dir, so bash mints and
    # python reads the same cache. A dedicated project dir keeps the minted cache out
    # of the live repo and makes the assertion deterministic.
    proj = tempfile.mkdtemp(prefix="ze-sid-nosrc-", dir=_fixture_root())
    try:
        e4 = env_with(None)
        e4["CLAUDE_PROJECT_DIR"] = proj
        b4, p4 = _sid_bash(e4), _sid_python(e4)
        results.check(
            "session-id-no-source-parity",
            b4 == p4
            and b4 != ""
            and b4 != "claude-session-fallback"
            and _UUID_RE.match(b4) is not None,
            f"bash={b4!r} py={p4!r}",
        )
    finally:
        shutil.rmtree(proj, ignore_errors=True)

    # AC-9: the THIRD derivation is gone -- commit_helper keys on the SAME id, and
    # the env source dominates all three ends.
    e5 = env_with("cccccccc-1111-2222-3333-444444444444")
    ch = _sid_commit_helper(e5)
    results.check(
        "session-id-commit-helper-agrees",
        ch == _sid_bash(e5) == "cccccccc-1111-2222-3333-444444444444",
        f"commit_helper={ch!r}",
    )

    # AC-9: exactly ONE derivation survives. The independent copies' signature tokens
    # are gone from every consumer; only session_id.py carries the resolution logic.
    fallback_const = _grep_lines("SESSION_ID_FALLBACK", ".claude/hooks", "scripts/dev")
    results.check(
        "session-id-no-python-fallback-constant",
        not fallback_const,
        "; ".join(fallback_const),
    )
    psfield = _grep_lines("_ps_field", "scripts/dev/commit_helper.py")
    results.check(
        "session-id-commit-helper-walk-removed", not psfield, "; ".join(psfield)
    )
    argv_files = {
        ln.split(":", 1)[0]
        for ln in _grep_lines("_session_id_from_argv", ".claude/hooks")
    }
    results.check(
        "session-id-one-argv-walk",
        argv_files == {".claude/hooks/lib/session_id.py"},
        f"files={sorted(argv_files)}",
    )

    _run_session_id_mint(results)
    _run_session_id_cleanup_reparse(results)


# --------------------------------------------------------------------------- #


def run_rfc_test_guard(results: Results) -> None:
    """The RFC-tagged test guard (plan/spec-rfc-requirement-coverage.md).

    An RFC-tagged test is the proof behind a public compliance claim, so editing it to
    match the code retires the claim's evidence while the claim stays up. The golden
    exit-code table cannot isolate this: it depends on the CONTENT of both sides of the
    edit, which only a fixture can supply.
    """
    print("rfc-test-guard:")
    mod = _load_pretool_writeedit()
    cw = mod.c_test_weakening
    fp = "/repo/internal/component/bgp/message/rfc7606_test.go"

    tagged = (
        "// RFC requirement: RFC7606-7.1-1 negative - ORIGIN len != 1 withdraws.\n"
        "func TestX(t *testing.T) {\n"
        "\trequire.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)\n"
        "}\n"
    )

    def edit(old: str, new: str, path: str = fp):
        return cw(
            {"tool": "Edit", "ti": {"old_string": old, "new_string": new}, "fp": path}
        )

    # The core case: "fix" the failing test instead of the code.
    r = edit(tagged, tagged.replace("TreatAsWithdraw", "None"))
    results.check(
        "rfc-guard-blocks-expectation-swap", r is not None and r[0] == 2, repr(r)
    )

    # test-relax is SELF-SERVICE. It must not buy a pass here: an agent writing its own
    # justification is not the user's approval. This is the loophole the guard closes.
    r = edit(
        tagged,
        tagged.replace(
            "\trequire.Equal", "\t// test-relax: no longer applies\n\trequire.NotEqual"
        ),
    )
    results.check(
        "rfc-guard-relax-token-insufficient", r is not None and r[0] == 2, repr(r)
    )

    # Deleting the assertion entirely.
    r = edit(
        tagged,
        "// RFC requirement: RFC7606-7.1-1 negative - x.\nfunc TestX(t *testing.T) {\n}\n",
    )
    results.check(
        "rfc-guard-blocks-assertion-delete", r is not None and r[0] == 2, repr(r)
    )

    # Must NOT over-block ordinary maintenance, or the hook gets disabled and protects
    # nothing (spec risk R-8).
    r = edit(tagged, tagged.replace("\t", "    "))
    results.check("rfc-guard-allows-reformat", r is None, repr(r))

    r = edit(
        tagged,
        tagged.replace("ORIGIN len != 1 withdraws", "malformed ORIGIN is withdrawn"),
    )
    results.check("rfc-guard-allows-comment-edit", r is None, repr(r))

    # The user approved: recorded, auditable, allowed.
    approved = tagged.replace(
        "func TestX",
        "// rfc-test-change-approved: 2026-07-17 user agreed 3(j) mandates reset\nfunc TestX",
    ).replace("TreatAsWithdraw", "SessionReset")
    results.check("rfc-guard-allows-user-approved", edit(tagged, approved) is None, "")

    # An untagged test keeps the old behavior exactly: this guard adds a rule, it does not
    # replace c_test_weakening's heuristic.
    untagged = tagged.split("\n", 1)[1]
    r = edit(untagged, untagged.replace("require.Equal", "require.NotEqual"))
    results.check("rfc-guard-untagged-unaffected", r is None, repr(r))

    # A .ci test is tagged with '#', and the guard must see it there too.
    ci = (
        "# RFC requirement: RFC7606-3.a-1 negative - NOTIFICATION on reset.\n"
        "expect=bgp:conn=1:seq=1:hex=FFFF\n"
    )
    r = edit(ci, ci.replace("FFFF", "DEAD"), "/repo/test/plugin/rfc7606-reset.ci")
    results.check("rfc-guard-covers-ci", r is not None and r[0] == 2, repr(r))

    _run_rfc_guard_enclosing_scope(results, cw)


def _run_rfc_guard_enclosing_scope(results: Results, cw) -> None:
    """The guard must see a tag that the EDIT HUNK does not contain.

    An Edit replaces one hunk, and that hunk is all c_test_weakening used to be given. A
    tag sits on the line above the function, or on a sibling table case -- so editing the
    BODY of a tagged test slipped past the one guard written to stop exactly that
    (spec-rfc-gate-regression-ratchets.md G1/AC-1). Widening needs the file on disk, so
    these cases write real files rather than the /repo/ paths above.
    """
    tmp = tempfile.mkdtemp(prefix="ze-rfc-guard-")
    try:
        _rfc_guard_scope_cases(results, cw, tmp)
    finally:
        # try/finally, not a trailing call: a failing check must not leak the temp dir.
        shutil.rmtree(tmp, ignore_errors=True)


def _rfc_guard_scope_cases(results: Results, cw, tmp: str) -> None:
    fp = os.path.join(tmp, "rfc7606_test.go")

    body = "\trequire.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)\n"
    other = "\trequire.Equal(t, 1, other)\n"
    tag = "// RFC requirement: RFC7606-7.1-1 negative - ORIGIN len != 1 withdraws.\n"
    # The UNTAGGED helper is deliberately FIRST, directly above the tagged test's doc
    # comment. That ordering is the whole point: a scope that ran to the next `func`
    # keyword instead of the next func's DOC COMMENT swallowed the tag below and blocked
    # this helper. With the tagged function first, the bug is invisible.
    tagged_file = (
        "package message\n"
        "\n"
        "func helperUntagged(t *testing.T) {\n" + other + "}\n"
        "\n"
        "// TestTagged proves the withdraw path.\n"
        + tag
        + "func TestTagged(t *testing.T) {\n"
        + body
        + "}\n"
    )
    with open(fp, "w", encoding="utf-8") as fh:
        fh.write(tagged_file)

    def edit(old: str, new: str, path: str = fp, **extra):
        ti = {"old_string": old, "new_string": new}
        ti.update(extra)
        return cw({"tool": "Edit", "ti": ti, "fp": path})

    def blocked_on_rfc(r):
        """Exit 2 alone is not proof: the generic test-weakening path returns 2 as well.
        Only the RFC message proves WHICH guard fired."""
        return r is not None and r[0] == 2 and "RFC-tagged test" in r[1]

    # AC-1: the hunk is the assertion alone. The tag is two lines above it.
    r = edit(body, body.replace("TreatAsWithdraw", "None"))
    results.check(
        "rfc-guard-blocks-body-edit-tag-outside-hunk", blocked_on_rfc(r), repr(r)
    )

    # AC-2: the helper directly above a tagged test carries no tag of its own. Measured at
    # 331 of 3220 untagged functions falsely blocked before the boundary was fixed.
    r = edit(other, other.replace("require.Equal", "require.NotEqual"))
    results.check("rfc-guard-untagged-func-in-tagged-file-passes", r is None, repr(r))

    # AC-3: widening must not turn ordinary maintenance into a block.
    r = edit(body, body.replace("\t", "    "))
    results.check("rfc-guard-body-edit-reformat-passes", r is None, repr(r))

    r = edit(body, body + "\t// explain the expectation\n")
    results.check("rfc-guard-body-edit-comment-only-passes", r is None, repr(r))

    # AC-4: the approval token still works when the tag is out of hunk.
    r = edit(
        body,
        "\t// rfc-test-change-approved: 2026-07-20 user confirmed\n"
        + body.replace("TreatAsWithdraw", "None"),
    )
    results.check("rfc-guard-body-edit-approved-passes", r is None, repr(r))

    # Deleting the TAG is not a comment edit. Left unguarded it is the cheapest retirement
    # of a compliance claim there is: drop the marker, then `// test-relax:` buys every
    # later weakening on its own.
    r = edit(tag, "// test-relax: obsolete\n")
    results.check("rfc-guard-blocks-tag-deletion", blocked_on_rfc(r), repr(r))

    # replace_all rewrites EVERY occurrence. Inspecting only the first would let an edit
    # aimed at the untagged helper gut the identical assertion inside the tagged test.
    dup = os.path.join(tmp, "dup_test.go")
    with open(dup, "w", encoding="utf-8") as fh:
        fh.write(
            "package message\n"
            "\n"
            "func helperFirst(t *testing.T) {\n" + other + "}\n"
            "\n" + tag + "func TestTaggedDup(t *testing.T) {\n" + other + "}\n"
        )
    r = edit(other, "\trequire.NotNil(t, other)\n", dup, replace_all=True)
    results.check(
        "rfc-guard-replace-all-reaches-tagged-copy", blocked_on_rfc(r), repr(r)
    )

    # A tag that no function scope covers (a hoisted table) must widen to the whole file
    # rather than leave a silent hole: the gate credits a tag ANYWHERE in the file.
    hoisted = os.path.join(tmp, "hoisted_test.go")
    with open(hoisted, "w", encoding="utf-8") as fh:
        fh.write(
            "package message\n"
            "\n"
            "var cases = []tc{\n"
            "\t" + tag + '\t{name: "withdraw"},\n'
            "}\n"
            "\n"
            "func TestRunner(t *testing.T) {\n" + other + "}\n"
        )
    r = edit(other, other.replace("require.Equal", "require.NotEqual"), hoisted)
    results.check("rfc-guard-hoisted-tag-widens-to-file", blocked_on_rfc(r), repr(r))

    # The same shape BETWEEN two funcs. While spans were contiguous this tag was silently
    # re-homed onto the PRECEDING function: the gate credited it, the hook protected the
    # wrong function, and the "outside every scope" fallback could never fire because there
    # was no gap to fall into.
    between = os.path.join(tmp, "between_test.go")
    with open(between, "w", encoding="utf-8") as fh:
        fh.write(
            "package message\n"
            "\n"
            "func helperFirst(t *testing.T) {\n" + other + "}\n"
            "\n"
            "var cases = []tc{\n"
            "\t" + tag + '\t{name: "withdraw"},\n'
            "}\n"
            "\n"
            "func TestRunner(t *testing.T) {\n" + body + "}\n"
        )
    r = edit(body, body.replace("TreatAsWithdraw", "None"), between)
    results.check("rfc-guard-tag-between-funcs-widens", blocked_on_rfc(r), repr(r))

    # A blank line between the tag and its func. The doc-comment walk-back stops at the
    # blank line, so the tag belongs to no func's comment block -- it must widen, not
    # attach to whichever function happens to sit above it.
    gapped = os.path.join(tmp, "gapped_test.go")
    with open(gapped, "w", encoding="utf-8") as fh:
        fh.write(
            "package message\n"
            "\n"
            "func helperFirst(t *testing.T) {\n" + other + "}\n"
            "\n" + tag + "\n"
            "func TestGapped(t *testing.T) {\n" + body + "}\n"
        )
    r = edit(body, body.replace("TreatAsWithdraw", "None"), gapped)
    results.check("rfc-guard-blank-line-tag-widens", blocked_on_rfc(r), repr(r))

    # The inverse of the two above: the helper ABOVE an unowned tag must not be blocked
    # with an RFC message it has nothing to do with... except that an unowned tag widens to
    # the whole file, which is the deliberate conservative side. Pin the direction that
    # matters -- the tagged test IS protected -- and let the helper share the file's scope.
    r = edit(other, other.replace("require.Equal", "require.NotEqual"), gapped)
    results.check("rfc-guard-unowned-tag-covers-file", blocked_on_rfc(r), repr(r))

    # A non-unique hunk WITHOUT replace_all: the tool rejects it for being ambiguous, so
    # only the first occurrence could ever be edited. Blocking on a tagged copy elsewhere
    # answered a question nobody asked, and told the author the wrong cause.
    r = edit(other, "\trequire.NotNil(t, other)\n", dup)
    results.check("rfc-guard-ambiguous-hunk-no-replace-all-passes", r is None, repr(r))

    # A MultiEdit whose hunks land in different functions is judged per hunk: the tagged
    # one blocks. Joining the hunks and searching the file for that join would find
    # nothing and silently fall back to the old, narrow behavior.
    r = cw(
        {
            "tool": "MultiEdit",
            "ti": {
                "edits": [
                    {"old_string": other, "new_string": other.replace("1", "2")},
                    {
                        "old_string": body,
                        "new_string": body.replace("TreatAsWithdraw", "None"),
                    },
                ]
            },
            "fp": fp,
        }
    )
    results.check("rfc-guard-multiedit-tagged-hunk-blocks", blocked_on_rfc(r), repr(r))

    # The .ci branch on a REAL on-disk file: the pre-existing .ci fixture uses a /repo/
    # path that cannot be opened, so it never reaches this code.
    # Under test/: c_test_weakening only treats a .ci as a test when the path contains
    # "/test/" (:1835), so a .ci anywhere else is not judged at all.
    os.makedirs(os.path.join(tmp, "test", "plugin"), exist_ok=True)
    ci = os.path.join(tmp, "test", "plugin", "rfc7606-reset.ci")
    with open(ci, "w", encoding="utf-8") as fh:
        fh.write(
            "# RFC requirement: RFC7606-3.a-1 negative - NOTIFICATION on reset.\n"
            "expect=bgp:conn=1:seq=1:hex=FFFF\n"
            "expect=bgp:conn=1:seq=2:hex=00AA\n"
        )
    r = edit(
        "expect=bgp:conn=1:seq=2:hex=00AA\n",
        "expect=bgp:conn=1:seq=2:hex=00BB\n",
        ci,
    )
    results.check("rfc-guard-ci-on-disk-covers-whole-file", blocked_on_rfc(r), repr(r))

    # An interop scenario's check.py. `is_test` covers `_test.go` and a `/test/` `.ci` and
    # NOTHING else, so when plan/spec-rfcgate-2-evidence.md started admitting interop
    # evidence, two check.py files began carrying RFC obligations the gate counts as proof
    # while this guard could not see them at all (spec-rfcgate-3-audit-teeth.md C-4).
    scen = os.path.join(tmp, "test", "interop", "scenarios", "47-shape")
    os.makedirs(scen, exist_ok=True)
    py = os.path.join(scen, "check.py")
    py_body = "    assert peer_installed(route), 'FRR must install the relayed route'\n"
    with open(py, "w", encoding="utf-8") as fh:
        fh.write(
            "# RFC requirement: RFC7606-5.1-3 positive - the mixed shape is accepted.\n"
            "def check():\n" + py_body
        )
    r = edit(py_body, py_body.replace("assert ", "# assert "), py)
    results.check("rfc-guard-covers-tagged-check-py", blocked_on_rfc(r), repr(r))

    # ...and only a TAGGED one. Widening the predicate to every .py in the repository would
    # drag unrelated scenarios into a guard that has nothing to say about them.
    plain = os.path.join(scen, "helper.py")
    with open(plain, "w", encoding="utf-8") as fh:
        fh.write("def check():\n" + py_body)
    r = edit(py_body, py_body.replace("assert ", "# assert "), plain)
    results.check("rfc-guard-untagged-py-unaffected", r is None, repr(r))

    # A comment-only edit to a tagged check.py must PASS. `#` is Python's comment syntax and
    # also the carrier its tag lives on, so judging it with the Go `//` stripper would read
    # every re-worded comment as a behaviour change -- the over-blocking that gets a guard
    # switched off.
    r = edit(
        "# RFC requirement: RFC7606-5.1-3 positive - the mixed shape is accepted.\n",
        "# RFC requirement: RFC7606-5.1-3 positive - a mixed shape is accepted on receive.\n",
        py,
    )
    results.check("rfc-guard-py-comment-edit-passes", r is None, repr(r))

    # A ONE-LINE func has no closing brace at column 0, so its span falls back to the cap.
    # If that cap were the next func KEYWORD instead of the next func's DOC COMMENT, the
    # one-liner would swallow the tag below it and block. This is the only shape where the
    # two caps differ, so without it the original 331-false-block boundary bug can be
    # reintroduced with every other fixture still green.
    oneline = os.path.join(tmp, "oneline_test.go")
    with open(oneline, "w", encoding="utf-8") as fh:
        fh.write(
            "package message\n"
            "\n"
            "func helperOneLine(t *testing.T) { require.Equal(t, 9, nine) }\n"
            "\n" + tag + "func TestAfterOneLine(t *testing.T) {\n" + body + "}\n"
        )
    r = edit(
        "func helperOneLine(t *testing.T) { require.Equal(t, 9, nine) }\n",
        "func helperOneLine(t *testing.T) { require.NotEqual(t, 9, nine) }\n",
        oneline,
    )
    results.check("rfc-guard-one-line-func-does-not-absorb-tag", r is None, repr(r))

    # ...and the tagged test below it is still protected.
    r = edit(body, body.replace("TreatAsWithdraw", "None"), oneline)
    results.check(
        "rfc-guard-one-line-func-neighbour-still-blocks", blocked_on_rfc(r), repr(r)
    )

    # A hunk that is nowhere in the file (a chained MultiEdit, a stale read) cannot be
    # located, so no narrow scope is honest. Fail closed: the file has tags, so ask.
    r = edit("\tthis text is not in the file at all\n", "\tsomething else\n", fp)
    results.check("rfc-guard-unlocatable-hunk-fails-closed", blocked_on_rfc(r), repr(r))

    # A file with no tag at all keeps the cheap path and the old behavior.
    plain = os.path.join(tmp, "plain_test.go")
    with open(plain, "w", encoding="utf-8") as fh:
        fh.write("package message\n\nfunc TestPlain(t *testing.T) {\n" + other + "}\n")
    r = edit(other, other.replace("require.Equal", "require.NotEqual"), plain)
    results.check("rfc-guard-untagged-file-unaffected", r is None, repr(r))


# --------------------------------------------------------------------------- #
# mark-source-read: the spec-write gate's evidence set (T-4)
# --------------------------------------------------------------------------- #


def _run_mark_source_read(file_path: str) -> bool:
    """Drive mark-source-read.sh over a Read of `file_path`; return marker-written.

    The script cd's to CLAUDE_PROJECT_DIR and sources
    .claude/hooks/lib/session-id.sh relative to it, so the fixture project needs a
    copy of that lib. A fixed CLAUDE_CODE_SESSION_ID makes the marker path
    deterministic (env source wins in _session_id).
    """
    work = tempfile.mkdtemp(prefix="mark-source-read-", dir=_fixture_root())
    try:
        libdst = os.path.join(work, ".claude", "hooks", "lib")
        os.makedirs(libdst, exist_ok=True)
        shutil.copytree(os.path.join(HOOKS, "lib"), libdst, dirs_exist_ok=True)
        sid = "11111111-2222-3333-4444-555555555555"
        env = dict(os.environ, CLAUDE_PROJECT_DIR=work, CLAUDE_CODE_SESSION_ID=sid)
        payload = json.dumps({"tool_input": {"file_path": file_path}})
        subprocess.run(
            ["bash", os.path.join(HOOKS, "mark-source-read.sh")],
            input=payload,
            text=True,
            capture_output=True,
            env=env,
            timeout=30,
        )
        return os.path.isfile(
            os.path.join(work, "tmp", "session", f".source-read-{sid}")
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_mark_source_read(results: Results) -> None:
    """T-4 (AC-6): reading the .py/.sh/Makefile a spec is ABOUT must satisfy the
    spec-write gate -- the marker is written for those, not only for Go, so an
    agent need not read an unrelated .go file purely to pass."""
    print("mark-source-read:")

    for label, path in (
        ("go-internal", "/repo/internal/x/y.go"),
        ("py-scripts", "/repo/scripts/dev/foo.py"),
        ("sh-hooks", "/repo/.claude/hooks/foo.sh"),
        ("makefile", "/repo/Makefile"),
        ("mk-file", "/repo/mk/inventory.mk"),
    ):
        results.check(
            f"mark-source-read-writes-{label}",
            _run_mark_source_read(path),
            f"marker not written for {path}",
        )

    # MUST-NOT-FIRE: an unrelated doc/spec read does not ground a spec, so it must
    # NOT write the marker (the gate stays honest about what counts as evidence).
    for label, path in (
        ("doc", "/repo/docs/guide/x.md"),
        ("spec", "/repo/plan/spec-foo.md"),
        ("py-outside-scripts", "/repo/internal/x/tool.py"),
    ):
        results.check(
            f"mark-source-read-skips-{label}",
            not _run_mark_source_read(path),
            f"marker wrongly written for {path}",
        )


# --------------------------------------------------------------------------- #
# delegation: mark-agent-spawned + the stop-hook nudge + subagent context
# --------------------------------------------------------------------------- #

_DELEG_SID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"


def _deleg_project(spec: str | None, status: str = "ready", spawned: bool = False):
    """Build a fixture project: hook libs, an optional claimed spec, an optional
    agent-spawned marker. Returns the project dir (caller removes it)."""
    work = tempfile.mkdtemp(prefix="delegation-", dir=_fixture_root())
    libdst = os.path.join(work, ".claude", "hooks", "lib")
    os.makedirs(libdst, exist_ok=True)
    shutil.copytree(os.path.join(HOOKS, "lib"), libdst, dirs_exist_ok=True)
    os.makedirs(os.path.join(work, "tmp", "session"), exist_ok=True)
    if spec:
        os.makedirs(os.path.join(work, "plan"), exist_ok=True)
        with open(os.path.join(work, "plan", spec), "w") as fh:
            fh.write(f"# Spec: fixture\n\n| Status | {status} |\n")
        with open(
            os.path.join(work, "tmp", "session", f".session-{_DELEG_SID}"), "w"
        ) as fh:
            fh.write(spec + "\n")
    if spawned:
        with open(
            os.path.join(work, "tmp", "session", f".agent-spawned-{_DELEG_SID}"), "w"
        ) as fh:
            fh.write("2026-07-28T00:00:00+00:00\n")
    return work


def _deleg_env(work: str) -> dict:
    return dict(os.environ, CLAUDE_PROJECT_DIR=work, CLAUDE_CODE_SESSION_ID=_DELEG_SID)


def _run_stop_hook(work: str, message: str | None = None) -> tuple[int, str]:
    """Drive block-premature-stop.sh. The default message carries no stop phrases,
    so only the STATE reasons can fire. Pass `message` to drive the phrase scan."""
    payload = json.dumps(
        {
            "last_assistant_message": message
            or "Implemented the change and ran the tests."
        }
    )
    r = subprocess.run(
        ["bash", os.path.join(HOOKS, "block-premature-stop.sh")],
        input=payload,
        text=True,
        capture_output=True,
        env=_deleg_env(work),
        timeout=60,
    )
    return r.returncode, r.stderr


def run_delegation(results: Results) -> None:
    """ai/rules/spec-delegation.md: a session that claimed a spec and never
    spawned an agent ran the phase inline instead of supervising it. The nudge
    must fire on exactly that, WARN rather than block, and stay silent once a
    subagent was spawned or when no spec is claimed."""
    print("delegation:")

    # mark-agent-spawned.sh writes the marker the nudge reads.
    work = _deleg_project(spec=None)
    try:
        subprocess.run(
            ["bash", os.path.join(HOOKS, "mark-agent-spawned.sh")],
            input="{}",
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=30,
        )
        results.check(
            "delegation-marker-written",
            os.path.isfile(
                os.path.join(work, "tmp", "session", f".agent-spawned-{_DELEG_SID}")
            ),
            "mark-agent-spawned.sh did not write the marker",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # MUST FIRE: spec claimed, no agent ever spawned. Warn (1), never block (2).
    work = _deleg_project(spec="spec-fixture.md", spawned=False)
    try:
        rc, err = _run_stop_hook(work)
        results.check("delegation-nudge-fires", "Delegation:" in err, err)
        results.check("delegation-nudge-warns-not-blocks", rc == 1, f"rc={rc} {err}")
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # MUST NOT FIRE: the session delegated at least once.
    work = _deleg_project(spec="spec-fixture.md", spawned=True)
    try:
        rc, err = _run_stop_hook(work)
        results.check(
            "delegation-nudge-silent-when-spawned", "Delegation:" not in err, err
        )
        results.check("delegation-spawned-allows-stop", rc == 0, f"rc={rc} {err}")
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # MUST NOT FIRE: no spec claimed at all -- the rule is about spec phases.
    work = _deleg_project(spec=None, spawned=False)
    try:
        rc, err = _run_stop_hook(work)
        results.check("delegation-no-spec-no-nudge", "Delegation:" not in err, err)
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # subagent-context.sh names the parent's claimed spec, so the main thread does
    # not have to paste it into every prompt (the friction this rule died on).
    work = _deleg_project(spec="spec-fixture.md")
    try:
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "subagent-context.sh")],
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=30,
        )
        results.check(
            "delegation-context-names-spec",
            "plan/spec-fixture.md" in r.stdout,
            r.stdout,
        )
        results.check(
            "delegation-context-carries-contract",
            "spec-delegation.md" in r.stdout and "no-fabrication.md" in r.stdout,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # No spec claimed: the context still loads, minus the spec block.
    work = _deleg_project(spec=None)
    try:
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "subagent-context.sh")],
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=30,
        )
        results.check(
            "delegation-context-no-spec-block",
            "Spec claimed by" not in r.stdout and r.returncode == 0,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # Present is not wired. Every fixture above passes against a script that is
    # registered on NO event, which is exactly the state block-premature-stop.sh
    # sat in from 2026-06-29 to 2026-07-31: on disk, green in this file, and
    # described as live by three rules, while the Stop array said otherwise.
    settings = os.path.join(ROOT, ".claude", "settings.json")
    with open(settings, encoding="utf-8") as fh:
        cfg = json.load(fh)
    stop = [
        entry.get("command", "")
        for group in cfg.get("hooks", {}).get("Stop", [])
        for entry in group.get("hooks", [])
    ]
    results.check(
        "delegation-stop-hook-registered",
        any(c.endswith("block-premature-stop.sh") for c in stop),
        repr(stop),
    )

    # ORDER IS LOAD-BEARING, though it is not sufficient on its own.
    # session-end-summary.sh USED TO call _release_session, deleting
    # tmp/session/.session-<SID> (lib/state-file.sh) on every Stop event.
    # block-premature-stop.sh reads that marker to decide whether to run the
    # closure gate, the in-progress warning and this delegation nudge, so it must
    # still run first. The release now happens on SessionEnd instead, which is
    # what keeps the claim alive past turn one; `delegation-claim-survives-stop`
    # below pins that half, and this fixture pins the ordering half. Neither
    # alone makes the three gates work.
    def _index(suffix: str) -> int:
        for i, c in enumerate(stop):
            if c.endswith(suffix):
                return i
        return -1

    guard, summary = _index("block-premature-stop.sh"), _index("session-end-summary.sh")
    results.check(
        "delegation-stop-hook-runs-before-marker-release",
        guard >= 0 and (summary < 0 or guard < summary),
        f"block-premature-stop at {guard}, session-end-summary at {summary}: {stop!r}",
    )

    # PHRASE SCAN: a banned phrase that is QUOTED is being named, not used.
    # The scan blocked its own first live turn on 2026-07-31, on a report that
    # documented `would you like me to` as an example inside backticks.
    work = _deleg_project(spec=None)
    try:
        # Still blocks when the phrase is genuinely used.
        rc, err = _run_stop_hook(work, "Done. Would you like me to run the tests?")
        results.check("stop-phrase-blocks-real-use", rc == 2, f"rc={rc} {err}")

        # Does not block when the phrase sits in an inline code span.
        rc, err = _run_stop_hook(
            work, "The scan matches `would you like me to` and blocks the turn."
        )
        results.check("stop-phrase-ignores-backticks", rc == 0, f"rc={rc} {err}")

        # Does not block when the phrase sits in a fenced block.
        fenced = "Banned patterns:\n\n```\nwould you like me to\nshall I proceed\n```\n\nRegistered and verified."
        rc, err = _run_stop_hook(work, fenced)
        results.check("stop-phrase-ignores-fenced-block", rc == 0, f"rc={rc} {err}")

        # A phrase outside the fence still blocks, so the fence is not a bypass.
        #
        # Assert WHICH phrase matched, not merely that something did. Checking
        # rc == 2 alone made this fixture decorative: under an inverted fence
        # toggle the outside text is discarded and the FENCED phrase matches, so
        # it stayed green through the exact bypass it exists to disprove.
        #
        # The fenced phrase is ordered EARLIER in PHRASES than the outside one,
        # and the loop breaks on first match. So if the fence ever leaks, the
        # reported pattern changes and this fixture goes red. With the two
        # phrases the other way round a leak is invisible, because the outside
        # phrase matches first either way.
        mixed = (
            "Example:\n\n```\nlet me know if you need it\n```"
            "\n\nDone. Would you like me to continue?"
        )
        rc, err = _run_stop_hook(work, mixed)
        results.check(
            "stop-phrase-fence-is-not-a-bypass",
            rc == 2 and "would you like me to" in err and "let me know" not in err,
            f"rc={rc} {err}",
        )

        # An UNCLOSED fence is not a code block. Dropping the lines after it made
        # the gate fail OPEN: a real request passed with rc=0.
        unclosed = (
            "Intro\n\n```bash\nmake ze-verify\n\nDone. Would you like me to continue?"
        )
        rc, err = _run_stop_hook(work, unclosed)
        results.check(
            "stop-phrase-unclosed-fence-still-blocks",
            rc == 2 and "would you like me to" in err,
            f"rc={rc} {err}",
        )

        # All-markup must not strip the message down to nothing and match nothing.
        allfence = "```\nWould you like me to continue?\n```"
        rc, err = _run_stop_hook(work, allfence)
        results.check(
            "stop-phrase-all-markup-scans-raw-text",
            rc == 2 and "would you like me to" in err,
            f"rc={rc} {err}",
        )

        # A fence closes only on a run at least as long as the opening one, so a
        # ````markdown wrapper does not leak its inner block.
        nested = (
            "Example:\n\n````markdown\n```\nwould you like me to\n```\n````\n\nDone."
        )
        rc, err = _run_stop_hook(work, nested)
        results.check("stop-phrase-nested-fence-ignored", rc == 0, f"rc={rc} {err}")

        # Unparseable input must ALLOW the stop, with a documented exit code.
        # Under `set -eo pipefail` the jq failure used to kill the script, so the
        # hook returned 5, which its own header does not define.
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "block-premature-stop.sh")],
            input="not json",
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        results.check(
            "stop-hook-malformed-input-allows-stop",
            r.returncode == 0,
            f"rc={r.returncode} {r.stderr}",
        )

        # Loop bound: a stop already refused once is not refused again.
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "block-premature-stop.sh")],
            input=json.dumps(
                {
                    "stop_hook_active": True,
                    "last_assistant_message": "Would you like me to continue?",
                }
            ),
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        results.check(
            "stop-hook-honours-stop-hook-active",
            r.returncode == 0,
            f"rc={r.returncode} {r.stderr}",
        )

        # .claude/rules/session-start.md:72 REQUIRES asking "What next?" once the
        # original task is done. With no claimed in-progress spec there is no open
        # work, so the completion list must not fire. This project has no spec.
        rc, err = _run_stop_hook(work, "Spec closed and committed. What next?")
        results.check(
            "stop-phrase-what-next-allowed-when-no-open-work",
            rc == 0,
            f"rc={rc} {err}",
        )

        # Permission-seeking is a different failure and still blocks with no claim.
        rc, err = _run_stop_hook(work, "Done. Would you like me to run the tests?")
        results.check(
            "stop-phrase-permission-blocks-without-open-work",
            rc == 2,
            f"rc={rc} {err}",
        )

        # An UNPAIRED backtick must not delete the request. A left-to-right strip
        # pairs the stray tick with the OPENING tick of the later legitimate span
        # and removes everything between, which is where the request sits. A
        # dropped closing backtick is an ordinary typo, so this needs no intent.
        stray = (
            "I fixed the ` escaping bug. "
            "Would you like me to also run `make ze-verify`?"
        )
        rc, err = _run_stop_hook(work, stray)
        results.check(
            "stop-phrase-unpaired-backtick-still-blocks",
            rc == 2 and "would you like me to" in err,
            f"rc={rc} {err}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The retry bound must gate the PHRASE SCAN ONLY. An early exit also skipped
    # the spec-closure gate, which is a real exit 2 and needs no bound because it
    # has two documented escapes. Proxy for that: on a retry the state checks must
    # still run, so the delegation nudge still warns.
    work = _deleg_project(spec="spec-fixture.md", spawned=False)
    try:
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "block-premature-stop.sh")],
            input=json.dumps(
                {
                    "stop_hook_active": True,
                    "last_assistant_message": "Would you like me to continue?",
                }
            ),
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        results.check(
            "stop-hook-retry-still-runs-state-checks",
            r.returncode == 1 and "Delegation:" in r.stderr,
            f"rc={r.returncode} {r.stderr!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The same completion phrase DOES block while a claimed spec is in-progress,
    # which is the state that means work remains. Without this pair the fix reads
    # as "deleted the phrases" rather than "made them conditional".
    work = _deleg_project(spec="spec-fixture.md", status="in-progress", spawned=True)
    try:
        rc, err = _run_stop_hook(work, "Spec closed and committed. What next?")
        results.check(
            "stop-phrase-what-next-blocks-with-open-work",
            rc == 2 and "what next" in err.lower(),
            f"rc={rc} {err}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # MARKER LIFETIME: the array order is necessary but NOT sufficient. Releasing
    # the claim on Stop destroyed it after turn one, so the closure gate, the
    # in-progress warning and the delegation nudge each fired once per claim and
    # were silent afterwards. The closure gate suffered worst, since it can only
    # exit 3 once commit A has landed, many turns after the claim. Drive two
    # consecutive Stop events and require the nudge on BOTH.
    work = _deleg_project(spec="spec-fixture.md", spawned=False)
    try:
        # session-end-summary.sh returns early on a clean tree, which is the path
        # that skips the release entirely. Give it a dirty git repo so it runs to
        # the END, where the release actually lived. Without this the fixture
        # passes even with the bug restored, which is how the first version of it
        # was written and why it caught nothing.
        subprocess.run(["git", "init", "-q", "."], cwd=work, capture_output=True)
        with open(os.path.join(work, "dirty.txt"), "w") as fh:
            fh.write("uncommitted\n")

        _, err1 = _run_stop_hook(work)
        summary = subprocess.run(
            ["bash", os.path.join(HOOKS, "session-end-summary.sh")],
            input="{}",
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        results.check(
            "delegation-summary-reaches-end",
            summary.returncode == 0
            and os.path.isfile(
                os.path.join(
                    work, "tmp", "session", f"session-state-fixture-{_DELEG_SID}.md"
                )
            ),
            "session-end-summary.sh did not run its full path, so the release "
            f"site is untested (rc={summary.returncode})",
        )
        marker = os.path.join(work, "tmp", "session", f".session-{_DELEG_SID}")
        results.check(
            "delegation-claim-survives-stop",
            os.path.isfile(marker),
            "session-end-summary.sh released the claim on a Stop event",
        )
        _, err2 = _run_stop_hook(work)
        results.check(
            "delegation-nudge-fires-on-second-stop",
            "Delegation:" in err1 and "Delegation:" in err2,
            f"turn1={err1!r} turn2={err2!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The OFF half of the same lifetime. Moving the release to SessionEnd is only
    # half a fix if nothing asserts the release still happens: deleting the line
    # outright left the whole suite green, so a future session could "fix a leak"
    # by removing the release and get a clean bar. SessionEnd resolves its id from
    # stdin, not the environment, so drive it the way the harness does.
    def _session_end(work: str, sid: str, reason: str) -> None:
        subprocess.run(
            ["bash", os.path.join(HOOKS, "session-end-scratch.sh")],
            input=json.dumps({"session_id": sid, "reason": reason}),
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )

    work = _deleg_project(spec="spec-fixture.md")
    try:
        marker = os.path.join(work, "tmp", "session", f".session-{_DELEG_SID}")
        _session_end(work, _DELEG_SID, "other")
        results.check(
            "delegation-claim-released-at-session-end",
            not os.path.isfile(marker),
            "SessionEnd did not release the claim, so claims leak forever",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # A session ending only to be resumed is not over, so it keeps its claim.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        marker = os.path.join(work, "tmp", "session", f".session-{_DELEG_SID}")
        _session_end(work, _DELEG_SID, "resume")
        results.check(
            "delegation-claim-survives-resume",
            os.path.isfile(marker),
            "a resumed session lost its claim",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_delegation_reminder(results: Results) -> None:
    """ai/rules/spec-delegation.md: the harness guard "Do not call the AgentTool
    unless the user requested it" arrives LAST in the system prompt and wins on
    position. UserPromptSubmit stdout is the only harness position that lands
    after the whole system prompt, so the counter-reminder must reach STDOUT.
    A stderr reminder is invisible to the model and guards nothing, which is the
    one failure this section exists to catch."""
    print("delegation-reminder:")

    hook = os.path.join(HOOKS, "delegation-reminder.sh")
    results.check("delegation-reminder-exists", os.path.isfile(hook), hook)

    r = subprocess.run(
        ["bash", hook],
        input='{"prompt":"fixture"}',
        text=True,
        capture_output=True,
        timeout=30,
    )

    results.check(
        "delegation-reminder-exits-zero", r.returncode == 0, f"rc={r.returncode}"
    )

    # The two load-bearing substrings: the permission it grants, and the rule it
    # cites. Asserting the whole line would make every re-wording a red.
    results.check(
        "delegation-reminder-grants-permission",
        "needs no permission" in r.stdout,
        repr(r.stdout),
    )
    results.check(
        "delegation-reminder-cites-rule",
        "spec-delegation.md" in r.stdout,
        repr(r.stdout),
    )

    # THE point of the hook. UserPromptSubmit stdout reaches the model and stderr
    # does not, so anything on stderr is a reminder the model never sees.
    results.check("delegation-reminder-stderr-empty", r.stderr == "", repr(r.stderr))

    # It fires every turn, so it must stay one line.
    results.check(
        "delegation-reminder-single-line",
        len(r.stdout.strip().splitlines()) == 1,
        repr(r.stdout),
    )

    # Present is not wired. Prove the hook is registered on UserPromptSubmit.
    settings = os.path.join(ROOT, ".claude", "settings.json")
    with open(settings, encoding="utf-8") as fh:
        cfg = json.load(fh)
    commands = [
        entry.get("command", "")
        for group in cfg.get("hooks", {}).get("UserPromptSubmit", [])
        for entry in group.get("hooks", [])
    ]
    results.check(
        "delegation-reminder-registered",
        any(c.endswith("delegation-reminder.sh") for c in commands),
        repr(commands),
    )


SECTIONS = {
    "format-alloc": run_format_alloc,
    "validate-spec": run_validate_spec,
    "commit-gate": run_commit_gate,
    "session-id": run_session_id,
    "rfc-test-guard": run_rfc_test_guard,
    "mark-source-read": run_mark_source_read,
    "delegation": run_delegation,
    "delegation-reminder": run_delegation_reminder,
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--only", choices=sorted(SECTIONS), help="run one section")
    args = parser.parse_args()
    results = Results()
    for name, fn in SECTIONS.items():
        if args.only and name != args.only:
            continue
        fn(results)
    total = results.passed + results.failed
    print(f"\nhook fixture check: {results.passed}/{total} passed")
    print("OK" if results.failed == 0 else f"{results.failed} FAILURE(S)")
    return 1 if results.failed else 0


if __name__ == "__main__":
    sys.exit(main())
