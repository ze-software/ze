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

Sections (select one with --only):
    format-alloc   c_format_alloc guarded-file / comment-exemption logic
    validate-spec  validate-spec.sh over ASCII / Unicode / malformed specs
    commit-gate    commit_helper.py creation-time gates in git fixtures
    session-id     lib/session-id.sh vs pretool-writeedit.py resolve ONE id

    python3 scripts/dev/hook-fixture-check.py                 # all sections
    python3 scripts/dev/hook-fixture-check.py --only validate-spec

Exit 0 = every fixture matched its expectation, 1 = a hook regressed.
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile

ROOT = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..")
)
HOOKS = os.path.join(ROOT, ".claude", "hooks")
DEV = os.path.abspath(os.path.dirname(__file__))


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


# Six-column layout matching the real plan/deferrals.md (Date | Source | What |
# Reason | Destination | Status); the gate reads Destination and Status by index.
_DEFERRALS_HEADER = (
    "# Deferrals\n\n"
    "| Date | Source | What | Reason | Destination | Status |\n"
    "|------|--------|------|--------|-------------|--------|\n"
)


def run_commit_gate(results: Results) -> None:
    print("commit-gate:")
    from pathlib import Path

    ch = _load_commit_helper()

    # --- deferral-unassigned (block) ---
    repo = _init_repo()
    try:
        _write(
            repo,
            "plan/deferrals.md",
            _DEFERRALS_HEADER + "| 2026-07-09 | abc | thing | reason |  | open |\n",
        )
        problems = ch.deferral_unassigned_problems(Path(repo))
        results.check("commit-gate-deferral-unassigned", bool(problems), repr(problems))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # An assigned destination passes only when the spec it names EXISTS. Both
    # spellings resolve to the same file (ai/rules/deferral-tracking.md).
    repo = _init_repo()
    try:
        _write(repo, "plan/spec-foo.md", "# Spec: foo\n")
        _write(
            repo,
            "plan/deferrals.md",
            _DEFERRALS_HEADER
            + "| 2026-07-09 | abc | thing | reason | spec-foo.md | open |\n"
            + "| 2026-07-09 | abc | thing2 | reason | `plan/spec-foo.md` | open |\n",
        )
        problems = ch.deferral_unassigned_problems(Path(repo))
        results.check("commit-gate-deferral-assigned-ok", not problems, repr(problems))
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # A destination naming a spec nobody created loses the work exactly as a
    # prose destination does, so it must block too.
    repo = _init_repo()
    try:
        _write(
            repo,
            "plan/deferrals.md",
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
        # deferrals.md included in the commit clears it
        _write(repo, "plan/deferrals.md", _DEFERRALS_HEADER)
        problems = ch.deferral_in_diff_problems(
            Path(repo), ("docs/notes.md", "plan/deferrals.md"), ()
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
        import contextlib
        import io

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
    # must fall through together, or they would disagree on the marker path.
    for label, bad in (
        ("traversal", "../../../etc/passwd"),
        ("slash", "a/b"),
        ("space", "has space"),
        ("empty", ""),
    ):
        e3 = env_with(bad)
        b3, p3 = _sid_bash(e3), _sid_python(e3)
        results.check(
            f"session-id-rejects-{label}",
            b3 == p3 and "/" not in b3 and b3 != "" and b3 != bad,
            f"bash={b3!r} py={p3!r}",
        )

    # With no id source at all, both ends still agree (on the shared constant).
    e4 = env_with(None)
    b4, p4 = _sid_bash(e4), _sid_python(e4)
    results.check(
        "session-id-no-source-parity", b4 == p4 and b4 != "", f"bash={b4!r} py={p4!r}"
    )


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


SECTIONS = {
    "format-alloc": run_format_alloc,
    "validate-spec": run_validate_spec,
    "commit-gate": run_commit_gate,
    "session-id": run_session_id,
    "rfc-test-guard": run_rfc_test_guard,
    "mark-source-read": run_mark_source_read,
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
