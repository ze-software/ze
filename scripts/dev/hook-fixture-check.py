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
sections, so this file keeps no second list (ai/rules/evidence.md).

    python3 scripts/dev/hook-fixture-check.py                 # all sections
    python3 scripts/dev/hook-fixture-check.py --help          # list the sections
    python3 scripts/dev/hook-fixture-check.py --only validate-spec

Exit 0 = every fixture matched its expectation, 1 = a hook regressed.
"""

from __future__ import annotations

import argparse
import datetime
import importlib.util
import glob
import json
import os
import re
import shutil
import shlex
import subprocess
import sys
import tempfile
import time

ROOT = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
    os.path.join(os.path.dirname(__file__), "..", "..")
)
HOOKS = os.path.join(ROOT, ".claude", "hooks")
DEV = os.path.abspath(os.path.dirname(__file__))

# This runner is not a sub-make. Under `make ze-unit-hook-test` it inherits
# MAKELEVEL and MAKEFLAGS, so every `make` a fixture starts announces
# "Entering directory" on stdout -- and the session-id fixtures compare that
# stdout byte for byte against the path the target prints. Dropped once, here,
# because every child environment below derives from os.environ.
for _make_var in ("MAKELEVEL", "MAKEFLAGS", "MFLAGS"):
    os.environ.pop(_make_var, None)

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


def _load_pretool_bash():
    path = os.path.join(HOOKS, "pretool-bash.py")
    spec = importlib.util.spec_from_file_location("pretool_bash", path)
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


def run_design_ref(results: Results) -> None:
    """c_require_design_ref asks for a `// Design:` line, and vendored code has no
    ze design document to name. The check refused every edit under vendor/, which
    made the vendor patch this tree carries deliberately (scripts/dev/patches/,
    pinned by TestNetlinkXFRMPatchApplied) unrepairable with Write or Edit."""
    print("design-ref:")
    mod = _load_pretool_writeedit()
    crd = mod.c_require_design_ref
    body = "package netlink\n\nfunc f() {}\n"

    def call(fp: str, content: str = body):
        return crd({"tool": "Write", "ti": {"content": content}, "fp": fp})

    r = call("/repo/vendor/github.com/vishvananda/netlink/xfrm_state_linux.go")
    results.check("design-ref-vendor-exempt", r is None, repr(r))

    r = call("vendor/github.com/vishvananda/netlink/nl/xfrm_state_linux.go")
    results.check("design-ref-vendor-relative-exempt", r is None, repr(r))

    # The exemption is a PATH COMPONENT, not a substring: a ze package whose name
    # merely starts with the word keeps the obligation.
    r = call("/repo/internal/component/vendored/thing.go")
    results.check(
        "design-ref-vendor-substring-still-blocked",
        r is not None and r[0] == 2,
        repr(r),
    )

    r = call("/repo/internal/component/bgp/reactor/peer.go")
    results.check("design-ref-ze-file-blocked", r is not None and r[0] == 2, repr(r))

    r = call(
        "/repo/internal/component/bgp/reactor/peer.go",
        "// Design: docs/architecture/bgp/reactor.md -- FSM\n" + body,
    )
    results.check("design-ref-ze-file-with-line", r is None, repr(r))


# --------------------------------------------------------------------------- #
# rendered-rule: ai/rules/<rule>.md is generated, ai/rules/points/** is not
# --------------------------------------------------------------------------- #


def _writeedit(fp: str, tool: str = "Edit", content: str = "x") -> tuple[int, str]:
    """Drive the WHOLE pretool-writeedit dispatcher over one payload.

    Through the dispatcher, never by importing one check and calling it. A
    helper's return value cannot see whether the function is in `CHECKS`, cannot
    see a sibling check refusing the same path for another reason, and cannot
    see `main()` failing to hand it a file path at all -- which is exactly how
    the NotebookEdit branch stayed unreachable while a tuple claimed it.
    """
    ti = {"file_path": fp}
    if tool == "Write":
        ti["content"] = content
    else:
        ti["old_string"] = "a"
        ti["new_string"] = content
    payload = json.dumps({"tool_name": tool, "tool_input": ti})
    proc = subprocess.run(
        [sys.executable, os.path.join(HOOKS, "pretool-writeedit.py")],
        input=payload,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return proc.returncode, proc.stderr


def _multiedit(fp: str, edits: list[dict]) -> tuple[int, str]:
    """Drive the dispatcher over a MultiEdit payload, edits verbatim."""
    payload = json.dumps(
        {"tool_name": "MultiEdit", "tool_input": {"file_path": fp, "edits": edits}}
    )
    proc = subprocess.run(
        [sys.executable, os.path.join(HOOKS, "pretool-writeedit.py")],
        input=payload,
        capture_output=True,
        text=True,
        timeout=30,
    )
    return proc.returncode, proc.stderr


def _case_insensitive(directory: str) -> bool:
    """Whether `directory`'s filesystem opens one name under two spellings.

    Asked of the filesystem rather than of `sys.platform`: a case-sensitive
    volume mounted on macOS, and a case-insensitive one on Linux, are both
    ordinary. The expectation the fixtures assert is a property of the volume
    the repository sits on.
    """
    try:
        return os.path.samefile(directory, directory.upper())
    except OSError:
        return False


def run_rendered_rule(results: Results) -> None:
    print("rendered-rule:")
    rules = os.path.join(ROOT, "ai", "rules")

    # AC-7: the rendered rule is refused, and the refusal names the point dir.
    code, err = _writeedit(os.path.join(rules, "performance.md"))
    results.check(
        "rendered-rule-edit-refused",
        code == 2 and "ai/rules/points/performance/" in err,
        repr((code, err)),
    )

    # AC-8: a point file is the canonical source and is permitted. The tree is at
    # a fixed depth of two, so a point sits under its `##` section directory.
    code, err = _writeedit(
        os.path.join(
            rules, "points", "performance", "hot-path-rule", "some-directive.md"
        )
    )
    results.check("point-file-edit-allowed", code == 0, repr((code, err)))

    code, err = _writeedit(os.path.join(rules, "points", "performance", "manifest.md"))
    results.check("point-manifest-edit-allowed", code == 0, repr((code, err)))

    # The three artifacts beside the rendered rules, each pointed at its OWN
    # generator rather than at the points.
    for name, target in (
        ("INDEX.md", "make ze-rules-index-update"),
        ("TRIGGERS.md", "make ze-rules-condensed-update"),
        ("CORE.md", "make ze-rules-condensed-update"),
    ):
        code, err = _writeedit(os.path.join(rules, name))
        results.check(
            f"rendered-rule-{name.split('.')[0].lower()}-refused",
            code == 2 and target in err,
            repr((code, err)),
        )

    # A relative path resolves against PROJECT_DIR, so the check cannot fail OPEN
    # for a caller whose CWD is not the project root.
    code, err = _writeedit(os.path.join("ai", "rules", "performance.md"))
    results.check("rendered-rule-relative-path-refused", code == 2, repr((code, err)))

    # Write, not only Edit.
    code, err = _writeedit(os.path.join(rules, "testing.md"), tool="Write")
    results.check("rendered-rule-write-refused", code == 2, repr((code, err)))

    # Discrimination: a same-named rule in ANOTHER checkout is not this project's
    # generated file, exactly as c_generated_files documents for CLAUDE.md.
    code, err = _writeedit("/nonexistent-checkout/ai/rules/performance.md")
    results.check("rendered-rule-other-checkout-allowed", code == 0, repr((code, err)))

    # Discrimination: nothing outside ai/rules/ is touched.
    code, err = _writeedit(
        os.path.join(ROOT, "docs", "contributing", "writing-style.md")
    )
    results.check("rendered-rule-unrelated-doc-allowed", code == 0, repr((code, err)))

    # c_rendered_rules must not BLOCK a .claude/rules/ file: only ai/rules/ holds
    # rendered rules. Since 2026-08-18 the same path draws a WARN from
    # c_claude_tree_has_ai_home, so the aggregate is 1 rather than 0. Asserted as
    # exactly 1 with the message named, not as `!= 2`, which a later unrelated
    # warning would satisfy without anyone noticing this one had stopped firing.
    code, err = _writeedit(os.path.join(ROOT, ".claude", "rules", "planning.md"))
    results.check(
        "rendered-rule-claude-rules-allowed",
        code == 1 and "shared home" in err,
        repr((code, err)),
    )

    # c_claude_tree_has_ai_home: the population is DERIVED -- `.claude/<sub>/`
    # warns exactly when `ai/<sub>/` exists. Both halves are pinned, because a
    # check that warned on everything and one that warned on nothing would each
    # pass a one-sided test.
    for sub, leaf in (("rules", "x.md"), ("skills", "x/SKILL.md"), ("agents", "x.md")):
        code, err = _writeedit(os.path.join(ROOT, ".claude", sub, leaf))
        results.check(
            f"claude-tree-warns-for-{sub}",
            code == 1 and f"ai/{sub}/" in err,
            repr((code, err)),
        )
    for sub, leaf in (
        ("hooks", "x.py"),
        ("plan", "ze-plan-x"),
        ("output-styles", "x.md"),
    ):
        code, err = _writeedit(os.path.join(ROOT, ".claude", sub, leaf))
        results.check(
            f"claude-tree-silent-for-{sub}",
            "shared home" not in err,
            repr((code, err)),
        )

    # A non-markdown file in ai/rules/ is not a rendered rule.
    code, err = _writeedit(os.path.join(rules, "notes.txt"))
    results.check("rendered-rule-non-md-allowed", code == 0, repr((code, err)))

    # --- c_point_overwrite: a Write over an existing point destroys it --------
    #
    # The point files are the canonical source. `write_split` in
    # scripts/dev/rules_points.py refuses this same move and cites
    # ai/rules/never-destroy-work.md; before this check the hook permitted it,
    # and one point was clobbered and recovered only from git.
    existing = os.path.join(rules, "points", "performance", "manifest.md")
    code, err = _writeedit(existing, tool="Write")
    results.check(
        "point-overwrite-write-refused",
        code == 2 and "already exists" in err,
        repr((code, err)),
    )
    results.check(
        "point-overwrite-names-both-routes",
        "Edit ai/rules/points/performance/manifest.md" in err and "a-free-slug" in err,
        repr(err),
    )

    # The other canonical shape: a point inside its `##` section directory. The
    # manifest is one level up from a point, so a depth test written for one
    # shape silently permits an overwrite of the other.
    point = os.path.join(
        rules,
        "points",
        "performance",
        "hot-path-rule",
        "apply-the-hot-path-ban-to-these-packages.md",
    )
    code, err = _writeedit(point, tool="Write")
    results.check(
        "point-overwrite-section-point-refused",
        code == 2 and "one instruction of the rule" in err,
        repr((code, err)),
    )
    results.check(
        "point-overwrite-section-point-names-its-path",
        "ai/rules/points/performance/hot-path-rule/"
        "apply-the-hot-path-ban-to-these-packages.md" in err,
        repr(err),
    )

    code, err = _writeedit(point, tool="Edit")
    results.check(
        "point-overwrite-section-point-edit-allowed", code == 0, repr((code, err))
    )

    # Discrimination, four ways. Each one is the same path shape with ONE thing
    # changed, so a check that refused everything under points/ would fail here.
    code, err = _writeedit(
        os.path.join(
            rules, "points", "performance", "hot-path-rule", "no-such-slug-exists.md"
        ),
        tool="Write",
    )
    results.check("point-overwrite-new-slug-allowed", code == 0, repr((code, err)))

    code, err = _writeedit(existing, tool="Edit")
    results.check("point-overwrite-edit-allowed", code == 0, repr((code, err)))

    code, err = _writeedit(
        "/nonexistent-checkout/ai/rules/points/performance/manifest.md", tool="Write"
    )
    results.check(
        "point-overwrite-other-checkout-allowed", code == 0, repr((code, err))
    )

    # One level too deep for a point, and a loose `*.md` at the rule level that
    # is not the manifest: neither is a canonical source, so neither is refused.
    code, err = _writeedit(
        os.path.join(
            rules, "points", "performance", "hot-path-rule", "nested", "manifest.md"
        ),
        tool="Write",
    )
    results.check("point-overwrite-nested-path-allowed", code == 0, repr((code, err)))

    code, err = _writeedit(
        os.path.join(rules, "points", "performance", "loose-point.md"), tool="Write"
    )
    results.check("point-overwrite-loose-md-allowed", code == 0, repr((code, err)))

    # --- the spelling never decides the verdict; the filesystem does ----------
    #
    # `os.path.realpath` resolves symlinks and NOT case, so on a case-insensitive
    # volume every one of these paths opens the file the check exists to protect
    # and a string comparison exited 0 on all of them. One fixture per varied
    # SEGMENT, because a fix applied to one component only is the shape a
    # refactor produces.
    #
    # The expectation follows the filesystem rather than the platform. Where one
    # spelling opens the file, refusing it is the whole point; where it opens
    # nothing, refusing it would be a false block on a file that is genuinely
    # different. Both are asserted by the same rows.
    insensitive = _case_insensitive(rules)
    variant_expect = 2 if insensitive else 0
    how = "case-insensitive" if insensitive else "case-sensitive"

    point_rel = (
        "ai/rules/points/performance/hot-path-rule/"
        "apply-the-hot-path-ban-to-these-packages.md"
    )
    # The varied segment decides which check answers when the file is genuinely
    # new. Every row below varies a DIRECTORY, so the basename stays kebab-case
    # and c_enforce_naming has nothing to say; the slug row varies the BASENAME
    # and is asserted separately under it.
    for label, varied in (
        ("ai", point_rel.replace("ai/", "AI/", 1)),
        ("rules", point_rel.replace("rules/", "RULES/", 1)),
        ("points", point_rel.replace("points/", "Points/", 1)),
        ("rule-dir", point_rel.replace("performance/", "Performance/", 1)),
        ("section-dir", point_rel.replace("hot-path-rule/", "Hot-Path-Rule/", 1)),
    ):
        code, err = _writeedit(os.path.join(ROOT, varied), tool="Write")
        results.check(
            f"point-overwrite-case-variant-{label}",
            code == variant_expect,
            f"{how} fs, wanted {variant_expect}: {(code, err)!r}",
        )

    # The slug row is the one whose variance lands in the FILENAME, so two
    # checks own it and the filesystem picks which one answers. On a
    # case-insensitive volume the path opens the existing point and
    # c_point_overwrite refuses it with 2, before naming is ever consulted. On a
    # case-sensitive volume it names a file that does not exist, c_point_overwrite
    # permits it as a new point, and c_enforce_naming refuses the capitals with 1
    # because a point slug is lowercase kebab-case. Both refusals are correct and
    # neither is the other's fallback, so the row asserts the code the filesystem
    # earns rather than a single number.
    slug_expect = 2 if insensitive else 1
    code, err = _writeedit(
        os.path.join(ROOT, point_rel.replace("apply-the-hot", "Apply-The-Hot", 1)),
        tool="Write",
    )
    results.check(
        "point-overwrite-case-variant-slug",
        code == slug_expect,
        f"{how} fs, wanted {slug_expect}: {(code, err)!r}",
    )

    for label, varied in (
        ("ai", "AI/rules/performance.md"),
        ("rules", "ai/RULES/performance.md"),
    ):
        code, err = _writeedit(os.path.join(ROOT, varied))
        results.check(
            f"rendered-rule-case-variant-{label}",
            code == variant_expect,
            f"{how} fs, wanted {variant_expect}: {(code, err)!r}",
        )

    # The rendered-rule branch routes on the ON-DISK name, so a case variant of
    # a file directly in ai/rules/ is sent to the generator that owns it rather
    # than to a points directory that does not exist.
    code, err = _writeedit(os.path.join(rules, "index.md"))
    results.check(
        "rendered-rule-case-variant-names-the-real-generator",
        code == 2 and ("make ze-rules-index-update" in err if insensitive else True),
        repr((code, err)),
    )

    # MultiEdit with an empty old_string replaces the whole file, which is a
    # Write wearing another name. The previous docstring asserted MultiEdit
    # could not drop a body; nothing proved it, and this shape disproved it.
    code, err = _multiedit(point, [{"old_string": "", "new_string": "gone"}])
    results.check(
        "point-overwrite-multiedit-empty-old-string-refused",
        code == 2 and "already exists" in err,
        repr((code, err)),
    )
    code, err = _multiedit(
        point, [{"old_string": "the hot path", "new_string": "the hot path"}]
    )
    results.check(
        "point-overwrite-multiedit-targeted-allowed", code == 0, repr((code, err))
    )

    # The two checks must be REACHED by the dispatcher. The fixtures above already
    # go through it; these name the cause directly when one of them goes red.
    mod = _load_pretool_writeedit()
    for check in ("c_rendered_rules", "c_point_overwrite"):
        results.check(
            f"{check}-wired-into-CHECKS",
            getattr(mod, check) in mod.CHECKS,
            f"{check} is not in pretool-writeedit.CHECKS, so it never runs",
        )


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

- [ ] `internal/x/y.go` <!-- doc-links: ignore (fixture spec body, deliberately absent) -->
  @ARROW@ Constraint: fixture only.

## Current Behavior

- [ ] `internal/x/y.go` <!-- doc-links: ignore (fixture spec body, deliberately absent) -->

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

- `internal/x/y.go` - fixture feature file <!-- doc-links: ignore (fixture spec body, deliberately absent) -->

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | No | fixture spec adds no command |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | fixture spec ships nothing |

## Implementation Steps

1. Implement the handler.

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Tests PASS
- [ ] make ze-standard-test passes
"""


def _run_validate_spec(
    script: str, spec_text: str, *, argv=None, payload=_UNSET, sources=None
):
    """Drive validate-spec.sh over a crafted spec.

    argv/payload let a case bypass the normal JSON-stdin call to assert the
    absent-tool-name refusal: argv=[spec] sends NO stdin, mimicking the
    `validate-spec.sh plan/spec-foo.md` invocation that used to exit 0 without
    running a single check.

    sources maps a repo-relative source path to the `// Design:` document it
    declares, materializing that file in the fake root. The design-document owner
    check reads the header from the file itself, so a case that wants the check to
    FIRE has to provide one. Cases that pass no sources exercise the same code path
    with nothing to find, which is why they stay green.
    """
    work = tempfile.mkdtemp(prefix="validate-spec-", dir=_fixture_root())
    try:
        plan = os.path.join(work, "plan")
        os.makedirs(plan, exist_ok=True)
        fp = os.path.join(plan, "spec-fixture.md")
        with open(fp, "w", encoding="utf-8") as fh:
            fh.write(spec_text)
        # The owner check runs a repo script against a repo-generated index. The
        # fake root has neither, and a hook that silently skips when its checker is
        # missing is the fail-open this check exists to prevent -- so the harness
        # provides both rather than the hook tolerating their absence.
        here = os.path.dirname(os.path.abspath(__file__))
        os.makedirs(os.path.join(work, "scripts", "dev"), exist_ok=True)
        shutil.copy(
            os.path.join(here, "spec_doc_anchors.py"),
            os.path.join(work, "scripts", "dev", "spec_doc_anchors.py"),
        )
        os.makedirs(os.path.join(work, "ai"), exist_ok=True)
        with open(
            os.path.join(work, "ai", "CODE-TO-DOCS.md"), "w", encoding="utf-8"
        ) as fh:
            fh.write(
                "# CODE-TO-DOCS\n\n## `internal/fixture/`\n\n| File | Docs |\n|---|---|\n| `z.go` | `docs/architecture/fixture.md` |\n"
            )
        for path, design in (sources or {}).items():
            full = os.path.join(work, path)
            os.makedirs(os.path.dirname(full), exist_ok=True)
            with open(full, "w", encoding="utf-8") as fh:
                fh.write(f"// Design: {design} -- fixture\n\npackage fixture\n")
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
    # only mean no check ran. See ai/rules/evidence.md.
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
    _CB = "## Current Behavior\n\n- [ ] `internal/x/y.go`"  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->

    # T-1 (AC-1): a citation carrying a line number is the form evidence.md
    # mandates; the old regex required the backtick to END in the extension, so a
    # trailing :line defeated the match. Must now be ACCEPTED.
    rc, err = _run_validate_spec(
        script,
        base.replace(
            _CB,
            "## Current Behavior\n\n- [ ] `scripts/dev/foo.py:42`",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        ),  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
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
            "## Current Behavior\n\n- [ ] `.claude/hooks/foo.sh`\n- [ ] `Makefile:12`",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
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
            "## Current Behavior\n\n"
            + long_preamble
            + "\n\n- [ ] `internal/x/y.go`",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
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
            "- `internal/x/y.go` - fixture feature file",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
            "- `scripts/dev/foo.py` - fixture tooling file",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        )
        .replace(
            "| foo.ci | test/x/ | user runs foo | |",
            "| hook fixtures | `scripts/dev/hook-fixture-check.py` | fixtures drive the hook | |",
        )
        .replace(
            _CB,
            "## Current Behavior\n\n- [ ] `scripts/dev/foo.py`",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        )  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
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
        "| foo unit | `internal/x/y_test.go` | user runs foo | |",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    rc, err = _run_validate_spec(script, daemon)
    results.check(
        "validate-spec-daemon-still-needs-ci",
        rc == 2 and "must reference a functional test file" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-5 MUST-NOT-FIRE (AC-8, review NIT-2): a daemon spec must NOT be able to
    # take the tooling escape by naming a .py surface -- the TOUCHES_DAEMON guard
    # blocks it. internal/x/y.go stays in Files to Modify, so it is a daemon spec.
    daemon_py = base.replace(
        "| foo.ci | test/x/ | user runs foo | |",
        "| py surface | `scripts/dev/foo.py` | user runs foo | |",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    rc, err = _run_validate_spec(script, daemon_py)
    results.check(
        "validate-spec-daemon-py-surface-still-rejected",
        rc == 2 and "must reference a functional test file" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # T-1 MUST-NOT-FIRE (AC-7, review ISSUE-1): an empty-basename citation `.go`
    # is garbage, not a real source path, and must be REJECTED. The basename `+`
    # (not `*`) closes the zero-value-looks-valid hole (evidence.md).
    rc, err = _run_validate_spec(
        script, base.replace(_CB, "## Current Behavior\n\n- [ ] `.go`")
    )
    results.check(
        "validate-spec-empty-basename-citation-rejected",
        rc == 2 and "list source files read" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # --- verification command: ONE gate, `make ze-precommit-verify` -------------------
    # The template used to ship three spellings at once and this hook demanded
    # the fuzz-inclusive `ze-standard-test` target, which is NOT the pre-commit gate
    # (ai/rules/git-safety.md). `ze-precommit-verify` is clean; the legacy string still
    # passes (50 specs predate the change) but warns; neither is an error.
    _LEGACY_LINE = "- [ ] make ze-standard-test passes"
    _VERIFY_LINE = "- [ ] `make ze-precommit-verify` passes"

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
    # leave the rest (ai/rules/planning.md). Blocking its placeholders
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

    # --- design-document owner check ----------------------------------------
    # A changed .go file declares its design doc in a `// Design:` header. A spec
    # that changes the file and never names that doc ships a design change with
    # its design unwritten. This is the shape that got through:
    # plan/spec-streaming-answer-protocol.md changed pkg/plugin/rpc/message.go and
    # mux.go, both declaring docs/architecture/api/ipc_protocol.md, and named two
    # OTHER docs while answering the checklist row "Yes".
    owner_spec = _VALID_SPEC.replace("@ARROW@", "->").replace(
        "- `internal/x/y.go` - fixture feature file",
        "- `internal/fixture/z.go` - fixture feature file",
    )
    rc, err = _run_validate_spec(
        script,
        owner_spec,
        sources={"internal/fixture/z.go": "docs/architecture/fixture.md"},
    )
    results.check(
        "validate-spec-undeclared-design-doc-blocks",
        rc == 2 and "docs/architecture/fixture.md" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # MUST-NOT-FIRE: naming the doc anywhere satisfies it. The requirement is that
    # the author LOOKED, so a checklist row explaining why it is unaffected counts
    # exactly as much as listing it for edit. Without this case the check could
    # demand an edit nobody needs and get worked around.
    rc, err = _run_validate_spec(
        script,
        owner_spec.replace(
            "- `internal/fixture/z.go` - fixture feature file",
            "- `internal/fixture/z.go` - fixture feature file\n"
            "- `docs/architecture/fixture.md` - unaffected, states framing only",
        ),
        sources={"internal/fixture/z.go": "docs/architecture/fixture.md"},
    )
    results.check(
        "validate-spec-named-design-doc-accepted",
        rc == 0 or "docs/architecture/fixture.md" not in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # --- the two-session handoff status -------------------------------------
    # `verification` says the implementation is complete and COMMITTED and the
    # spec awaits an independent review (ai/rules/planning.md, "Two-Session
    # Handoff"). It sits between in-progress and done, so the Status check must
    # accept it...
    rc, err = _run_validate_spec(
        script, base.replace("| Status | in-progress |", "| Status | verification |")
    )
    results.check(
        "validate-spec-verification-status-accepted",
        rc == 0 and "Invalid Status" not in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # ...and MUST NOT gain the skeleton placeholder licence by doing so. A
    # placeholder at `verification` is a spec claiming committed code over an
    # unwritten section.
    rc, err = _run_validate_spec(
        script,
        _placeholder.replace("| Status | in-progress |", "| Status | verification |"),
    )
    results.check(
        "validate-spec-verification-placeholder-blocks",
        rc == 2 and "Entry Point contains placeholder" in err,
        f"rc={rc} err={err[:200]!r}",
    )

    # --- the two checklists plan/TEMPLATE.md ships --------------------------
    # "Documentation Update Checklist (BLOCKING)" bound a reader and nothing
    # else: REQUIRED_SECTIONS named neither it nor the Integration Checklist, so
    # no check read either. They are status-aware for the same reason the
    # placeholder guards are -- a skeleton fills only `## Task`.
    _no_doc_checklist = base.replace(
        "### Documentation Update Checklist (BLOCKING)", "### Something Else"
    )
    rc, err = _run_validate_spec(
        script,
        _no_doc_checklist.replace("| Status | in-progress |", "| Status | design |"),
    )
    results.check(
        "validate-spec-design-missing-doc-checklist-warns",
        rc == 0 and "Documentation Update Checklist" in err,
        f"rc={rc} err={err[:300]!r}",
    )

    rc, err = _run_validate_spec(
        script,
        _no_doc_checklist.replace("| Status | in-progress |", "| Status | skeleton |"),
    )
    results.check(
        "validate-spec-skeleton-missing-doc-checklist-warns",
        rc == 0 and "warning" in err.lower(),
        f"rc={rc} err={err[:300]!r}",
    )

    rc, err = _run_validate_spec(
        script,
        base.replace("### Integration Checklist", "### Something Else").replace(
            "| Status | in-progress |", "| Status | design |"
        ),
    )
    results.check(
        "validate-spec-design-missing-integration-checklist-warns",
        rc == 0 and "Integration Checklist" in err,
        f"rc={rc} err={err[:300]!r}",
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
    """A fixture repository shaped like a Ze source checkout.

    The `plan/` tree is load-bearing, not decoration: `deferral_in_diff_problems`
    returns early for a repository that holds none, because a repository with no
    `plan/` tree cannot record a deferral and the gate would then have no
    reachable satisfying action there (`8f2ae417a`). A fixture without one models
    the published website rather than a source checkout, and every deferral case
    built on it passes while asserting nothing.
    """
    repo = tempfile.mkdtemp(prefix="commit-gate-", dir=_fixture_root())
    _git(repo, "init", "-q")
    _git(repo, "config", "user.email", "fixture@example.com")
    _git(repo, "config", "user.name", "fixture")
    _git(repo, "config", "commit.gpgsign", "false")
    os.makedirs(os.path.join(repo, "plan", "deferrals"), exist_ok=True)
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

# Five-column layout matching a real plan/journal/<class>.md (Date | Spec |
# Surface | Symptom | Fix); the closure gates read the Spec cell by index.
_JOURNAL_HEADER = "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"


def _journal_row(spec: str) -> str:
    """One well-formed journal row naming `spec` in its Spec cell."""
    return f"| 2026-08-09 | {spec} | surface | symptom | fix |\n"


def _design_go(topic: str) -> str:
    """A Go file whose `// Design:` header is a DOCS-TO-CODE source row."""
    return f"// Design: docs/{topic}.md -- {topic}\npackage {topic}\n"


def _package_go(topic: str) -> str:
    """A Go file whose `// Package` doc comment is a PACKAGE-MAP source row."""
    return f"// Package {topic} does a thing.\npackage {topic}\n"


def _seed_index_repo(repo: str, extra_generators: tuple[str, ...] = ()) -> None:
    """A fixture repo the discovery-index gate can actually run in.

    It copies the generator discovery_sources.GENERATORS actually names, plus
    discovery_sources.py which that generator imports. Seeding any other script
    judges nothing: discovery_index_freshness zips GENERATORS with OUTPUTS and
    SKIPS a generator the tree does not carry, so a fixture that seeds a script
    outside that tuple leaves the state "unknown" and the gate returns no
    problems at all. Four cases here asserted a block and got an empty list for
    exactly that reason, after PACKAGE-MAP replaced DOCS-TO-CODE as the one
    discovery index and this helper kept seeding the old generator.

    The source it seeds carries a `// Package` doc comment, which is what
    PACKAGE-MAP rows are built from. It used to be a `plan/learned/NNN-*.md`
    summary feeding a learned index; that corpus and its generator are gone
    (plan/spec-problem-journal.md). The journal that replaced the corpus
    generates NO index by design, so no `plan/journal/` file can stand in here:
    what these cases exercise is index FRESHNESS, and an index is what the
    journal deliberately does not have. The journal's own commit-gate coverage
    is the spec-audit and closure-stem cases above, which need no generator.
    """
    gens = (
        "scripts/dev/package_map.py",
        "scripts/dev/discovery_sources.py",
    ) + extra_generators
    for rel in gens:
        dst = os.path.join(repo, rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copyfile(os.path.join(DEV, os.path.basename(rel)), dst)
    _write(repo, "internal/alpha/a.go", _package_go("alpha"))
    os.makedirs(os.path.join(repo, "ai"), exist_ok=True)
    _regen_package_map(repo)
    _git(repo, "add", "scripts", "internal", "ai")
    _git(repo, "commit", "-q", "-m", "seed package-map index")


def _regen(repo: str, generator: str) -> None:
    subprocess.run(
        [sys.executable, os.path.join(repo, "scripts/dev", generator)],
        check=True,
        capture_output=True,
        text=True,
    )


def _regen_package_map(repo: str) -> None:
    _regen(repo, "package_map.py")


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
    # spellings resolve to the same file, across shards (ai/rules/planning.md).
    repo = _init_repo()
    try:
        _write(repo, "plan/spec-foo.md", "# Spec: foo\n")
        _write(
            repo,
            "plan/deferrals/foo.md",
            _DEFERRALS_HEADER
            + "| 2026-07-09 | abc | thing | reason | spec-foo.md | open |\n"
            + "| 2026-07-09 | abc | thing2 | reason | `plan/spec-foo.md` | open |\n",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
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

        # A commit that does NOT add this spec's closure artifact is not a closure
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

    # --- spec-audit through the JOURNAL, which is the live closure artifact ---
    # ai/skills/ze-close.md step 6a writes a plan/journal/<class>.md row, not a
    # learned summary, and plan/learned/ is gone. Keyed on the learned path alone
    # this gate could no longer fire on ANY closure. The cases below drive the
    # journal branch, and the last two are what make them discriminating: a row
    # naming ANOTHER spec, and a row already at HEAD, must both leave it silent.
    repo = _init_repo()
    try:
        _write(repo, "plan/spec-fixture.md", empty_pcv)
        _write(repo, "plan/journal/a-class.md", _JOURNAL_HEADER)
        _git(repo, "add", "plan")
        _git(repo, "commit", "-q", "-m", "seed journal class")

        _write(
            repo,
            "plan/journal/a-class.md",
            _JOURNAL_HEADER + _journal_row("fixture"),
        )
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/journal/a-class.md",), "spec-fixture.md"
        )
        results.check(
            "commit-gate-spec-audit-journal-row-blocks", bool(problems), repr(problems)
        )

        _write(repo, "plan/spec-fixture.md", filled_pcv)
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/journal/a-class.md",), "spec-fixture.md"
        )
        results.check(
            "commit-gate-spec-audit-journal-filled-ok", not problems, repr(problems)
        )

        # A row naming a DIFFERENT spec is somebody else's closure.
        _write(repo, "plan/spec-fixture.md", empty_pcv)
        _write(
            repo,
            "plan/journal/a-class.md",
            _JOURNAL_HEADER + _journal_row("other-spec"),
        )
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/journal/a-class.md",), "spec-fixture.md"
        )
        results.check(
            "commit-gate-spec-audit-journal-other-spec-skips",
            not problems,
            repr(problems),
        )

        # A row this spec's stem already owns AT HEAD is not added by this commit,
        # so it is not this commit's closure signal.
        _write(
            repo,
            "plan/journal/a-class.md",
            _JOURNAL_HEADER + _journal_row("fixture"),
        )
        _git(repo, "add", "plan")
        _git(repo, "commit", "-q", "-m", "commit the row")
        problems = ch.spec_audit_problems(
            Path(repo), ("plan/journal/a-class.md",), "spec-fixture.md"
        )
        results.check(
            "commit-gate-spec-audit-journal-row-at-head-skips",
            not problems,
            repr(problems),
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- journal-row: a MALFORMED added row BLOCKS, through create() ---
    # `_journal_added_spec_stems` skips a row it cannot parse. When the row is the
    # only closure artifact that leaves `spec_closure_stem` None, so
    # `review_gate_problems` returns [] and a closure commit carrying code lands
    # unreviewed: the miss path returned the permissive answer
    # (ai/rules/evidence.md). Driven from create(), because that is where the
    # commit is refused and where the ordering (this gate before the review gate)
    # is what makes the skip safe.
    repo = _init_repo()
    try:
        _write(repo, "plan/journal/a-class.md", _JOURNAL_HEADER)
        _git(repo, "add", "plan")
        _git(repo, "commit", "-q", "-m", "seed journal class")
        _write(
            repo,
            "plan/journal/a-class.md",
            _JOURNAL_HEADER + "| 2026-08-09 | fixture | surface | symptom |\n",
        )
        with contextlib.redirect_stderr(io.StringIO()):
            rc = ch.main(
                [
                    "--repo",
                    repo,
                    "create",
                    "--session",
                    "cafe1234",
                    "--subject",
                    "add a journal row missing a cell",
                    "--file",
                    "plan/journal/a-class.md",
                ]
            )
        script_exists = bool(
            glob.glob(os.path.join(repo, "tmp", "commit-cafe1234-*.sh"))
        )
        results.check(
            "commit-gate-journal-malformed-row-blocks-via-create",
            rc == 2 and not script_exists,
            f"rc={rc} script={script_exists}",
        )

        # The same commit with the fifth cell present is accepted, so the block
        # above is the row's shape and not the path.
        _write(
            repo,
            "plan/journal/a-class.md",
            _JOURNAL_HEADER + _journal_row("fixture"),
        )
        with contextlib.redirect_stderr(io.StringIO()):
            rc = ch.main(
                [
                    "--repo",
                    repo,
                    "create",
                    "--session",
                    "cafe5678",
                    "--subject",
                    "add a well-formed journal row",
                    "--file",
                    "plan/journal/a-class.md",
                ]
            )
        # create() names each script with a random suffix, so glob rather than
        # spell it: the negative cases above assert "no script at all" and a
        # literal name is enough for them.
        scripts = glob.glob(os.path.join(repo, "tmp", "commit-cafe5678-*.sh"))
        results.check(
            "commit-gate-journal-wellformed-row-passes",
            rc == 0 and bool(scripts),
            f"rc={rc} scripts={scripts}",
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
                ]
            )
        script_exists = bool(
            glob.glob(os.path.join(repo, "tmp", "commit-abcd1234-*.sh"))
        )
        results.check(
            "commit-gate-create-blocks-deferral",
            rc == 2 and not script_exists,
            f"rc={rc} script={script_exists}",
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: own staleness blocks, a concurrent session's does not ---
    # The gate judges the tree the commit PRODUCES (HEAD + adds - removes), not the
    # working tree, so an untracked source belonging to another session cannot
    # force a commit to either block or cross-commit that session's index row.
    repo = _init_repo()
    try:
        _seed_index_repo(repo)
        # The cases below run in sequence against ONE repo and each depends on the
        # state the previous one left. The order is load-bearing; inserting a case
        # changes what the ones after it test.
        #
        # A: this commit adds a source and omits the regenerated index -> block.
        # Asserts the MESSAGE, not merely that something blocked: the pre-change
        # implementation also blocked here, by a different branch.
        _write(repo, "internal/beta/b.go", _package_go("beta"))
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(Path(repo), ("internal/beta/b.go",))
        results.check(
            "commit-gate-index-own-staleness-blocks",
            bool(problems) and "omitted:" in "".join(problems),
            repr(problems),
        )

        # D (runs here deliberately): at THIS state the working tree is stale from
        # the source A left, so an unrelated commit is the case that proves a
        # concurrent session's staleness does not block. Run after B or C, where
        # the tree is fresh again, it would assert on a branch that returns early
        # and would pass with the whole change reverted.
        _write(repo, "docs/unrelated.md", "# unrelated\n")
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(Path(repo), ("docs/unrelated.md",))
        results.check(
            "commit-gate-index-unrelated-commit-passes", not problems, repr(problems)
        )

        # B: same commit, index regenerated to match HEAD + its own source, while a
        # concurrent session leaves an UNTRACKED source in the tree -> no block.
        _regen_package_map(repo)
        _write(repo, "internal/foreign/f.go", _package_go("foreign"))
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo),
                ("internal/beta/b.go", "ai/PACKAGE-MAP.md"),
            )
        results.check(
            "commit-gate-index-foreign-staleness-passes", not problems, repr(problems)
        )

        # C: the index is regenerated WITH the concurrent session's untracked
        # source and then committed -> it would publish a row for a file absent
        # from HEAD. A never-committed summary reached HEAD's committed index
        # this way, and a working-tree check calls this state "fresh".
        _regen_package_map(repo)
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo),
                ("internal/beta/b.go", "ai/PACKAGE-MAP.md"),
            )
        results.check(
            "commit-gate-index-foreign-row-included-blocks",
            bool(problems) and "included but wrong" in "".join(problems),
            repr(problems),
        )

    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- go-design-ref: the commit-time half of c_require_design_ref. The hook
    # reaches Write/Edit only, so a .go written from Bash meets no gate at all;
    # this one judges the file rather than the tool that produced it.
    #
    # Both halves are pinned. A refusing-only test passes for a gate that refuses
    # everything, and an exempting-only test passes for one that never fires --
    # which is the failure mode that matters here, since a silent gate looks
    # exactly like a clean tree.
    repo = _init_repo()
    try:
        _write(repo, "internal/alpha/a.go", "package alpha\n\nfunc F() {}\n")
        problems = ch.go_design_ref_problems(Path(repo), ("internal/alpha/a.go",))
        results.check(
            "commit-gate-go-design-ref-missing-blocks",
            bool(problems) and "// Design:" in "".join(problems),
            repr(problems),
        )

        _write(
            repo,
            "internal/alpha/b.go",
            "// Design: docs/architecture/x.md -- a thing\npackage alpha\n",
        )
        results.check(
            "commit-gate-go-design-ref-present-passes",
            not ch.go_design_ref_problems(Path(repo), ("internal/alpha/b.go",)),
            "a file carrying the header must pass",
        )

        # Every exemption c_require_design_ref applies. Kept in one loop so a
        # divergence between the hook and this gate shows up as a named failure
        # rather than as a commit nobody can make.
        for rel, body in (
            ("internal/alpha/a_test.go", "package alpha\n"),
            ("internal/alpha/a_gen.go", "package alpha\n"),
            ("internal/alpha/register.go", "package alpha\n"),
            ("internal/alpha/embed.go", "package alpha\n"),
            ("internal/alpha/doc.go", "package alpha\n"),
            ("vendor/example.com/x/x.go", "package x\n"),
            (
                "internal/alpha/g.go",
                "// Code generated by t; DO NOT EDIT.\npackage alpha\n",
            ),
        ):
            _write(repo, rel, body)
            results.check(
                f"commit-gate-go-design-ref-exempt-{os.path.basename(rel)}",
                not ch.go_design_ref_problems(Path(repo), (rel,)),
                rel,
            )

        # Not a .go file at all, and a .go the commit names but the tree does not
        # hold (a path removed in the same commit) must both stay silent.
        _write(repo, "docs/notes.md", "# notes\n")
        results.check(
            "commit-gate-go-design-ref-ignores-non-go",
            not ch.go_design_ref_problems(Path(repo), ("docs/notes.md",)),
            "only .go files are judged",
        )
        results.check(
            "commit-gate-go-design-ref-ignores-absent",
            not ch.go_design_ref_problems(Path(repo), ("internal/alpha/gone.go",)),
            "a path not on disk is not a missing header",
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- go-style: the content checks pretool-writeedit.py applies to Go,
    # re-run over the lines the commit ADDS. Same bypass as go-design-ref, a
    # different half of it: design-ref judges a file property, these judge the
    # code. c_panic is the one that matters most -- ze-style.md calls "a peer
    # MUST NOT be able to panic the daemon" the single most important line on
    # the page, and it was exactly as bypassable as the rest.
    #
    # The ADDED lines are the subject because std_content in the hook returns
    # "the text added by Write, Edit, or MultiEdit". Measured: over added lines
    # this set fires twice in the last 40 commits; over whole files it fires on
    # 1646 of 10212, which would gate on code the commit never touched.
    repo = _init_repo()
    try:
        _write(
            repo, "internal/alpha/a.go", "// Design: docs/a.md -- a\npackage alpha\n"
        )
        _git(repo, "add", "internal")
        _git(repo, "commit", "-q", "-m", "seed")

        # The real finding this gate was built from: a fmt.Printf an agent wrote
        # through a Bash heredoc, which reached HEAD because no hook saw it.
        _write(
            repo,
            "internal/alpha/a.go",
            "// Design: docs/a.md -- a\npackage alpha\n\n"
            'func F() { fmt.Printf("regenerated %q (%d years)\\n", n, y) }\n',
        )
        problems = ch.go_style_problems(Path(repo), ("internal/alpha/a.go",))
        results.check(
            "commit-gate-go-style-added-line-blocks",
            bool(problems) and "c_sprintf_new" in "".join(problems),
            repr(problems)[:160],
        )

        # A bare panic is refused; the documented BUG: prefix is not. Both halves,
        # because a gate that refused every panic would have no route to commit.
        _write(
            repo,
            "internal/alpha/p.go",
            '// Design: docs/a.md -- p\npackage alpha\n\nfunc P() { panic("boom") }\n',
        )
        problems = ch.go_style_problems(Path(repo), ("internal/alpha/p.go",))
        results.check(
            "commit-gate-go-style-panic-blocks",
            bool(problems) and "c_panic" in "".join(problems),
            repr(problems)[:160],
        )
        _write(
            repo,
            "internal/alpha/p.go",
            '// Design: docs/a.md -- p\npackage alpha\n\nfunc P() { panic("BUG: unreachable") }\n',
        )
        results.check(
            "commit-gate-go-style-bug-prefix-passes",
            not ch.go_style_problems(Path(repo), ("internal/alpha/p.go",)),
            "the documented prefix must stay committable",
        )

        # The scope. A test file, a non-Go path, and a Go file this commit does
        # not change must all produce nothing: the subject is the added lines.
        results.check(
            "commit-gate-go-style-test-file-ignored",
            not ch.go_style_problems(Path(repo), ("internal/alpha/a_test.go",)),
            "_test.go is out of scope",
        )
        results.check(
            "commit-gate-go-style-non-go-ignored",
            not ch.go_style_problems(Path(repo), ("docs/notes.md",)),
            "non-Go is out of scope",
        )

        # The exemptions these checks carry must mean the same thing here as at
        # write time. Four of them test a path form with a LEADING slash
        # (`"/scripts/" in fp`), which the absolute path the Edit hook receives
        # satisfies and a repo-relative add_path does not. Left diverged, a
        # `scripts/*.go` file the hook waves through cannot be committed at all,
        # and `commit_gate_problems` offers no override flag for it.
        _write(
            repo,
            "scripts/checks/tool.go",
            "// Design: docs/a.md -- tool\npackage main\n\n"
            'func F() { os.Exit(1); panic("boom") }\n',
        )
        results.check(
            "commit-gate-go-style-scripts-exempt",
            not ch.go_style_problems(Path(repo), ("scripts/checks/tool.go",)),
            "scripts/ is exempt at write time and must be exempt here",
        )
        # The other half: the exemption is scripts/, not everywhere. Without
        # this, passing a form that matched nothing would read as a pass.
        _write(
            repo,
            "internal/alpha/x.go",
            "// Design: docs/a.md -- x\npackage alpha\n\nfunc F() { os.Exit(1) }\n",
        )
        results.check(
            "commit-gate-go-style-non-scripts-still-fires",
            "c_os_exit"
            in "".join(ch.go_style_problems(Path(repo), ("internal/alpha/x.go",))),
            "os.Exit outside scripts/ must still be refused",
        )

        # Driven from the ENTRY POINT, not the helper. Deleting the
        # `go_style_problems` call in commit_gate_problems leaves every check
        # above green, which is the shape `ai/rules/evidence.md` refuses.
        results.check(
            "commit-gate-go-style-reached-from-commit-gate",
            "c_os_exit"
            in "".join(
                ch.commit_gate_problems(Path(repo), ("internal/alpha/x.go",), ())
            ),
            "commit_gate_problems must run the Go content checks",
        )

        # A name that stops resolving is an error, never a silent skip: renaming
        # a check in the hook would otherwise leave this gate reporting clean.
        saved = ch.GO_CONTENT_CHECKS
        ch.GO_CONTENT_CHECKS = saved + ("c_no_such_check_exists",)
        try:
            ch.go_style_problems(Path(repo), ("internal/alpha/x.go",))
            raised = False
        except ch.UsageError:
            raised = True
        finally:
            ch.GO_CONTENT_CHECKS = saved
        results.check(
            "commit-gate-go-style-missing-check-is-loud",
            raised,
            "a vanished check must raise",
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: an index the commit does not visibly FEED is still
    # verified. `indexes_fed_by` recognises a PACKAGE-MAP source by a `// Package`
    # header or a register.go name, so a new .go carrying only `// Design:` feeds
    # NOTHING by that rule -- yet package_map keys its rows on DIRECTORY
    # existence, so the new package drifts the map anyway. A gate that verified
    # only the indexes a commit feeds would pass this and publish a stale map.
    repo = _init_repo()
    try:
        _seed_index_repo(repo)
        _write(
            repo,
            "internal/existing/a.go",
            "// Package existing does a thing.\npackage existing\n",
        )
        _regen_package_map(repo)
        _git(repo, "add", "internal", "ai")
        _git(repo, "commit", "-q", "-m", "seed package map")

        # The new file feeds no index by `indexes_fed_by`, yet it adds a
        # PACKAGE-MAP row. The author regenerated the map, so the WORKING TREE is
        # fresh, and --file'd only the source: only the commit view (HEAD plus
        # this commit's own files) can see that the committed map will not match
        # the committed tree.
        _write(
            repo,
            "internal/newpkg/thing.go",
            "// Design: docs/x.md -- thing\npackage newpkg\n",
        )
        _regen_package_map(repo)
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
        _seed_index_repo(repo)
        _write(repo, "internal/beta/b.go", _package_go("beta"))
        _regen_package_map(repo)
        _git(repo, "add", "internal", "ai")
        _git(repo, "commit", "-q", "-m", "add beta")
        os.remove(os.path.join(repo, "internal/beta/b.go"))

        # E1: removal committed, index left listing the removed source -> block.
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), (), ("internal/beta/b.go",)
            )
        results.check(
            "commit-gate-index-removal-stale-blocks", bool(problems), repr(problems)
        )

        # E2: same removal with the regenerated index riding along -> passes.
        _regen_package_map(repo)
        with contextlib.redirect_stderr(io.StringIO()):
            problems = ch.discovery_index_problems(
                Path(repo), ("ai/PACKAGE-MAP.md",), ("internal/beta/b.go",)
            )
        results.check(
            "commit-gate-index-removal-regenerated-passes", not problems, repr(problems)
        )
    finally:
        shutil.rmtree(repo, ignore_errors=True)

    # --- discovery-index: the ENTRY POINT passes remove_paths through ---
    # E1/E2 above call discovery_index_problems directly, so dropping the argument
    # at create()'s call site leaves them green. Drive the guard from where a user
    # reaches it (ai/rules/evidence.md).
    repo = _init_repo()
    try:
        _seed_index_repo(repo)
        _write(repo, "internal/beta/b.go", _package_go("beta"))
        _regen_package_map(repo)
        _git(repo, "add", "internal", "ai")
        _git(repo, "commit", "-q", "-m", "add beta")
        os.remove(os.path.join(repo, "internal/beta/b.go"))
        with contextlib.redirect_stderr(io.StringIO()):
            rc = ch.main(
                [
                    "--repo",
                    repo,
                    "create",
                    "--session",
                    "beef1234",
                    "--subject",
                    "remove a source without refreshing the index",
                    "--remove",
                    "internal/beta/b.go",
                ]
            )
        script_exists = bool(
            glob.glob(os.path.join(repo, "tmp", "commit-beef1234-*.sh"))
        )
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
        a1 = mod._mint_cached(cache, "111-4242")
        a2 = mod._mint_cached(cache, "111-4242")  # same key -> stable
        b1 = mod._mint_cached(cache, "222-4242")  # distinct key -> unique
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
        empty = os.path.join(cache, ".sid-by-pid-333-4242")
        with open(empty, "w"):
            pass  # zero bytes, exactly what a crashed O_EXCL create leaves
        c1 = mod._mint_cached(cache, "333-4242")
        c2 = mod._mint_cached(cache, "333-4242")
        results.check(
            "session-id-mint-heals-empty-cache",
            c1 == c2
            and _UUID_RE.match(c1) is not None
            and c1 != "claude-session-fallback",
            f"c1={c1!r} c2={c2!r}",
        )
    finally:
        shutil.rmtree(cache, ignore_errors=True)


def _run_session_id_cache_key(results: Results) -> None:
    """AC-17/AC-18: the minted-id cache is keyed on the PID *and* its START TIME.

    R-9: the key used to be the CLI-ancestor PID alone, and only the 24h age sweep
    stopped a reused PID from reading a DEAD session's id -- with it, that session's
    spec claim and gate markers (incidents 1162, 1246). Nothing under tmp/session/
    ages out any more, so the invalidation moved into the key itself: a reused PID
    carries a different start time, so the stale entry is never looked up again.

    The reuse case cannot be produced by waiting for the kernel to hand back a PID,
    so the dead session's start time is substituted for the length of one mint. The
    LIVE half uses the real reader, on both platforms' branches.
    """
    mod = _load_session_id_module()
    pid = os.getpid()
    cache = tempfile.mkdtemp(prefix="ze-sid-key-", dir=_fixture_root())
    try:
        # AC-18, Linux branch: /proc/<pid>/stat field 22 answers for a live process,
        # is usable as a filename component, and does not move between two calls.
        live = mod._pstart(pid)
        results.check(
            "session-id-start-time-readable",
            bool(live) and mod._sid_safe(live) == live and live == mod._pstart(pid),
            f"live={live!r}",
        )
        # AC-18, macOS/BSD branch: `ps -o lstart=` answers for the same process and
        # is equally stable once squeezed to a token. One runner cannot be on both
        # platforms, so the branch the other one takes is exercised directly rather
        # than left to the reader's confidence.
        ps_start = mod._path_token(mod._ps("lstart=", pid))
        results.check(
            "session-id-start-time-ps-branch",
            bool(ps_start)
            and mod._sid_safe(ps_start) == ps_start
            and ps_start == mod._path_token(mod._ps("lstart=", pid)),
            f"ps_start={ps_start!r}",
        )

        # AC-18: a LIVE session hits its own cache entry and resolves the same id.
        key = mod._cache_key(pid)
        results.check(
            "session-id-cache-key-carries-start-time",
            key == f"{pid}-{live}",
            f"key={key!r} pid={pid} live={live!r}",
        )
        first = mod._mint_cached(cache, key)
        second = mod._mint_cached(cache, mod._cache_key(pid))
        results.check(
            "session-id-live-cache-hit-stable",
            first == second and _UUID_RE.match(first) is not None,
            f"first={first!r} second={second!r}",
        )
        results.check(
            "session-id-cache-file-named-by-key",
            os.path.isfile(os.path.join(cache, f".sid-by-pid-{key}")),
            f"key={key!r} files={sorted(os.listdir(cache))}",
        )

        # AC-17: PID REUSE. The dead session held this PID with an earlier start
        # time, and its cache entry is still on disk because nothing sweeps
        # tmp/session/ any more. The new session MUST mint a fresh id.
        real_pstart = mod._pstart
        try:
            mod._pstart = lambda _pid: "1"  # the dead session's start time
            dead_key = mod._cache_key(pid)
            dead_id = mod._mint_cached(cache, dead_key)
        finally:
            mod._pstart = real_pstart
        live_id = mod._mint_cached(cache, mod._cache_key(pid))
        results.check(
            "session-id-reused-pid-mints-fresh-id",
            dead_key != key and dead_id != live_id,
            f"dead_key={dead_key!r} live_key={key!r} dead={dead_id!r} live={live_id!r}",
        )
        # The dead entry is not deleted, only unreachable: this fix replaces an
        # expiry sweep, it does not reintroduce one under another name.
        results.check(
            "session-id-reused-pid-leaves-dead-entry",
            mod._read_cached(os.path.join(cache, f".sid-by-pid-{dead_key}")) == dead_id,
            f"dead_key={dead_key!r} dead={dead_id!r}",
        )
    finally:
        shutil.rmtree(cache, ignore_errors=True)


def _fork_payload_env(work: str, project_dir: str | None = None) -> dict:
    """Fork environment with no inherited identity and no usable ps command."""
    deny_bin = os.path.join(work, "deny-bin")
    os.makedirs(deny_bin, exist_ok=True)
    ps = os.path.join(deny_bin, "ps")
    with open(ps, "w", encoding="utf-8") as fh:
        fh.write("#!/bin/sh\nexit 126\n")
    os.chmod(ps, 0o755)

    env = dict(os.environ)
    for name in (
        "CLAUDE_CODE_SESSION_ID",
        "CLAUDE_CODE_SESSION_ACCESS_TOKEN",
        "ZE_SESSION_ID",
        "CLAUDE_ENV_FILE",
    ):
        env.pop(name, None)
    env.update(
        {
            "CLAUDE_CODE_FORK_SUBAGENT": "1",
            "CLAUDE_PROJECT_DIR": project_dir or work,
            "PATH": deny_bin + os.pathsep + env["PATH"],
        }
    )
    return env


# VALIDATES: SessionStart accepts a payload id only when the raw JSON string is
# already a safe, non-dot filename component.
# PREVENTS: jq and shell command substitution deleting a trailing newline or
# encoded NUL, then persisting a different id than the payload supplied.
def _run_session_start_raw_payload_ids(results: Results) -> None:
    work = _deleg_project(spec=None)
    try:
        env_file = os.path.join(work, "persistent-env")
        env = _fork_payload_env(work)
        env["CLAUDE_ENV_FILE"] = env_file
        for label, sid in (
            ("trailing-newline", _DELEG_SID + "\n"),
            ("encoded-nul", _DELEG_SID + "\0suffix"),
            ("dot", "."),
            ("dot-dot", ".."),
        ):
            with open(env_file, "w", encoding="utf-8"):
                pass
            proc = subprocess.run(
                ["bash", os.path.join(HOOKS, "session-start.sh")],
                input=json.dumps(
                    {
                        "hook_event_name": "SessionStart",
                        "session_id": sid,
                        "source": "startup",
                    }
                ),
                capture_output=True,
                text=True,
                env=env,
                timeout=60,
            )
            with open(env_file, encoding="utf-8") as fh:
                persisted = fh.read()
            results.check(
                f"session-start-rejects-raw-{label}-id",
                proc.returncode == 0 and persisted == "",
                f"rc={proc.returncode} env={persisted!r} err={proc.stderr!r}",
            )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: the canonical hook-payload parser distinguishes a safe id, an
# absent id that permits legacy fallback, and a present malformed id.
# PREVENTS: payload consumers treating malformed input as absent and minting a
# replacement id that is then mislabeled as the parent session.
def _run_hook_payload_status(results: Results) -> None:
    parser = os.path.join(HOOKS, "lib", "session_id.py")
    for name, payload, expected_status, expected_stdout in (
        (
            "hook-session-id-status-safe",
            {"session_id": _DELEG_SID},
            0,
            _DELEG_SID,
        ),
        ("hook-session-id-status-absent", {}, 1, ""),
        ("hook-session-id-status-malformed-dot", {"session_id": "."}, 2, ""),
        (
            "hook-session-id-status-malformed-trailing-newline",
            {"session_id": _DELEG_SID + "\n"},
            2,
            "",
        ),
    ):
        proc = subprocess.run(
            [sys.executable, parser, "--hook-session-id"],
            input=json.dumps(payload),
            capture_output=True,
            text=True,
            timeout=30,
        )
        results.check(
            name,
            proc.returncode == expected_status
            and proc.stdout.strip() == expected_stdout,
            f"rc={proc.returncode} out={proc.stdout!r} err={proc.stderr!r}",
        )


# VALIDATES: the empty SessionStart matcher routes startup, resume, clear,
# compact, and fork events to session-start.sh.
# PREVENTS: an explicit startup matcher silently excluding four supported
# SessionStart sources from session identity initialization.
def _run_session_start_registration(results: Results) -> None:
    with open(os.path.join(ROOT, ".claude", "settings.json"), encoding="utf-8") as fh:
        cfg = json.load(fh)
    registrations = [
        (group.get("matcher"), entry.get("command", ""))
        for group in cfg.get("hooks", {}).get("SessionStart", [])
        for entry in group.get("hooks", [])
    ]
    results.check(
        "session-start-empty-matcher-registered",
        any(
            matcher == "" and command.endswith("session-start.sh")
            for matcher, command in registrations
        ),
        repr(registrations),
    )


def _subagent_context_output(
    proc: subprocess.CompletedProcess,
) -> tuple[dict | None, str | None]:
    try:
        output = json.loads(proc.stdout)
    except (json.JSONDecodeError, TypeError):
        return None, None
    if not isinstance(output, dict):
        return None, None
    specific = output.get("hookSpecificOutput")
    if not isinstance(specific, dict):
        return None, None
    additional = specific.get("additionalContext")
    return specific, additional if isinstance(additional, str) else None


# VALIDATES: a safe SubagentStart payload identifies the parent session and its
# exact scratch directory in hookSpecificOutput.additionalContext, even when ps
# is unavailable.
# PREVENTS: plain stdout bypassing the SubagentStart context contract, or the
# subagent receiving a minted id and another session's state or scratch path.
def _run_subagent_parent_payload_context(results: Results) -> None:
    parent_sid = _DELEG_SID
    work = _deleg_project(spec="spec-parent-context.md")
    try:
        state_dir = _deleg_state_dir(work)
        parent_scratch = os.path.relpath(
            os.path.join(os.path.dirname(state_dir), "scratch"), work
        )
        env = _fork_payload_env(work)
        proc = subprocess.run(
            ["bash", os.path.join(HOOKS, "subagent-context.sh")],
            input=json.dumps(
                {
                    "hook_event_name": "SubagentStart",
                    "agent_id": "fixture-agent",
                    "session_id": parent_sid,
                }
            ),
            capture_output=True,
            text=True,
            env=env,
            timeout=60,
        )
        specific, additional = _subagent_context_output(proc)
        lines = additional.splitlines() if additional is not None else []
        results.check(
            "subagent-context-emits-json-contract",
            proc.returncode == 0
            and specific is not None
            and specific.get("hookEventName") == "SubagentStart"
            and additional is not None,
            f"rc={proc.returncode} out={proc.stdout!r} err={proc.stderr!r}",
        )
        results.check(
            "subagent-context-names-safe-parent-id",
            f"Parent session ID: {parent_sid}" in lines
            and additional is not None
            and "plan/spec-parent-context.md" in additional,
            f"additionalContext={additional!r}",
        )
        results.check(
            "subagent-context-names-exact-parent-scratch",
            f"Parent session scratch: {parent_scratch}" in lines,
            f"expected={parent_scratch!r} additionalContext={additional!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: a present malformed SubagentStart session_id is rejected as an
# identity source while the hook still returns valid SubagentStart JSON.
# PREVENTS: a dot or trailing newline falling through to process identity or a
# minted UUID that is then labeled as the parent session and scratch directory.
def _run_subagent_malformed_parent_payloads(results: Results) -> None:
    work = _deleg_project(spec=None)
    try:
        env = _fork_payload_env(work)
        for label, sid in (
            ("dot", "."),
            ("trailing-newline", _DELEG_SID + "\n"),
        ):
            proc = subprocess.run(
                ["bash", os.path.join(HOOKS, "subagent-context.sh")],
                input=json.dumps(
                    {
                        "hook_event_name": "SubagentStart",
                        "agent_id": "fixture-agent",
                        "session_id": sid,
                    }
                ),
                capture_output=True,
                text=True,
                env=env,
                timeout=60,
            )
            specific, additional = _subagent_context_output(proc)
            context = additional or ""
            minted_parent = any(
                line.startswith("Parent session ID: ")
                and _UUID_RE.match(line.removeprefix("Parent session ID: ")) is not None
                for line in context.splitlines()
            )
            results.check(
                f"subagent-context-rejects-present-{label}-parent-id",
                proc.returncode == 0
                and specific is not None
                and specific.get("hookEventName") == "SubagentStart"
                and additional is not None
                and "Parent session ID:" not in context
                and "Parent session scratch:" not in context
                and not minted_parent,
                f"rc={proc.returncode} out={proc.stdout!r} err={proc.stderr!r}",
            )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: an absent SubagentStart session_id retains the intentional legacy
# fallback and reports its minted parent id and exact scratch path in JSON.
# PREVENTS: the malformed-id fail-closed rule accidentally removing fallback
# behavior for older callers that omit the field.
def _run_subagent_absent_parent_payload(results: Results) -> None:
    work = _deleg_project(spec=None)
    try:
        proc = subprocess.run(
            ["bash", os.path.join(HOOKS, "subagent-context.sh")],
            input=json.dumps(
                {
                    "hook_event_name": "SubagentStart",
                    "agent_id": "fixture-agent",
                }
            ),
            capture_output=True,
            text=True,
            env=_fork_payload_env(work),
            timeout=60,
        )
        specific, additional = _subagent_context_output(proc)
        lines = additional.splitlines() if additional is not None else []
        parent_prefix = "Parent session ID: "
        parent_ids = [
            line.removeprefix(parent_prefix)
            for line in lines
            if line.startswith(parent_prefix)
        ]
        fallback_sid = parent_ids[0] if len(parent_ids) == 1 else ""
        expected_scratch = (
            "Parent session scratch: "
            f"tmp/session/{datetime.date.today().isoformat()}-{fallback_sid}/scratch"
        )
        results.check(
            "subagent-context-absent-id-keeps-legacy-fallback",
            proc.returncode == 0
            and specific is not None
            and specific.get("hookEventName") == "SubagentStart"
            and additional is not None
            and _UUID_RE.match(fallback_sid) is not None
            and expected_scratch in lines,
            f"rc={proc.returncode} additionalContext={additional!r} "
            f"err={proc.stderr!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def _updated_bash_command(proc: subprocess.CompletedProcess) -> str | None:
    if not proc.stdout.strip():
        return None
    try:
        output = json.loads(proc.stdout)
    except json.JSONDecodeError:
        return None
    specific = output.get("hookSpecificOutput") or {}
    updated = specific.get("updatedInput") or output.get("updatedInput") or {}
    command = updated.get("command")
    return command if isinstance(command, str) else None


# VALIDATES: PreToolUse Bash injects the safe parent payload id into a subagent
# command, and every session-path consumer inherits that one identity.
# PREVENTS: separate scratch, Make, and review subprocesses minting unrelated
# fallback ids when the restricted fork cannot inspect ps.
def _run_pretool_parent_payload_export(results: Results) -> None:
    parent_sid = _DELEG_SID
    work = tempfile.mkdtemp(prefix="ze-pretool-parent-", dir=_fixture_root())
    try:
        review_probe = (
            "import sys;"
            f"sys.path.insert(0,{DEV!r});"
            "import review_gate;"
            "print(review_gate.session_id())"
        )
        original = "; ".join(
            [
                "printf 'scratch1=%s\\n' \"$(scripts/dev/session-scratch.sh --path)\"",
                "printf 'scratch2=%s\\n' \"$(scripts/dev/session-scratch.sh --path)\"",
                "printf 'scratch3=%s\\n' \"$(scripts/dev/session-scratch.sh --path)\"",
                "printf 'make=%s\\n' \"$(make -s ze-session-binary-path)\"",
                "printf 'review=%s\\n' "
                f'"$({shlex.quote(sys.executable)} -c {shlex.quote(review_probe)})"',
            ]
        )
        env = _fork_payload_env(work, ROOT)

        def pretool(sid: str) -> subprocess.CompletedProcess:
            return subprocess.run(
                [sys.executable, os.path.join(HOOKS, "pretool-bash.py")],
                input=json.dumps(
                    {
                        "hook_event_name": "PreToolUse",
                        "tool_name": "Bash",
                        "agent_id": "fixture-agent",
                        "session_id": sid,
                        "tool_input": {"command": original},
                    }
                ),
                capture_output=True,
                text=True,
                env=env,
                timeout=60,
            )

        safe = pretool(parent_sid)
        updated = _updated_bash_command(safe)
        results.check(
            "pretool-bash-prefixes-safe-parent-export",
            safe.returncode == 0
            and updated == f"export CLAUDE_CODE_SESSION_ID={parent_sid}; {original}",
            f"rc={safe.returncode} updated={updated!r} err={safe.stderr!r}",
        )

        for label, bad in (
            ("trailing-newline", parent_sid + "\n"),
            ("encoded-nul", parent_sid + "\0suffix"),
            ("dot", "."),
            ("dot-dot", ".."),
        ):
            malformed = pretool(bad)
            malformed_update = _updated_bash_command(malformed)
            results.check(
                f"pretool-bash-leaves-{label}-id-unchanged",
                malformed.returncode == 0 and malformed_update in (None, original),
                f"rc={malformed.returncode} updated={malformed_update!r} "
                f"err={malformed.stderr!r}",
            )

        executed = None
        if updated:
            executed = subprocess.run(
                ["bash", "-c", updated],
                capture_output=True,
                text=True,
                cwd=ROOT,
                env=env,
                timeout=60,
            )
        values = {}
        if executed is not None:
            values = dict(
                line.split("=", 1)
                for line in executed.stdout.splitlines()
                if "=" in line
            )
        scratches = [values.get(f"scratch{i}", "") for i in range(1, 4)]
        scratch = scratches[0]
        session_root = scratch.removesuffix("/scratch")
        results.check(
            "pretool-bash-parent-export-keeps-three-scratch-calls-stable",
            executed is not None
            and executed.returncode == 0
            and all(path == scratch for path in scratches)
            and re.fullmatch(
                rf"tmp/session/\d{{4}}-\d{{2}}-\d{{2}}-{re.escape(parent_sid)}/scratch",
                scratch,
            )
            is not None,
            f"values={values!r} err={executed.stderr if executed else ''!r}",
        )
        results.check(
            "pretool-bash-parent-export-reaches-make",
            values.get("make") == f"{session_root}/bin/ze",
            f"values={values!r}",
        )
        results.check(
            "pretool-bash-parent-export-reaches-review",
            values.get("review") == parent_sid,
            f"values={values!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: a fork SessionStart payload persists its safe parent session id, so
# later Bash, Make, Python review, and scratch subprocesses use that parent.
# PREVENTS: CLAUDE_CODE_FORK_SUBAGENT=1 losing the exported parent id while
# ZE_SESSION_ID is absent, then minting a fallback for later commands.
def _run_fork_parent_session_id(results: Results) -> None:
    parent_sid = _DELEG_SID
    spec_name = "spec-fork-parent.md"
    work = _deleg_project(spec=spec_name)
    try:
        env_file = os.path.join(work, "persistent-env")
        with open(env_file, "w"):
            pass

        # Deny the resolver's `ps` fallback. Linux can still read /proc, but the
        # process tree carries no --session-id, so ancestry has no parent id on
        # either supported platform. The payload is the only identity source.
        deny_bin = os.path.join(work, "deny-bin")
        os.makedirs(deny_bin)
        ps = os.path.join(deny_bin, "ps")
        with open(ps, "w") as fh:
            fh.write("#!/bin/sh\nexit 126\n")
        os.chmod(ps, 0o755)

        env = dict(os.environ)
        for name in (
            "CLAUDE_CODE_SESSION_ID",
            "CLAUDE_CODE_SESSION_ACCESS_TOKEN",
            "ZE_SESSION_ID",
        ):
            env.pop(name, None)
        env.update(
            {
                "CLAUDE_CODE_FORK_SUBAGENT": "1",
                "CLAUDE_ENV_FILE": env_file,
                "CLAUDE_PROJECT_DIR": work,
                "PATH": deny_bin + os.pathsep + env["PATH"],
            }
        )
        payload = json.dumps(
            {
                "hook_event_name": "SessionStart",
                "session_id": parent_sid,
                "source": "startup",
            }
        )
        start = subprocess.run(
            ["bash", os.path.join(HOOKS, "session-start.sh")],
            input=payload,
            text=True,
            capture_output=True,
            env=env,
            timeout=60,
        )
        results.check(
            "session-id-fork-parent-marker",
            start.returncode == 0 and f"SPEC: {spec_name}" in start.stdout,
            f"rc={start.returncode} stdout={start.stdout!r} stderr={start.stderr!r}",
        )

        def later(*command: str, cwd: str = ROOT) -> subprocess.CompletedProcess:
            # Model a later command invocation. Claude starts a fresh subprocess
            # after it sources CLAUDE_ENV_FILE, rather than mutating this runner.
            return subprocess.run(
                [
                    "bash",
                    "-c",
                    '. "$CLAUDE_ENV_FILE"; exec "$@"',
                    "fork-session-fixture",
                    *command,
                ],
                cwd=cwd,
                capture_output=True,
                text=True,
                env=env,
                timeout=60,
            )

        env_probe = later(
            sys.executable,
            "-c",
            (
                "import os;"
                "print(os.environ.get('CLAUDE_CODE_SESSION_ID', ''));"
                "print('set' if 'ZE_SESSION_ID' in os.environ else 'unset')"
            ),
        )
        results.check(
            "session-id-fork-parent-persisted-env",
            env_probe.returncode == 0
            and env_probe.stdout.splitlines() == [parent_sid, "unset"],
            f"rc={env_probe.returncode} out={env_probe.stdout!r} err={env_probe.stderr!r}",
        )

        bash_sid = later(
            "bash",
            "-c",
            "source .claude/hooks/lib/session-id.sh; _session_id",
        )
        results.check(
            "session-id-fork-parent-bash-command",
            bash_sid.returncode == 0 and bash_sid.stdout.strip() == parent_sid,
            f"rc={bash_sid.returncode} out={bash_sid.stdout!r} err={bash_sid.stderr!r}",
        )

        make_path = later("make", "ze-session-binary-path")
        make_value = make_path.stdout.strip()
        make_session = make_value.removesuffix("/bin/ze")
        results.check(
            "session-id-fork-parent-make-path",
            make_path.returncode == 0
            and make_value == f"{make_session}/bin/ze"
            and re.fullmatch(
                rf"tmp/session/\d{{4}}-\d{{2}}-\d{{2}}-{re.escape(parent_sid)}",
                make_session,
            )
            is not None,
            f"rc={make_path.returncode} out={make_path.stdout!r} err={make_path.stderr!r}",
        )

        review_path = later(
            sys.executable,
            "-c",
            (
                "import sys;"
                "sys.path.insert(0, sys.argv[1]);"
                "import review_gate;"
                "print(review_gate.artifact_path('fork-parent'))"
            ),
            DEV,
        )
        results.check(
            "session-id-fork-parent-review-artifact",
            review_path.returncode == 0
            and review_path.stdout.strip() == f"tmp/review/fork-parent-{parent_sid}.md",
            f"rc={review_path.returncode} out={review_path.stdout!r} "
            f"err={review_path.stderr!r}",
        )

        scratch_one = later("scripts/dev/session-scratch.sh", "--path")
        scratch_two = later("scripts/dev/session-scratch.sh", "--path")
        expected_scratch = f"{make_session}/scratch"
        results.check(
            "session-id-fork-parent-scratch-stable",
            scratch_one.returncode == scratch_two.returncode == 0
            and scratch_one.stdout.strip()
            == scratch_two.stdout.strip()
            == expected_scratch,
            f"first={scratch_one.stdout!r}/{scratch_one.stderr!r} "
            f"second={scratch_two.stdout!r}/{scratch_two.stderr!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: fork tool subprocesses with no session env use the canonical
# session_id.py id in Make, review_gate, and repeated scratch paths without a
# persistent environment file.
# PREVENTS: Make falling back to bin/ze and review_gate falling back to shared
# while session-scratch.sh resolves the fork's safe parent session id.
def _run_fork_tool_session_id(results: Results) -> None:
    work = tempfile.mkdtemp(prefix="ze-sid-fork-tool-", dir=_fixture_root())
    try:
        fork_env = dict(os.environ)
        for name in (
            "CLAUDE_CODE_SESSION_ID",
            "CLAUDE_CODE_SESSION_ACCESS_TOKEN",
            "ZE_SESSION_ID",
            "CLAUDE_ENV_FILE",
        ):
            fork_env.pop(name, None)
        fork_env.update(
            {
                "CLAUDE_CODE_FORK_SUBAGENT": "1",
                "CLAUDE_PROJECT_DIR": work,
            }
        )

        def run(
            *command: str, env: dict = fork_env, cwd: str = ROOT
        ) -> subprocess.CompletedProcess:
            return subprocess.run(
                command,
                cwd=cwd,
                capture_output=True,
                text=True,
                env=env,
                timeout=60,
            )

        canonical = run(sys.executable, os.path.join(HOOKS, "lib", "session_id.py"))
        canonical_sid = canonical.stdout.strip()
        canonical_safe = re.fullmatch(
            r"[A-Za-z0-9._-]+", canonical_sid
        ) is not None and canonical_sid not in (".", "..", "shared")
        results.check(
            "session-id-fork-tool-canonical",
            canonical.returncode == 0 and canonical_safe,
            f"rc={canonical.returncode} out={canonical.stdout!r} "
            f"err={canonical.stderr!r}",
        )

        make_path = run("make", "ze-session-binary-path")
        make_expected = re.fullmatch(
            rf"tmp/session/\d{{4}}-\d{{2}}-\d{{2}}-"
            rf"{re.escape(canonical_sid)}/bin/ze",
            make_path.stdout.strip(),
        )
        results.check(
            "session-id-fork-tool-make-path",
            make_path.returncode == 0 and canonical_safe and make_expected is not None,
            f"canonical={canonical_sid!r} rc={make_path.returncode} "
            f"out={make_path.stdout!r} err={make_path.stderr!r}",
        )

        review_sid = run(
            sys.executable,
            "-c",
            (
                "import sys;"
                "sys.path.insert(0, sys.argv[1]);"
                "import review_gate;"
                "print(review_gate.session_id())"
            ),
            DEV,
        )
        results.check(
            "session-id-fork-tool-review-session",
            review_sid.returncode == 0
            and canonical_safe
            and review_sid.stdout.strip() == canonical_sid,
            f"canonical={canonical_sid!r} rc={review_sid.returncode} "
            f"out={review_sid.stdout!r} err={review_sid.stderr!r}",
        )

        scratch_one = run(os.path.join(DEV, "session-scratch.sh"), "--path")
        scratch_two = run(os.path.join(DEV, "session-scratch.sh"), "--path")
        scratch_value = scratch_one.stdout.strip()
        scratch_expected = re.fullmatch(
            rf"tmp/session/\d{{4}}-\d{{2}}-\d{{2}}-"
            rf"{re.escape(canonical_sid)}/scratch",
            scratch_value,
        )
        results.check(
            "session-id-fork-tool-scratch-stable",
            scratch_one.returncode == scratch_two.returncode == 0
            and canonical_safe
            and scratch_one.stdout == scratch_two.stdout
            and scratch_expected is not None,
            f"canonical={canonical_sid!r} "
            f"first={scratch_one.stdout!r}/{scratch_one.stderr!r} "
            f"second={scratch_two.stdout!r}/{scratch_two.stderr!r}",
        )

        human_env = dict(fork_env)
        human_env.pop("CLAUDE_CODE_FORK_SUBAGENT")
        human_make = run("make", "ze-session-binary-path", env=human_env)
        results.check(
            "session-id-human-make-path",
            human_make.returncode == 0 and human_make.stdout.strip() == "bin/ze",
            f"rc={human_make.returncode} out={human_make.stdout!r} "
            f"err={human_make.stderr!r}",
        )

        human_review = run(
            sys.executable,
            "-c",
            (
                "import sys;"
                "sys.path.insert(0, sys.argv[1]);"
                "import review_gate;"
                "print(review_gate.session_id())"
            ),
            DEV,
            env=human_env,
        )
        results.check(
            "session-id-human-review-fallback",
            human_review.returncode == 0 and human_review.stdout.strip() == "shared",
            f"rc={human_review.returncode} out={human_review.stdout!r} "
            f"err={human_review.stderr!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_session_id(results: Results) -> None:
    """Lock the shell writer and the Python reader to ONE session id.

    These two ends key the same marker files (.lsp-invoked-<sid>,
    .source-read-<sid>, .session-<sid>) and the same session directory
    (<YYYY-MM-DD>-<sid>/state/session-state-<stem>-<sid>.md): the shell
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
    # UUID cached by the CLI-ancestor PID and its start time, stable across hook
    # subprocesses. Both ends
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
    _run_session_id_cache_key(results)
    _run_session_start_raw_payload_ids(results)
    _run_hook_payload_status(results)
    _run_session_start_registration(results)
    _run_subagent_parent_payload_context(results)
    _run_subagent_malformed_parent_payloads(results)
    _run_subagent_absent_parent_payload(results)
    _run_pretool_parent_payload_export(results)
    _run_fork_parent_session_id(results)
    _run_fork_tool_session_id(results)


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
    # The weakening hatch reads `test/weakened.md` under PROJECT_DIR. Point it at a
    # directory that holds none, so every case here judges an EMPTY hatch: a real
    # row naming a test called `TestX` would otherwise open it and flip a fixture
    # that has nothing to do with the row. `run_weakened_hatch` owns the file.
    mod.PROJECT_DIR = tempfile.mkdtemp(prefix="ze-no-weakened-", dir=_fixture_root())
    try:
        _run_rfc_test_guard(results, mod)
    finally:
        shutil.rmtree(mod.PROJECT_DIR, ignore_errors=True)


def _run_rfc_test_guard(results: Results, mod) -> None:
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

    # The user's approval used to be a marker in the replacement text. It is a
    # comment, so it outlived the diff it explained: 255 of them across 120 test
    # files. The approval is a row in `test/rfc-changed.md` now, and this guard
    # reads no marker, so the same edit is refused.
    approved = tagged.replace(
        "func TestX",
        "// rfc-test-change-approved: 2026-07-17 user agreed 3(j) mandates reset\nfunc TestX",
    ).replace("TreatAsWithdraw", "SessionReset")
    r = edit(tagged, approved)
    results.check(
        "rfc-guard-marker-no-longer-approves", r is not None and r[0] == 2, repr(r)
    )

    # An edit made ONLY of Go import lines passes. New tests need new imports, the import
    # block sits outside every function so the scope widens to the whole file, and the
    # guard used to charge an operator approval for GROWING a tagged file -- which is the
    # route ai/rules/testing.md prescribes (HOOK-FRICTION.md, 2026-08-01).
    # These cases need the WIDENED scope, which is the whole file when the edited lines
    # sit outside every function -- exactly where an import block lives. `edit` above
    # cannot produce that: its `fp` does not exist, so _enclosing_tagged_scope gets an
    # OSError and the scope falls back to the old side. Call the inner function with the
    # scope the real hook would have read off disk.
    def scoped(old: str, new: str, path: str = fp, scope: str = tagged):
        return mod._rfc_tagged_change_err(old, new, path, tag_scope=scope)

    go = "/repo/internal/component/bgp/plugins/rib/rfc4271_test.go"
    imports_old = '\t"testing"\n'
    imports_new = '\t"testing"\n\t"net/netip"\n'

    r = scoped(imports_old, imports_new, go)
    results.check("rfc-guard-allows-import-only-add", r is None, repr(r))

    # Aliased and blank imports are still imports.
    r = scoped(imports_old, '\tbgpctx "a/b/c"\n\t_ "d/e"\n', go)
    results.check("rfc-guard-allows-aliased-import", r is None, repr(r))

    # Growing a single import into a parenthesised block passes too.
    r = scoped('import "testing"\n', 'import (\n\t"testing"\n\t"os"\n)\n', go)
    results.check("rfc-guard-allows-import-block-growth", r is None, repr(r))

    # The exemption is NOT a hole: one non-import line in the same edit blocks it.
    r = scoped(imports_old, imports_new + "\trequire.NotEqual(t, want, got)\n", go)
    results.check("rfc-guard-import-plus-assertion-blocks", r is not None, repr(r))

    # Deleting a TEST is not import-only, because a function body is not import-shaped.
    r = scoped(tagged, "", go)
    results.check(
        "rfc-guard-import-exemption-spares-no-test-delete", r is not None, repr(r)
    )

    # Go-only: a .ci carrier has no imports, so nothing about it changes.
    r = scoped(imports_old, imports_new, "/repo/test/plugin/x.ci")
    results.check("rfc-guard-import-exemption-is-go-only", r is not None, repr(r))

    # A tag removed in the same breath as an import edit still blocks: the tag check runs
    # first, precisely so a comment deletion cannot ride in on the exemption.
    r = scoped(
        "// RFC requirement: RFC7606-7.1-1 negative - x.\n" + imports_old,
        imports_new,
        go,
    )
    results.check("rfc-guard-import-edit-cannot-drop-a-tag", r is not None, repr(r))

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

    # AC-4: the retired approval token is refused when the tag is out of hunk too.
    # Nothing about the marker satisfies this guard any more, whatever the scope.
    r = edit(
        body,
        "\t// rfc-test-change-approved: 2026-07-20 user confirmed\n"
        + body.replace("TreatAsWithdraw", "None"),
    )
    results.check("rfc-guard-body-edit-marker-blocks", blocked_on_rfc(r), repr(r))

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

    # A `.ci` drops an `expect=`, which `_test_weakening_errs` reports without refusing.
    os.makedirs(os.path.join(tmp, "test", "ui"), exist_ok=True)
    relax_ci = os.path.join(tmp, "test", "ui", "relax-form.ci")
    ci_old = "cmd=ze show\nexpect=out:text=one\nexpect=out:text=two\n"
    ci_new = "cmd=ze show\nexpect=out:text=one\n"
    with open(relax_ci, "w", encoding="utf-8") as fh:
        fh.write(ci_old)

    # A count falling REPORTS and lets the edit through (`soft` in
    # `_test_weakening_errs`): a count cannot tell a deleted expectation from two
    # consolidated into one, and refusing on it is what built 780 `test-relax:`
    # tokens. What this fixture still pins is that the arm SEES the drop, which
    # is why it asserts the notice names it rather than only asserting a 0.
    r = edit(ci_old, ci_new, relax_ci)
    results.check(
        "weakening-ci-shrink-reported-not-refused",
        r is not None and r[0] == 0 and "removing expectations" in r[1],
        repr(r),
    )

    # ---- what the guard is allowed to be WRONG about (2026-08-10) ----------------
    #
    # The `.ci` arm counted non-comment LINES until this date. That made it fire on
    # every mechanical improvement and stay silent on the damage it exists to catch,
    # and 368 of the 755 `test-relax:` tokens in the tree were written to get past
    # the false positive. A guard nobody believes is a guard nobody reads.

    # A blind sleep collapsed into a real barrier removes lines and adds coverage.
    # Under the line counter this was refused; refusing it is what taught agents to
    # write a token reflexively.
    r = edit(
        "import time\ntime.sleep(2.0)\nresp = dispatch(api, 'show bgp summary')\n",
        "api.wait_until(lambda: 'established' in dispatch(api, 'show bgp summary'))\n",
        relax_ci,
    )
    results.check("relax-ci-sleep-to-barrier-allowed", r is None, repr(r))

    # ...and dropping the barrier itself is still a weakening, so the new counter is
    # not simply more permissive.
    r = edit(
        "api.wait_until(lambda: ok())\nexpect=out:text=one\n",
        "expect=out:text=one\n",
        relax_ci,
    )
    results.check(
        "relax-ci-barrier-removal-reported",
        r is not None and r[0] == 0 and "removing expectations" in r[1],
        repr(r),
    )

    # The case the line counter could never see: same shape, same counts, no verdict
    # left. This is the in-place gutting `_TAUTOLOGY` was added for.
    r = edit(
        "assert 'established' in resp\nassert route in best\n",
        "assert True\nassert True\n",
        relax_ci,
    )
    results.check(
        "relax-ci-tautology-swap-blocked", r is not None and r[0] == 2, repr(r)
    )

    taut_go = os.path.join(tmp, "internal", "x", "taut_test.go")
    os.makedirs(os.path.dirname(taut_go), exist_ok=True)
    r = edit("\trequire.Equal(t, 179, port)\n", "\trequire.True(t, true)\n", taut_go)
    results.check(
        "relax-go-tautology-swap-blocked", r is not None and r[0] == 2, repr(r)
    )

    # ---- the .et carrier (2026-08-10) -------------------------------------------
    #
    # `is_test` named `_test.go` and `.ci` only, so `c_test_weakening` returned None
    # for all 164 editor tests and the guard was inert over that whole suite. None
    # carries an `RFC requirement:` tag, so `_carries_rfc_tag` was not admitting them
    # by the side door either.
    os.makedirs(os.path.join(tmp, "test", "editor", "commands"), exist_ok=True)
    et = os.path.join(tmp, "test", "editor", "commands", "hist.et")
    r = edit(
        "expect=input:value=show\nexpect=input:value=set\n",
        "expect=input:value=set\n",
        et,
    )
    results.check(
        "relax-et-expectation-removal-reported",
        r is not None and r[0] == 0 and "removing expectations" in r[1],
        repr(r),
    )

    # ---- what the .ci counter must not be fooled by (2026-08-10) ----------------
    #
    # A bare `\bassert\b` matched prose and string literals, so two words of comment
    # paid for a deleted expectation. Measured at HEAD on 2026-08-10, it matched 49
    # comment lines across 47 tracked `.ci` files, every one able to pay for a
    # deletion.
    for label, replacement in (
        ("comment", "expect=out:text=two\n# we no longer assert the first line\n"),
        ("string-literal", "expect=out:text=two\nmsg = 'assert this'\n"),
    ):
        r = edit("expect=out:text=one\nexpect=out:text=two\n", replacement, relax_ci)
        results.check(
            f"relax-ci-{label}-does-not-offset-a-deleted-expectation",
            r is not None and r[0] == 0 and "removing expectations (2 -> 1" in r[1],
            repr(r),
        )

    # `cmd=` was counted by the line counter this replaced. A deleted `cmd=` stops a
    # command running, which is coverage removed even when the expectations still match.
    r = edit(
        "cmd=ze show bgp\ncmd=ze show route\nexpect=out:text=one\n",
        "cmd=ze show bgp\nexpect=out:text=one\n",
        relax_ci,
    )
    results.check(
        "relax-ci-cmd-removal-reported",
        r is not None and r[0] == 0 and "removing expectations" in r[1],
        repr(r),
    )

    # Inverting a negative expectation keeps the combined count identical: the run now
    # DEMANDS the error it used to refuse. Only a separate reject= tally sees it.
    r = edit("reject=out:text=error\n", "expect=out:text=error\n", relax_ci)
    results.check(
        "relax-ci-reject-to-expect-reported",
        r is not None and r[0] == 0 and "removing negative expectations" in r[1],
        repr(r),
    )

    # An emptied needle stops being checked at all (`validateFileContent` guards on
    # `check.Contains != ""`), and the line is still there for any counter to find.
    r = edit("expect=out:text=Established\n", "expect=out:text=\n", relax_ci)
    results.check(
        "relax-ci-emptied-needle-blocked", r is not None and r[0] == 2, repr(r)
    )

    # ...but a needle may legitimately BEGIN with `#`, and 9 tracked lines are that
    # form. Judging it on comment-stripped text
    # misfired in BOTH directions: adding such a line was refused as an emptied
    # needle, and genuinely emptying one was allowed, because both sides looked
    # equally empty once the `#` and everything after it was cut.
    # The gate must not refuse its own cleanup. `_ASSERT_PAT` matches `require.` in
    # PROSE, so a comment that mentions an assertion counted as one, and deleting
    # that comment read as deleting the assertion. The `test-relax:` corpus this
    # gate produced was full of such prose, so the sweep that drained it would have
    # needed a fresh justification for every one it removed.
    r = edit(
        "// dropped the require.Equal on port when the flag went\n"
        "func TestA(t *testing.T) { require.NoError(t, err) }\n",
        "func TestA(t *testing.T) { require.NoError(t, err) }\n",
        taut_go,
    )
    results.check(
        "weakening-removing-a-comment-whose-prose-names-an-assertion",
        r is None,
        repr(r),
    )

    # ...while prose must not PAY for one either, in the other direction.
    # Assert the MESSAGE, not just the exit code: this shape also trips
    # `commenting out assertions`, so a code-only assertion passes even when the
    # count arm it names is reverted.
    r = edit(
        "\trequire.Equal(t, 1, f())\n\trequire.NoError(t, err)\n",
        "\t// we no longer require.Equal here\n\trequire.NoError(t, err)\n",
        taut_go,
    )
    results.check(
        "relax-prose-does-not-offset-a-deleted-go-assertion",
        r is not None and r[0] == 2 and "removing assertions (2 -> 1)" in r[1],
        repr(r),
    )

    # The comment strip is QUOTE-AWARE, and both halves of that matter. Neither is
    # covered by the two fixtures above, which use whole-line comments.
    #
    # `//` inside a Go string literal is not a comment. Stripping to end-of-line ate
    # the `t.Fatal` from the OLD side alone, so deleting the whole entry looked like
    # no change at all.
    r = edit(
        '\twant: "//go:build linux\\n" + "t.Fatal(x)",\n\trequire.NoError(t, err)\n',
        "\trequire.NoError(t, err)\n",
        taut_go,
    )
    results.check(
        "relax-go-comment-inside-a-string-literal-is-not-stripped",
        r is not None and r[0] == 0 and "removing assertions" in r[1],
        repr(r),
    )

    # ...and a TRAILING comment must not PAY for deleted coverage. Stripping only
    # whole-line comments left this open on every arm that is not statement-anchored.
    r = edit(
        "expect=out:text=one\nexpect=out:text=two\n",
        "expect=out:text=two  # dropped; we now wait_until(x)\n",
        relax_ci,
    )
    results.check(
        "relax-ci-trailing-comment-does-not-pay-for-a-deleted-expectation",
        r is not None and r[0] == 0 and "removing expectations" in r[1],
        repr(r),
    )
    r = edit(
        '\tt.Run("a", f)\n\tt.Run("b", g)\n',
        '\tt.Run("a", f) // dropped the t.Run( for b\n',
        taut_go,
    )
    results.check(
        "relax-go-trailing-comment-does-not-pay-for-a-deleted-subtest",
        r is not None and r[0] == 0 and "removing t.Run cases" in r[1],
        repr(r),
    )

    hashed_needle = "expect=stdout:contains=# tcp.bind\n"
    r = edit("cmd=ze x\n", "cmd=ze x\n" + hashed_needle, relax_ci)
    results.check("relax-ci-hash-leading-needle-allowed", r is None, repr(r))
    r = edit(hashed_needle, "expect=stdout:contains=\n", relax_ci)
    results.check(
        "relax-ci-emptying-a-hash-leading-needle-blocked",
        r is not None and r[0] == 2,
        repr(r),
    )

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
# The weakening hatch: test/weakened.md (plan/spec-weakened-per-commit.md)
# --------------------------------------------------------------------------- #


def run_weakened_hatch(results: Results) -> None:
    """The hatch opens on a row in `test/weakened.md` naming the test THIS edit weakens.

    The justification used to be a `test-relax:` comment written in the same edit.
    It stayed in the test file forever, explaining a diff its later readers could
    not see, and 601 of them piled up across 413 files. The record now lives in one
    file the commit carries and replaces.

    The hook reads that file from disk, so the row is written BEFORE the edit. That
    ordering is a real workflow change, and the first two cases below are the same
    edit against two states of the file: only the row changes the verdict.

    PROJECT_DIR is redirected at a fixture tree, which is where the hook looks for
    the file (`_weakened_hatch`).
    """
    print("weakened-hatch:")
    mod = _load_pretool_writeedit()
    work = tempfile.mkdtemp(prefix="ze-weakened-", dir=_fixture_root())
    mod.PROJECT_DIR = work
    try:
        _weakened_hatch_cases(results, mod, work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def _weakened_hatch_cases(results: Results, mod, work: str) -> None:
    cw = mod.c_test_weakening
    os.makedirs(os.path.join(work, "test"), exist_ok=True)
    pkg = os.path.join(work, "internal", "component", "rib")
    os.makedirs(pkg, exist_ok=True)

    fp = os.path.join(pkg, "rib_test.go")
    body = "\trequire.Equal(t, 1, f())\n"
    other = "\trequire.NoError(t, err)\n"
    source = (
        "package rib\n"
        "\n"
        "func TestRibHolds(t *testing.T) {\n" + body + "}\n"
        "\n"
        "func TestRibDrops(t *testing.T) {\n" + other + "}\n"
    )
    with open(fp, "w", encoding="utf-8") as fh:
        fh.write(source)

    def contract(*rows: str) -> None:
        with open(
            os.path.join(work, "test", "weakened.md"), "w", encoding="utf-8"
        ) as fh:
            fh.write(
                "# Tests this commit weakens\n"
                "\n"
                "| Test | Reason |\n"
                "|------|--------|\n" + "".join(rows)
            )

    def edit(old: str, new: str, path: str = fp):
        return cw(
            {"tool": "Edit", "ti": {"old_string": old, "new_string": new}, "fp": path}
        )

    skip = '\tt.Skip("flaky")\n'
    accepted = "| TestRibHolds | f() went with its only caller |\n"

    # The hunk is the assertion alone: it carries no `func` line, so the name of the
    # weakened test can only come from the file. The commit gate compares whole
    # files and asks for that name, so a hook that named the FILE here would demand
    # a different row from the one the commit needs.
    contract()
    r = edit(body, skip)
    results.check(
        "weakened-missing-row-refuses-the-edit",
        r is not None
        and r[0] == 2
        and "| TestRibHolds |" in r[1]
        and "test/weakened.md" in r[1]
        and "WRITE THE ROW FIRST" in r[1],
        repr(r),
    )

    # The same edit, with the row written first.
    contract(accepted)
    r = edit(body, skip)
    results.check("weakened-row-opens-the-hatch", r is None, repr(r))

    # Per weakening, never per file: a row for the test next door accepts nothing,
    # or one row would buy every later weakening in the file it sits beside.
    contract("| TestRibDrops | a different test entirely |\n")
    r = edit(body, skip)
    results.check(
        "weakened-row-for-another-test-buys-nothing",
        r is not None and r[0] == 2 and "| TestRibHolds |" in r[1],
        repr(r),
    )

    # `package.TestName` is the spelling `test/weakened.md` publishes for a name two
    # packages share, and the commit gate pairs on it. The hook must accept the same
    # row, or an author writing the qualified form is refused at edit time and
    # accepted at commit time.
    contract("| rib.TestRibHolds | f() went with its only caller |\n")
    r = edit(body, skip)
    results.check("weakened-qualified-row-opens-the-hatch", r is None, repr(r))

    # ...and the qualifier is checked rather than skipped.
    contract("| web.TestRibHolds | the same name in another package |\n")
    r = edit(body, skip)
    results.check(
        "weakened-wrong-package-qualifier-buys-nothing",
        r is not None and r[0] == 2,
        repr(r),
    )

    # A Write overwrite is judged the same way. The old hatch read the whole
    # replacement text, so one justification written months earlier sat in the file
    # and exempted every later overwrite of it: 468 test files, about a tenth of the
    # suite, were unguarded that way.
    gutted = source.replace(body, skip)
    contract()
    r = cw({"tool": "Write", "ti": {"content": gutted}, "fp": fp})
    results.check(
        "weakened-write-overwrite-needs-a-row",
        r is not None and r[0] == 2 and "| TestRibHolds |" in r[1],
        repr(r),
    )
    contract(accepted)
    r = cw({"tool": "Write", "ti": {"content": gutted}, "fp": fp})
    results.check("weakened-write-overwrite-row-opens-the-hatch", r is None, repr(r))

    # A row is SELF-SERVICE, so it must not reach an RFC-tagged test: that gate asks
    # for the USER's approval, and the two must not be confusable.
    tagged = os.path.join(pkg, "rfc7606_test.go")
    tag = "// RFC requirement: RFC7606-7.1-1 negative - ORIGIN len != 1 withdraws.\n"
    tagged_body = "\trequire.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)\n"
    with open(tagged, "w", encoding="utf-8") as fh:
        fh.write(
            "package rib\n\n"
            + tag
            + "func TestTagged(t *testing.T) {\n"
            + tagged_body
            + "}\n"
        )
    contract("| TestTagged | the withdraw path moved to the codec |\n")
    r = edit(tagged_body, skip, tagged)
    results.check(
        "weakened-row-does-not-authorize-an-rfc-tagged-test",
        r is not None and r[0] == 2 and "RFC-tagged test" in r[1],
        repr(r),
    )

    # A count falling still reports at code 0 and still asks for no row: a count
    # cannot tell a deleted check from three consolidated into one, and refusing on
    # it is what produced the corpus this file replaces. The notice names the commit
    # gate, which does record every kind.
    counts = os.path.join(pkg, "counts_test.go")
    with open(counts, "w", encoding="utf-8") as fh:
        fh.write(
            "package rib\n\nfunc TestCounts(t *testing.T) {\n" + body + other + "}\n"
        )
    contract()
    r = edit(body + other, other, counts)
    results.check(
        "weakened-count-drop-still-only-notices",
        r is not None and r[0] == 0 and "removing assertions" in r[1],
        repr(r),
    )


# --------------------------------------------------------------------------- #
# The RFC approval ledger: test/rfc-changed.md
# --------------------------------------------------------------------------- #


def run_rfc_changed_ledger(results: Results) -> None:
    """The owner's approval of a tagged-test change is a row in `test/rfc-changed.md`.

    It used to be an `// rfc-test-change-approved:` comment written into the test
    itself, which stayed there after the diff it explained was gone: 255 of them
    across 120 test files. The hook reads the per-commit ledger now, and reads no
    marker at all, so these cases hold BOTH halves -- a row opens the gate, and a
    marker no longer does.

    The row is written BEFORE the edit, because the hook reads the file from
    disk. That ordering is a real workflow change, and the first three cases are
    the same edit against three states of the file: only the ledger moves.

    PROJECT_DIR is redirected at a fixture tree, which is where the hook looks
    for the file (`_rfc_changed_hatch`).
    """
    print("rfc-changed-ledger:")
    mod = _load_pretool_writeedit()
    work = tempfile.mkdtemp(prefix="ze-rfc-changed-", dir=_fixture_root())
    mod.PROJECT_DIR = work
    try:
        _rfc_changed_ledger_cases(results, mod, work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def _rfc_changed_ledger_cases(results: Results, mod, work: str) -> None:
    cw = mod.c_test_weakening
    os.makedirs(os.path.join(work, "test"), exist_ok=True)
    pkg = os.path.join(work, "internal", "component", "message")
    os.makedirs(pkg, exist_ok=True)

    fp = os.path.join(pkg, "rfc7606_test.go")
    tag = "// RFC requirement: RFC7606-7.1-1 negative - ORIGIN len != 1 withdraws.\n"
    body = "\trequire.Equal(t, RFC7606ActionTreatAsWithdraw, result.Action)\n"
    other = "\trequire.NoError(t, err)\n"
    source = (
        "package message\n"
        "\n" + tag + "func TestTagged(t *testing.T) {\n" + body + "}\n"
        "\n"
        "func TestPlain(t *testing.T) {\n" + other + "}\n"
    )
    with open(fp, "w", encoding="utf-8") as fh:
        fh.write(source)

    ledger_path = os.path.join(work, "test", "rfc-changed.md")

    def ledger(*rows: str) -> None:
        with open(ledger_path, "w", encoding="utf-8") as fh:
            fh.write(
                "# RFC-tagged tests this commit changes\n"
                "\n"
                "| Test | Reason |\n"
                "|------|--------|\n" + "".join(rows)
            )

    def edit(old: str, new: str, path: str = fp):
        return cw(
            {"tool": "Edit", "ti": {"old_string": old, "new_string": new}, "fp": path}
        )

    def blocked_on_rfc(r) -> bool:
        """Exit 2 alone is not proof: the weakening path returns 2 as well."""
        return r is not None and r[0] == 2 and "RFC-tagged test" in r[1]

    swap = body.replace("TreatAsWithdraw", "SessionReset")
    approved = (
        "| TestTagged | Thomas ruled on 2026-08-19 that the reset is the "
        "conformant action; RFC7606-7.1-1 is proven by the same assertion |\n"
    )

    # No ledger on disk at all. An unreadable ledger is not an open one, so the
    # edit is refused and the message names the file to write.
    r = edit(body, swap)
    results.check(
        "rfc-changed-absent-ledger-refuses",
        blocked_on_rfc(r) and "test/rfc-changed.md" in r[1],
        repr(r),
    )

    # The ledger exists and names nothing: the same refusal, now carrying the row
    # the author has to get approved.
    ledger()
    r = edit(body, swap)
    results.check(
        "rfc-changed-missing-row-refuses-the-edit",
        blocked_on_rfc(r) and "| TestTagged |" in r[1] and "RFC7606-7.1-1" in r[1],
        repr(r),
    )

    # The same edit, with the row written first. One input moved.
    ledger(approved)
    r = edit(body, swap)
    results.check("rfc-changed-row-opens-the-gate", r is None, repr(r))

    # Per test, never per file: a row for the test next door approves nothing.
    ledger("| TestPlain | a different test entirely |\n")
    r = edit(body, swap)
    results.check(
        "rfc-changed-row-for-another-test-buys-nothing",
        blocked_on_rfc(r) and "| TestTagged |" in r[1],
        repr(r),
    )

    # `package.TestName` is the spelling both ledgers publish for an ambiguous
    # name, and the commit gate pairs on it. The hook accepts the same row, or an
    # author writing the qualified form is refused here and accepted there.
    ledger("| message.TestTagged | the qualified spelling of the same approval |\n")
    r = edit(body, swap)
    results.check("rfc-changed-qualified-row-opens-the-gate", r is None, repr(r))

    ledger("| web.TestTagged | the same name in another package |\n")
    r = edit(body, swap)
    results.check(
        "rfc-changed-wrong-package-qualifier-buys-nothing", blocked_on_rfc(r), repr(r)
    )

    # The old mechanism buys nothing now. A marker in the replacement text was the
    # whole approval until 2026-08-19, and the message must not send the author
    # back to writing one.
    ledger()
    marker = "\t// rfc-test-change-approved: 2026-08-19 user confirmed\n"
    r = edit(body, marker + swap)
    results.check(
        "rfc-changed-in-file-marker-no-longer-approves",
        blocked_on_rfc(r) and "test/rfc-changed.md" in r[1],
        repr(r),
    )
    results.check(
        "rfc-changed-message-does-not-teach-the-marker",
        r is not None and "// rfc-test-change-approved: <" not in r[1],
        repr(r),
    )

    # A ledger the parser cannot read fails CLOSED: a table nobody can pair
    # against is not an approval of everything in it.
    with open(ledger_path, "w", encoding="utf-8") as fh:
        fh.write("# RFC-tagged tests this commit changes\n\nno table at all\n")
    r = edit(body, swap)
    results.check("rfc-changed-unreadable-ledger-refuses", blocked_on_rfc(r), repr(r))

    # The untagged test in the same file owes nothing, whatever the ledger says.
    ledger()
    r = edit(other, other.replace("NoError", "Error"))
    results.check("rfc-changed-untagged-func-unaffected", r is None, repr(r))


# --------------------------------------------------------------------------- #
# mark-source-read: the spec-write gate's evidence set (T-4)
# --------------------------------------------------------------------------- #


_DESIGN_SID = "11111111-2222-3333-4444-555555555555"


def _mark_project() -> str:
    """A fixture project mark-source-read.sh and pretool-writeedit.py can share.

    The script cd's to CLAUDE_PROJECT_DIR and sources
    .claude/hooks/lib/session-id.sh relative to it, so the fixture project needs a
    copy of that lib. A fixed CLAUDE_CODE_SESSION_ID makes the marker path
    deterministic (env source wins in _session_id).
    """
    work = tempfile.mkdtemp(prefix="mark-source-read-", dir=_fixture_root())
    libdst = os.path.join(work, ".claude", "hooks", "lib")
    os.makedirs(libdst, exist_ok=True)
    shutil.copytree(os.path.join(HOOKS, "lib"), libdst, dirs_exist_ok=True)
    return work


def _read_source(work: str, file_path: str, response=_UNSET) -> None:
    """Drive mark-source-read.sh over a Read of `file_path` inside `work`.

    `response` is the PostToolUse `tool_response`. The default stands for a
    whole-file Read of a substantial file, which is what every case that is not
    ABOUT read depth wants. Pass None to omit it (an unrecognised payload shape),
    a dict to state exactly how much of the file the Read showed, or a string,
    which is what the harness returns when the Read FAILED.
    """
    ti: dict = {"file_path": file_path}
    if response is _UNSET:
        response = {"file": {"numLines": 400, "totalLines": 400}}
    if isinstance(response, dict):
        limit = response.pop("_limit", None)
        if limit is not None:
            ti["limit"] = limit
    payload: dict = {"tool_input": ti}
    if response is not None:
        payload["tool_response"] = response
    env = dict(os.environ, CLAUDE_PROJECT_DIR=work, CLAUDE_CODE_SESSION_ID=_DESIGN_SID)
    subprocess.run(
        ["bash", os.path.join(HOOKS, "mark-source-read.sh")],
        input=json.dumps(payload),
        text=True,
        capture_output=True,
        env=env,
        timeout=30,
    )


def _markers(work: str) -> set:
    sess = os.path.join(work, "tmp", "session")
    try:
        return set(os.listdir(sess))
    except OSError:
        return set()


def _run_mark_source_read(file_path: str, response=_UNSET) -> set:
    """Marker file names a Read of `file_path` produces (empty = nothing recorded)."""
    work = _mark_project()
    try:
        _read_source(work, file_path, response)
        return _markers(work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_draft_incubator(results: Results) -> None:
    """`test/draft/` holds work that is not a test yet, so the test guards skip it.

    A draft is gitignored and invisible to every repo-wide gate: it claims no
    evidence and proves no RFC obligation. Guarding it made the incubator the one
    directory an agent could fill and never empty, which is the opposite of a
    workflow whose only two endings are promote or delete
    (ai/rules/testing.md, "A draft is not a test yet").

    Both halves are pinned here, and so is the boundary: the same edit and the
    same deletion must still be refused for a LIVE test, or the exemption has
    eaten the rule it carves out of.
    """
    print("draft-incubator:")
    cw = _load_pretool_writeedit().c_test_weakening
    deletion = _load_pretool_bash().check_test_deletion

    tagged = (
        "# RFC requirement: RFC9552-5.2-1 positive - unknown NLRI preserved.\n"
        "expect=bgp:conn=2:seq=1:contains=DEADBEEF\n"
    )
    draft = "/repo/test/draft/plugin/wip.ci"
    live = "/repo/test/plugin/live.ci"

    def edit(path):
        return cw(
            {
                "tool": "Edit",
                "ti": {
                    "old_string": tagged,
                    "new_string": tagged.replace("DEADBEEF", "CAFE"),
                },
                "fp": path,
            }
        )

    # An `RFC requirement:` tag inside a draft is worth nothing until the file is
    # live, so the guard that protects the proof has no proof to protect.
    results.check(
        "draft-edit-rfc-tagged-passes", edit(draft) is None, repr(edit(draft))
    )

    # The boundary: promotion is what turns the tag into proof, so the identical
    # edit on a live test still blocks. Without this the case above proves only
    # that the guard is off.
    r = edit(live)
    results.check(
        "live-edit-rfc-tagged-still-blocks",
        r is not None and r[0] == 2 and "RFC-tagged test" in r[1],
        repr(r),
    )

    r = deletion("rm test/draft/plugin/wip.ci", None)
    results.check("draft-rm-needs-no-approval", r is None, repr(r))

    r = deletion("rm -r test/draft/plugin/", None)
    results.check("draft-rm-recursive-needs-no-approval", r is None, repr(r))

    r = deletion("rm test/plugin/live.ci", None)
    results.check("live-rm-still-needs-approval", r is not None and r[0] == 2, repr(r))

    # A command naming both is refused. The live test is the reason the guard
    # exists, and one draft in the argument list must not buy its removal.
    r = deletion("rm test/draft/plugin/wip.ci test/plugin/live.ci", None)
    results.check("mixed-rm-still-needs-approval", r is not None and r[0] == 2, repr(r))

    # The incubator root is a draft too, or the one directory an agent may fill
    # stays the one it may never empty. This is the boundary the matcher has to
    # keep while it normalizes: `test/draft/` normalizes to `test/draft`, which
    # is not under `test/draft/`.
    r = deletion("rm -r test/draft/", None)
    results.check("draft-root-rm-recursive-needs-no-approval", r is None, repr(r))

    r = deletion("rm test/draft/a.ci test/draft/b.ci", None)
    results.check("draft-pair-rm-needs-no-approval", r is None, repr(r))

    # A Go test carries no `test/` segment, so it is the shape a `test/`-only
    # matcher cannot see. Alone it blocked anyway, because an empty target list
    # is not a draft list; beside a draft it did not, and that was the defect.
    r = deletion("rm internal/x/y_test.go", None)
    results.check(
        "live-go-test-rm-still-needs-approval", r is not None and r[0] == 2, repr(r)
    )

    r = deletion("rm test/draft/a.ci internal/x/y_test.go", None)
    results.check(
        "mixed-go-test-rm-still-needs-approval", r is not None and r[0] == 2, repr(r)
    )

    # The block is raised over the whole command line, so the exemption is read
    # over the whole command line: a second segment is not a second command.
    r = deletion("rm test/draft/a.ci && rm internal/x/y_test.go", None)
    results.check(
        "draft-then-live-go-test-rm-still-needs-approval",
        r is not None and r[0] == 2,
        repr(r),
    )

    r = deletion("rm test/draft/a.ci\nrm -r test/encode", None)
    results.check(
        "draft-then-live-newline-rm-still-needs-approval",
        r is not None and r[0] == 2,
        repr(r),
    )

    r = deletion("rm -r test/draft/a.ci test/plugin/", None)
    results.check(
        "mixed-recursive-rm-still-needs-approval", r is not None and r[0] == 2, repr(r)
    )

    r = deletion("git rm test/draft/a.ci test/plugin/live", None)
    results.check(
        "mixed-git-rm-still-needs-approval", r is not None and r[0] == 2, repr(r)
    )

    # A path is matched by where it lands, not by the prefix it is spelled with.
    r = deletion("rm test/draft/../plugin/live.ci", None)
    results.check(
        "draft-traversal-rm-still-needs-approval", r is not None and r[0] == 2, repr(r)
    )


def run_mark_source_read(results: Results) -> None:
    """T-4 (AC-6): reading the .py/.sh/.yang/Makefile a spec is ABOUT must satisfy
    the spec-write gate -- the marker is written for those, not only for Go, so an
    agent need not read an unrelated .go file purely to pass. Each accepted Read
    also records its KIND, which is what lets the gate be scoped to the spec's own
    subject rather than relaxed for everyone (see design-gate below)."""
    print("mark-source-read:")

    aggregate = f".source-read-{_DESIGN_SID}"
    for label, path, kind in (
        ("go-internal", "/repo/internal/x/y.go", "go"),
        ("py-scripts", "/repo/scripts/dev/foo.py", "py"),
        ("py-hooks", "/repo/.claude/hooks/pretool-writeedit.py", "py"),
        ("sh-hooks", "/repo/.claude/hooks/foo.sh", "sh"),
        ("sh-scripts", "/repo/scripts/dev/verify-status.sh", "sh"),
        ("makefile", "/repo/Makefile", "make"),
        ("mk-file", "/repo/mk/inventory.mk", "make"),
        ("yang-model", "/repo/internal/component/iface/yang/ze-iface.yang", "yang"),
        # BLOCKER 1 (reviewer, 2026-08-07): the kind is the extension, so the
        # subjects real specs name are reachable. Each path below is a subject an
        # open spec lists today and NO accepted Read could record before: 11 specs
        # for py, 2 for sh. Their authors' only route past the gate was reading an
        # unrelated scripts/*.py.
        ("py-under-test", "/repo/test/interop-ipsec/lab.py", "py"),
        ("py-under-tools", "/repo/tools/kernel-builder/build.py", "py"),
        ("sh-under-packaging", "/repo/packaging/deb/preinstall.sh", "sh"),
        ("go-under-test", "/repo/test/interop/harness_test.go", "go"),
    ):
        written = _run_mark_source_read(path)
        results.check(
            f"mark-source-read-writes-{label}",
            aggregate in written and f".source-read-{kind}-{_DESIGN_SID}" in written,
            f"markers for {path}: {sorted(written)}",
        )

    # MUST-NOT-FIRE: an unrelated doc/spec read does not ground a spec, so it must
    # NOT write the marker (the gate stays honest about what counts as evidence).
    for label, path in (
        ("doc", "/repo/docs/guide/x.md"),
        ("spec", "/repo/plan/spec-foo.md"),
        ("json", "/repo/.claude/settings.json"),
        ("functional-test", "/repo/test/bgp/session.ci"),
        ("no-extension", "/repo/scripts/dev/some-tool"),
    ):
        results.check(
            f"mark-source-read-skips-{label}",
            not _run_mark_source_read(path),
            f"marker wrongly written for {path}",
        )

    # ISSUE 3 (reviewer, 2026-08-07): the gate is strict about WHICH file was read
    # and was trivial about HOW MUCH. Read(file, limit=1) cleared every spec of
    # that kind for 30 minutes. A window under 20 lines cannot have grounded a
    # claim about a producer, so it records nothing.
    for label, response, want in (
        ("keyhole-window", {"file": {"numLines": 1, "totalLines": 900}}, False),
        ("short-window", {"file": {"numLines": 19, "totalLines": 900}}, False),
        ("adequate-window", {"file": {"numLines": 20, "totalLines": 900}}, True),
        # A small file read ENTIRE is the producer, whatever its length.
        ("whole-small-file", {"file": {"numLines": 12, "totalLines": 12}}, True),
        # Content, when the harness sends text instead of a line count.
        ("content-counted", {"file": {"content": "x\n" * 3, "totalLines": 900}}, False),
        # No totalLines, so "was it the whole file" is unanswerable and the window
        # bar is the only thing left to judge by.
        ("num-without-total-short", {"file": {"numLines": 5}}, False),
        ("num-without-total-long", {"file": {"numLines": 50}}, True),
        # The request capped the window below the bar and the response is silent.
        ("limit-only-keyhole", {"_limit": 1}, False),
        ("limit-only-adequate", {"_limit": 200}, True),
        # BLOCKER 1 (reviewer, round 2): ZERO IS NOT UNMEASURABLE. Each shape
        # below shows the reader nothing, each was found in the real transcripts
        # under ~/.claude/projects, and each used to reach the fail-open default
        # and write a marker. Counted over 211 transcripts: 13, 36, 65.
        #
        # The harness answered a repeat Read without a body ("Wasted call -- file
        # unchanged since your last Read"), so this Read showed nothing while
        # renewing a 30-minute clearance.
        (
            "file-unchanged",
            {"type": "file_unchanged", "file": {"filePath": "/x"}},
            False,
        ),
        # A zero-byte file. numLines and totalLines are both 1 because the split
        # keeps an empty tail, so it used to pass as a whole-file read of a
        # one-line file: one Read of any empty .py cleared every py spec.
        (
            "empty-file",
            {
                "type": "text",
                "file": {
                    "filePath": "/x",
                    "content": "",
                    "numLines": 1,
                    "startLine": 1,
                    "totalLines": 1,
                },
            },
            False,
        ),
        # A FAILED Read: tool_response is a plain string, so jq could not index
        # it and every count defaulted to unmeasurable.
        ("read-error", "Error: File does not exist. Note: your cwd is /repo.", False),
        # UNRECOGNISED MUST STAY PERMISSIVE, and unrecognised now means exactly
        # that: no `file` object and not the failure string. A payload shape this
        # hook has never seen must not silently disable the evidence path and
        # block every spec write in the session. This is the fail-open half, and
        # it is deliberate -- the three cases above are what it stopped covering.
        ("unrecognised-no-response", None, True),
        ("unrecognised-shape", {"stdout": "..."}, True),
    ):
        written = _run_mark_source_read("/repo/internal/x/y.go", response)
        results.check(
            f"mark-source-read-depth-{label}",
            (f".source-read-go-{_DESIGN_SID}" in written) is want,
            f"markers for {response}: {sorted(written)}",
        )

    # The depth cases above use the fields the hook reads. These are the WHOLE
    # payload as the harness actually sends it, copied field for field off a real
    # transcript (`~/.claude/projects/*/*.jsonl`, `toolUseResult`). Asserting only
    # against an abstraction of the shape would keep passing if the shape moved.
    #
    # `shown` is the count of lines carrying TEXT; numLines and totalLines are one
    # HIGHER, because the harness splits on "\n" and keeps the empty tail. That is
    # measured, not assumed: numLines equalled the raw split length in all 5064
    # transcript records carrying both, and totalLines equalled `wc -l` + 1 for
    # all 118 whole-file reads of files still on disk.
    #
    # NOTE 6 (reviewer, round 2) is the boundary pair: 19 lines of text arrive as
    # numLines 20, so counting the phantom tail put a 19-line window over a bar
    # that exists to refuse it. The earlier cases sat at 3 and 4 lines, where the
    # off-by-one changes no verdict.
    for label, shown, total, want in (
        ("real-window", 4, 480, False),
        ("real-boundary-19-lines", 19, 480, False),
        ("real-boundary-20-lines", 20, 480, True),
        ("real-whole", 480, 480, True),
    ):
        written = _run_mark_source_read(
            "/repo/internal/x/y.go",
            {
                "type": "text",
                "file": {
                    "filePath": "/repo/internal/x/y.go",
                    "content": "package x\n" * shown,
                    "numLines": shown + 1,
                    "startLine": 1,
                    "totalLines": total + 1,
                },
            },
        )
        results.check(
            f"mark-source-read-depth-{label}",
            (f".source-read-go-{_DESIGN_SID}" in written) is want,
            f"markers for {shown} lines of {total}: {sorted(written)}",
        )


# --------------------------------------------------------------------------- #
# design-gate: c_design_without_lsp asks for the spec's OWN subject (T-4)
# --------------------------------------------------------------------------- #

_DESIGN_SPEC = """# Spec: fixture

## Task

Fixture spec for the design-without-lsp gate.

## Files to Modify

@FILES@

## Implementation Steps

1. Do the thing.
"""


def _write_spec(
    work: str,
    files_line: str,
    *,
    on_disk: bool = True,
    tool: str = "Write",
    trailer: str = "",
):
    """Drive the whole pretool-writeedit dispatcher over a spec write in `work`.

    Through the dispatcher for the reason _writeedit gives, and with
    CLAUDE_PROJECT_DIR pointed at the fixture so the gate reads the fixture's
    markers rather than this session's real ones. `tool` selects the payload
    SHAPE: a MultiEdit carries its text only inside `edits`, which is a place the
    gate has to look on purpose. `trailer` appends sections after Files to
    Modify, which is how a checklist subsection is put in the gate's way.
    """
    spec_text = _DESIGN_SPEC.replace("@FILES@", files_line + trailer)
    fp = os.path.join(work, "plan", "spec-fixture.md")
    os.makedirs(os.path.dirname(fp), exist_ok=True)
    if on_disk:
        with open(fp, "w", encoding="utf-8") as fh:
            fh.write(spec_text)
    if tool == "MultiEdit":
        ti = {
            "file_path": fp,
            "edits": [{"old_string": "", "new_string": spec_text}],
        }
    elif tool == "Write":
        ti = {"file_path": fp, "content": spec_text}
    else:
        ti = {"file_path": fp, "old_string": "x", "new_string": spec_text}
    payload = json.dumps({"tool_name": tool, "tool_input": ti})
    env = dict(os.environ, CLAUDE_PROJECT_DIR=work, CLAUDE_CODE_SESSION_ID=_DESIGN_SID)
    proc = subprocess.run(
        [sys.executable, os.path.join(HOOKS, "pretool-writeedit.py")],
        input=payload,
        capture_output=True,
        text=True,
        env=env,
        timeout=30,
    )
    return proc.returncode, proc.stderr


def _age_marker(work: str, name: str, seconds: int) -> None:
    """Backdate a marker, so a freshness window can be crossed without waiting."""
    path = os.path.join(work, "tmp", "session", name)
    old = time.time() - seconds
    os.utime(path, (old, old))


def _touch_marker(work: str, name: str) -> None:
    sess = os.path.join(work, "tmp", "session")
    os.makedirs(sess, exist_ok=True)
    with open(os.path.join(sess, name), "w", encoding="utf-8") as fh:
        fh.write("2026-08-07T00:00:00+00:00\n")


def _design_case(files_line: str, reads: tuple, *, read_response=_UNSET, **kw):
    """Read `reads` in a fresh fixture project, then write a spec about `files_line`.

    `read_response` is the `tool_response` every one of those Reads returned. The
    default is a whole-file read of a substantial file; pass a shape to state that
    the author was shown less than that, or nothing.
    """
    work = _mark_project()
    try:
        for path in reads:
            _read_source(work, path, read_response)
        return _write_spec(work, files_line, **kw)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def _design_blocked(r) -> bool:
    """Blocked BY THIS GATE. Advisory findings from sibling checks are not a block,
    and another check's exit 2 must not be read as this one firing. Every refusal
    this check emits carries its own name, so the test never guesses from prose."""
    rc, err = r
    return rc == 2 and "[design-without-lsp]" in err


def _design_degraded(r) -> bool:
    """The gate could not read a subject and SAID so. A permissive path that says
    nothing is the thing this asserts against (ai/rules/evidence.md)."""
    return "design-without-lsp: no subject read" in r[1]


def run_design_gate(results: Results) -> None:
    """T-4 (AC-6): the spec-write gate is SCOPED to the spec's own subject, not
    relaxed. A spec about a hook, a dev script or a YANG model is grounded by
    reading THAT file; a spec about the daemon still needs the daemon's Go, and
    an uninvestigated spec is still refused."""
    print("design-gate:")

    # SHOULD PASS NOW, DID NOT BEFORE: a hooks spec grounded by its own hook.
    # The hook it is about is Python (the 3 dispatchers), which wrote no marker at
    # all before, so writing that spec meant reading an unrelated .go file.
    r = _design_case(
        "- `.claude/hooks/pretool-writeedit.py` - the gate",
        ("/repo/.claude/hooks/pretool-writeedit.py",),
    )
    results.check(
        "design-gate-hooks-spec-reads-its-hook", not _design_blocked(r), repr(r)
    )

    # Same shape for a YANG spec and a dev-script spec.
    r = _design_case(
        "- `internal/component/iface/yang/ze-iface.yang` - the model",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/component/iface/yang/ze-iface.yang",),
    )
    results.check(
        "design-gate-yang-spec-reads-its-model", not _design_blocked(r), repr(r)
    )

    r = _design_case(
        "- `scripts/dev/commit_helper.py` - the helper",
        ("/repo/scripts/dev/commit_helper.py",),
    )
    results.check(
        "design-gate-tooling-spec-reads-its-tool", not _design_blocked(r), repr(r)
    )

    # MUST STILL FIRE: a daemon spec written with NOTHING investigated. This is the
    # refusal the gate exists for (inference-written specs, 2026-07-16).
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        (),
    )
    results.check(
        "design-gate-daemon-spec-uninvestigated-blocked", _design_blocked(r), repr(r)
    )

    # MUST STILL FIRE, and this is the scoping: a daemon spec grounded by reading a
    # shell hook is NOT grounded. Widening the accepted set without asking WHICH
    # kind the spec is about would have let this through -- relaxing the gate for
    # every Go spec in the repository.
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        (
            "/repo/.claude/hooks/foo.sh",
        ),  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    results.check(
        "design-gate-daemon-spec-grounded-by-hook-blocked", _design_blocked(r), repr(r)
    )

    # ...and the control: the same spec with its own Go read is allowed.
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        ("/repo/internal/x/y.go",),
    )
    results.check(
        "design-gate-daemon-spec-reads-its-go", not _design_blocked(r), repr(r)
    )

    # MUST STILL FIRE: the spec's own file, Read, but the harness showed the
    # author NOTHING. This is mark-source-read-depth-file-unchanged asserted at
    # the ENTRY POINT rather than at the marker file, which is where the operator
    # meets it: the refusal is what they get, and a guard tested only through its
    # helper is not tested (ai/rules/evidence.md).
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/x/y.go",),
        read_response={"type": "file_unchanged", "file": {"filePath": "/repo/x.go"}},
    )
    results.check(
        "design-gate-subject-read-showed-nothing-blocked", _design_blocked(r), repr(r)
    )

    # A spec that states no source subject (docs, a `.ci`, a bare directory) keeps
    # the pre-scoping bar: any implementation source, and still not nothing.
    r = _design_case(
        "- `docs/guide/x.md` - the page",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        ("/repo/scripts/dev/foo.py",),
    )
    results.check(
        "design-gate-subjectless-spec-any-source", not _design_blocked(r), repr(r)
    )

    r = _design_case(
        "- `docs/guide/x.md` - the page",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        (),
    )
    results.check(
        "design-gate-subjectless-spec-nothing-blocked", _design_blocked(r), repr(r)
    )

    # The aggregate marker says SOMETHING was read and never says what. It cannot
    # answer the question a subject-bearing spec asks, so it does not clear one:
    # the only thing it still carries is the subjectless path below.
    work = _mark_project()
    try:
        _touch_marker(work, f".source-read-{_DESIGN_SID}")
        r = _write_spec(
            work,
            "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        )  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        results.check(
            "design-gate-kindless-marker-not-enough-for-subject",
            _design_blocked(r),
            repr(r),
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The LSP tool is gopls, so an LSP invocation grounds a GO spec with no Read.
    work = _mark_project()
    try:
        _read_source(work, "/repo/.claude/hooks/foo.sh")  # a per-kind marker exists
        _touch_marker(work, f".lsp-invoked-{_DESIGN_SID}")
        r = _write_spec(
            work,
            "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        )  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        results.check(
            "design-gate-lsp-grounds-go-spec", not _design_blocked(r), repr(r)
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # BLOCKER 1 (reviewer, 2026-08-07): gopls is Go evidence and Go evidence ONLY.
    # An LSP-only session has no per-kind marker at all, and the old branch test
    # `kinds and any(per_kind)` fell through to the any-source bar, where the LSP
    # marker alone allowed a PYTHON spec. That is the inversion of the hole this
    # scoping closes, so it is asserted with no Read in the session whatsoever.
    work = _mark_project()
    try:
        _touch_marker(work, f".lsp-invoked-{_DESIGN_SID}")
        r = _write_spec(work, "- `scripts/dev/commit_helper.py` - the helper")
        results.check(
            "design-gate-lsp-alone-does-not-ground-py-spec",
            _design_blocked(r),
            repr(r),
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # BLOCKER 2: EVERY kind, not any kind. A spec naming a reactor `.go` beside an
    # `mk/*.mk` claims things about both. Any-of let the author list a cheap file
    # next to the expensive one and read only the cheap one.
    r = _design_case(
        "- `internal/component/bgp/reactor/peer.go` - the daemon\n"
        "- `mk/appliance.mk` - the build wiring",
        ("/repo/mk/appliance.mk",),
    )
    results.check(
        "design-gate-multi-kind-needs-every-kind", _design_blocked(r), repr(r)
    )

    # ...and reading both clears it, so the rule is every-kind, not go-only.
    r = _design_case(
        "- `internal/component/bgp/reactor/peer.go` - the daemon\n"
        "- `mk/appliance.mk` - the build wiring",
        ("/repo/mk/appliance.mk", "/repo/internal/component/bgp/reactor/peer.go"),
    )
    results.check(
        "design-gate-multi-kind-both-read-allowed", not _design_blocked(r), repr(r)
    )

    # BLOCKER 2, freshness half: each kind ages on its own clock. Taking the newest
    # mtime across kinds let a fresh `.mk` read carry a Go read that had gone stale,
    # which renews the expensive evidence for free every time the cheap file is
    # opened.
    work = _mark_project()
    try:
        _read_source(work, "/repo/internal/component/bgp/reactor/peer.go")
        _age_marker(work, f".source-read-go-{_DESIGN_SID}", 3 * 3600)
        _read_source(work, "/repo/mk/appliance.mk")
        r = _write_spec(
            work,
            "- `internal/component/bgp/reactor/peer.go` - the daemon\n"
            "- `mk/appliance.mk` - the build wiring",
        )
        results.check(
            "design-gate-fresh-kind-does-not-carry-stale-kind",
            _design_blocked(r) and "longer ago" in r[1],
            repr(r),
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # ISSUE 3: a subject the gate cannot read is the one permissive path left, so
    # it must SAY it degraded. Silence is what makes a weakened guard invisible.
    r = _design_case(
        "- `docs/guide/x.md` - the page",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        ("/repo/scripts/dev/foo.py",),
    )
    results.check("design-gate-subjectless-write-warns", _design_degraded(r), repr(r))

    # ISSUE 3: an un-backticked path in a table row is a subject too. Reading only
    # the backticked form derived nothing for 52 of 232 open specs, each of which
    # then took the weaker bar.
    r = _design_case(
        "| internal/component/bgp/reactor/peer.go | the daemon |",
        ("/repo/scripts/dev/foo.py",),
    )
    results.check("design-gate-bare-path-is-a-subject", _design_blocked(r), repr(r))

    # ISSUE 3: a MultiEdit carries its text only in `edits`. A spec authored that
    # way used to reach the gate with no visible subject at all.
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/scripts/dev/foo.py",),
        on_disk=False,
        tool="MultiEdit",
    )
    results.check("design-gate-multiedit-subject-seen", _design_blocked(r), repr(r))

    # ISSUE 5: the section ends at the next heading of ANY depth. `### Documentation
    # Checklist` rows name files the spec does not modify, and swallowing them let a
    # checklist `.yang` stand in for the Go this spec is actually about.
    _CHECKLIST = (
        "\n\n### Documentation Checklist\n\n"
        "- [ ] `internal/component/iface/yang/ze-iface.yang` documented\n"  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/component/iface/yang/ze-iface.yang",),
        trailer=_CHECKLIST,
    )
    results.check(
        "design-gate-checklist-row-is-not-a-subject", _design_blocked(r), repr(r)
    )

    # A spec whose new code all sits under `## Files to Create` is about that code
    # too. Reading only Files to Modify left such a spec subjectless on the weaker
    # bar (`plan/spec-anomaly-0-umbrella.md` is the shape: two docs to modify, its
    # Go to create).
    r = _design_case(
        "- `docs/features.md` - the feature row",
        ("/repo/scripts/dev/foo.py",),
        trailer="\n\n## Files to Create\n\n- `internal/plugins/anomaly/detect.go` - new",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    results.check(
        "design-gate-files-to-create-is-a-subject", _design_blocked(r), repr(r)
    )

    # ...and its companion: the checklist must not ADD a kind either, so the Go
    # read alone clears the same spec.
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/x/y.go",),
        trailer=_CHECKLIST,
    )
    results.check(
        "design-gate-checklist-adds-no-requirement", not _design_blocked(r), repr(r)
    )

    # A Write of a NEW spec has no file on disk, so the subject can only come from
    # the payload. Reading only the disk would silently fall back to "no subject".
    r = _design_case(
        "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/.claude/hooks/foo.sh",),
        on_disk=False,
    )
    results.check(
        "design-gate-new-spec-subject-from-payload", _design_blocked(r), repr(r)
    )

    # BLOCKER 2 (reviewer, 2026-08-07): EVERY row of _SUBJECT_PATTERNS must be
    # load-bearing in the REJECTING direction. Deleting the `sh`, `yang`,
    # `Makefile` or `.mk` row used to leave all 21 design-gate fixtures green,
    # because every rejecting case exercised `go` or `py`: no fixture made those
    # kinds the subject-under-test, so the rows were decoration.
    #
    # The shape is one spec naming ONE kind, grounded by a DIFFERENT kind. Delete
    # that kind's row and the spec becomes subjectless, the foreign read satisfies
    # the weaker any-source bar, and the case goes green -- which is the red.
    # `Makefile` and `.mk` are two rows and so are two cases.
    for label, files_line, read in (
        (
            "go",
            "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
            "/repo/.claude/hooks/foo.sh",
        ),  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        (
            "py",
            "- `scripts/dev/commit_helper.py` - the helper",
            "/repo/internal/x/y.go",
        ),
        (
            "sh",
            "- `.claude/hooks/mark-source-read.sh` - the writer",
            "/repo/internal/x/y.go",
        ),
        (
            "yang",
            "- `internal/component/iface/yang/ze-iface.yang` - the model",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
            "/repo/internal/x/y.go",
        ),
        ("makefile", "- `Makefile` - the entry point", "/repo/internal/x/y.go"),
        ("mk", "- `mk/appliance.mk` - the build wiring", "/repo/internal/x/y.go"),
    ):
        r = _design_case(files_line, (read,))
        results.check(
            f"design-gate-{label}-subject-needs-its-own-kind",
            _design_blocked(r),
            repr(r),
        )
        # ...and the control, which is what makes the case above a scoping test
        # rather than a "nothing ever passes" test: the spec's OWN subject clears
        # it. This half is also the BLOCKER 1 property, asserted per kind: the
        # file the spec NAMES is a file a Read can record.
        subject = files_line.split("`")[1]
        r = _design_case(files_line, ("/repo/" + subject.lstrip("/"),))
        results.check(
            f"design-gate-{label}-subject-cleared-by-its-own-file",
            not _design_blocked(r),
            repr(r),
        )

    # BLOCKER 1: the two ends of the kind contract, walked together. The reader
    # derives what a spec DEMANDS and the writer records what a Read SUPPLIES, and
    # when they disagree the only exit from a block is reading a file the spec does
    # not name -- the gate manufacturing the evidence it exists to demand. Real
    # subjects from open specs, so a directory anchor creeping back into either end
    # is a named red here.
    mod = _load_pretool_writeedit()
    for path in (
        "internal/component/bgp/reactor/peer.go",
        "test/interop/harness_test.go",
        "scripts/dev/commit_helper.py",
        ".claude/hooks/pretool-writeedit.py",
        "test/interop-ipsec/lab.py",
        "tools/kernel-builder/build.py",
        ".claude/hooks/mark-source-read.sh",
        "packaging/deb/preinstall.sh",
        "test/interop-ipsec/scenarios/06-eap-tls13/pki/gen-pki.sh",
        "Makefile",
        "mk/appliance.mk",
        "internal/component/iface/yang/ze-iface.yang",
        "docs/guide/x.md",
        "test/bgp/session.ci",
    ):
        written = _run_mark_source_read("/repo/" + path)
        supplied = {
            name.split("-")[2]
            for name in written
            if name.startswith(".source-read-")
            and name != f".source-read-{_DESIGN_SID}"
        }
        demanded = mod._spec_subject_kinds(
            "## Files to Modify\n\n- `%s` - the file\n" % path
        )
        results.check(
            f"design-gate-contract-both-ends-agree-{path.replace('/', '_')}",
            supplied == demanded,
            f"{path}: writer supplies {sorted(supplied)}, gate demands {sorted(demanded)}",
        )

    # ISSUE 4 (reviewer, 2026-08-07): the refusal must name the section that
    # actually carries the subject. A spec whose Go sits only under Files to
    # Create was sent to "Files to Modify", which does not hold it.
    r = _design_case(
        "- `docs/features.md` - the feature row",
        ("/repo/scripts/dev/foo.py",),
        trailer="\n\n## Files to Create\n\n- `internal/plugins/anomaly/detect.go` - new",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
    )
    results.check(
        "design-gate-message-names-both-sections",
        _design_blocked(r) and "Files to Modify / Files to Create" in r[1],
        repr(r),
    )

    # NOTE 7 (reviewer, 2026-08-07): a table's second column is prose. Heading
    # depth already stopped a `### Checklist` row becoming a subject; this is the
    # same hole one row further in, where a description mentioning a helper made
    # that helper's kind a requirement of a spec that modifies no Python.
    r = _design_case(
        "| `internal/x/y.go` | port the logic out of `scripts/dev/foo.py` |",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/x/y.go",),
    )
    results.check(
        "design-gate-description-column-is-not-a-subject",
        not _design_blocked(r),
        repr(r),
    )

    # ...and the first cell of that same row still IS one, so the fix narrowed the
    # scan without blinding it.
    r = _design_case(
        "| `scripts/dev/foo.py` | port the logic out of it |",  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        ("/repo/internal/x/y.go",),
    )
    results.check(
        "design-gate-first-cell-is-still-a-subject", _design_blocked(r), repr(r)
    )

    # ISSUE 3: the depth bar reaches the gate, not just the writer. A keyhole Read
    # of the spec's own subject leaves the session ungrounded.
    work = _mark_project()
    try:
        _read_source(
            work, "/repo/internal/x/y.go", {"file": {"numLines": 1, "totalLines": 900}}
        )
        r = _write_spec(
            work,
            "- `internal/x/y.go` - the daemon",  # <!-- doc-links: ignore (fixture literal in a hook test corpus, deliberately absent from the tree) -->
        )  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
        results.check(
            "design-gate-keyhole-read-does-not-ground", _design_blocked(r), repr(r)
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


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


def _deleg_state_dir(work: str) -> str:
    """The fixture session's `state/` directory, created.

    The same glob-then-name rule the shell (`lib/session-dir.sh`), make and Go
    use: the dated directory that already carries this id, else today's. A
    fixture that spelled `tmp/session/` flat would pin the location the digest
    left (plan/spec-session-bin-directory.md, AC-20).
    """
    root = os.path.join(work, "tmp", "session")
    found = sorted(
        d
        for d in glob.glob(os.path.join(root, f"????-??-??-{_DELEG_SID}"))
        if os.path.isdir(d)
    )
    session = (
        found[0]
        if found
        else os.path.join(root, f"{datetime.date.today().isoformat()}-{_DELEG_SID}")
    )
    state = os.path.join(session, "state")
    os.makedirs(state, exist_ok=True)
    return state


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
    """ai/rules/planning.md: a session that claimed a spec and never
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
            input=json.dumps({"session_id": _DELEG_SID}),
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
            "planning.md" in r.stdout and "evidence.md" in r.stdout,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # No spec claimed: the context still loads, minus the spec block.
    work = _deleg_project(spec=None)
    try:
        r = subprocess.run(
            ["bash", os.path.join(HOOKS, "subagent-context.sh")],
            input=json.dumps({"session_id": _DELEG_SID}),
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
    # still run first. No hook releases the claim now: `spec-session.sh release`
    # does, from /ze-close, which is what keeps the claim alive past turn one.
    # `delegation-claim-survives-stop` below pins that half, and this fixture
    # pins the ordering half. Neither alone makes the three gates work.
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

        # The REMEDY the block prints must not read as an order to go and do the
        # work that was just offered. It did until 2026-08-19, when the whole
        # remedy was "Continue without asking permission". That answers
        # permission-seeking, and the same list catches OFFERS of work nobody
        # commissioned, where the correct move is the opposite one: drop it. A
        # turn ending "Want me to spec the streaming writer?" was refused its end
        # and then wrote that spec, so the gate against uncommissioned work was
        # manufacturing it. rc == 2 alone cannot see that, so assert the routing
        # text and the absence of the old line.
        rc, err = _run_stop_hook(work, "Done.\n\nWant me to spec the streaming writer?")
        results.check(
            "stop-phrase-remedy-does-not-order-the-offered-work",
            rc == 2
            and "DROP IT" in err
            and "not an instruction to do the work you just offered" in err
            and "Continue without asking permission" not in err,
            f"rc={rc} {err!r}",
        )

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
        unclosed = "Intro\n\n```bash\nmake ze-precommit-verify\n\nDone. Would you like me to continue?"
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

        # ai/rules/completion.md MANDATES this exact line for a problem found while
        # doing something else: spec it, close the work in hand, ask. Refusing it
        # would order a sentence and then refuse the turn that carries it. It needs
        # no exemption -- it matches no pattern in either list -- and this fixture
        # is what keeps that true: a future PHRASES entry that swallowed it goes red
        # here. Filtering the scan's input to protect it is banned; see the comment
        # above SCAN_PATTERNS in the hook for what that cost when it was tried.
        rc, err = _run_stop_hook(
            work,
            "Closed and committed. New spec: `plan/bgp-med-strip.md`. "  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
            "Implement it? (yes / not now)",
        )
        results.check(
            "stop-phrase-mandated-spec-ask-allowed",
            rc == 0,
            f"rc={rc} {err}",
        )

        # The mandated ask licenses no OTHER request. This is the fixture that fails
        # if anybody reintroduces a line filter for the mandated form.
        # The banned phrase sits on the SAME physical line as the mandated ask, which
        # is precisely what a line filter would swallow. A separate line would pass
        # this assertion with or without such a filter and would pin nothing.
        rc, err = _run_stop_hook(
            work,
            "New spec: `plan/bgp-med-strip.md`. Implement it? (yes / not now) "  # <!-- doc-links: ignore (fixture path in a hook case, deliberately absent) -->
            "Would you like me to run the tests first?",
        )
        results.check(
            "stop-phrase-mandated-ask-does-not-cover-a-second-request",
            rc == 2 and "would you like me to" in err,
            f"rc={rc} {err}",
        )

        # An UNPAIRED backtick must not delete the request. A left-to-right strip
        # pairs the stray tick with the OPENING tick of the later legitimate span
        # and removes everything between, which is where the request sits. A
        # dropped closing backtick is an ordinary typo, so this needs no intent.
        stray = (
            "I fixed the ` escaping bug. "
            "Would you like me to also run `make ze-precommit-verify`?"
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
                    _deleg_state_dir(work), f"session-state-fixture-{_DELEG_SID}.md"
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

    # The OFF half of the same lifetime, inverted on 2026-08-10.
    #
    # The claim used to be released at SessionEnd, by a hook whose other job was
    # `rm -rf` of the whole session directory. Both are gone: nothing under
    # tmp/session/ is ever removed automatically, at any event or on any timer
    # (owner decision 2026-08-03, plan/spec-session-bin-directory.md AC-7). The
    # claim now ages out of RELEVANCE rather than existence, and the heartbeat
    # below is what dates it.
    #
    # Deletion at that event can only come back through settings.json, so that
    # is what this reads. A hook file nobody registers runs never, and a
    # registered one runs on every session end in this checkout.
    with open(os.path.join(ROOT, ".claude", "settings.json")) as fh:
        session_end_hooks = json.load(fh).get("hooks", {}).get("SessionEnd", [])
    results.check(
        "delegation-no-session-end-hook-registered",
        session_end_hooks == [],
        f"a SessionEnd hook is registered again: {session_end_hooks!r}. The only "
        "work that event ever did here was deleting tmp/session/, which AC-7 bans",
    )

    # The claim's mtime is a LIVENESS date, so a session that is still working
    # must refresh it: reaching a Stop proves the session is alive. Nothing
    # deletes a stale claim any more, so the mtime is the only thing that tells
    # a live claim from one a dead session left behind.
    work = _deleg_project(spec="spec-fixture.md", spawned=False)
    try:
        marker = os.path.join(work, "tmp", "session", f".session-{_DELEG_SID}")
        old = time.time() - 25 * 3600
        os.utime(marker, (old, old))
        _run_stop_hook(work)
        results.check(
            "delegation-claim-heartbeat-on-stop",
            os.path.getmtime(marker) > old + 3600,
            "the hook did not refresh the claim, so a >24h session cannot be "
            "told from a dead one that left its claim behind",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # SessionStart deletes NOTHING. It once ran a seven-`find` sweep over
    # tmp/session/, reclaimed "orphaned" state files, and reaped whole dated
    # session directories whose files were 24h idle. Every one of those removed
    # the operator's data without being asked, and the orphan reclaim deleted a
    # LIVE session's state file whenever the sid parse missed (incident R-11).
    #
    # Everything planted here is aged past the old 24h threshold and would have
    # been swept by name. Each survivor is one deleted `find`, so this fixture
    # goes red the moment any of them is written back, whatever it is called.
    work = _deleg_project(spec="spec-fixture.md", spawned=True)
    try:
        sess = os.path.join(work, "tmp", "session")
        old = time.time() - 26 * 3600
        uuid_sid = "8d3d7c6b-fbad-4077-8f06-4678828041d0"
        planted = [
            # The claim, and the gate markers the mark-*.sh hooks write.
            os.path.join(sess, f".session-{_DELEG_SID}"),
            os.path.join(sess, f".agent-spawned-{_DELEG_SID}"),
            os.path.join(sess, ".compaction-detected-dead-fixture-sid"),
            os.path.join(sess, ".source-read-dead-fixture-sid"),
            os.path.join(sess, ".lsp-invoked-dead-fixture-sid"),
            # The minted-id cache. Its expiry was the one sweep doing real work,
            # and session_id.py `_cache_key` replaced it with a start-time key.
            os.path.join(sess, ".sid-by-pid-4242-99"),
            # A state file with NO marker beside it: the orphan the reclaim took.
            os.path.join(sess, f"session-state-spec-vrrp-4-transport-{uuid_sid}.md"),
        ]
        for path in planted:
            if not os.path.exists(path):
                with open(path, "w") as fh:
                    fh.write("planted\n")
            os.utime(path, (old, old))
        # A whole session directory, idle past the threshold: what --reap took.
        dead_dir = os.path.join(sess, "2026-01-02-dead-fixture-sid")
        os.makedirs(os.path.join(dead_dir, "bin"), exist_ok=True)
        binary = os.path.join(dead_dir, "bin", "ze")
        with open(binary, "w") as fh:
            fh.write("binary\n")
        for path in (binary, os.path.join(dead_dir, "bin"), dead_dir):
            os.utime(path, (old, old))

        start = subprocess.run(
            ["bash", os.path.join(HOOKS, "session-start.sh")],
            input="{}",
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        gone = [os.path.basename(p) for p in planted if not os.path.exists(p)]
        results.check(
            "delegation-session-start-deletes-no-marker",
            start.returncode == 0 and not gone,
            f"SessionStart removed {gone} (rc={start.returncode}); nothing under "
            "tmp/session/ may be deleted automatically",
        )
        results.check(
            "delegation-session-start-reaps-no-session-dir",
            os.path.isfile(binary),
            "SessionStart reaped a dated session directory and the binaries in it",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The spawn marker ages out too, and losing it is worse than losing the claim:
    # the nudge fires FALSELY and tells a properly supervising session it never
    # delegated. Newly reachable, because the claim now survives long enough for a
    # session to still be gated when the spawn marker expires.
    work = _deleg_project(spec="spec-fixture.md", spawned=True)
    try:
        spawned = os.path.join(work, "tmp", "session", f".agent-spawned-{_DELEG_SID}")
        old = time.time() - 25 * 3600
        os.utime(spawned, (old, old))
        _, err = _run_stop_hook(work)
        results.check(
            "delegation-spawn-marker-heartbeat",
            os.path.getmtime(spawned) > old + 3600 and "Delegation:" not in err,
            f"spawn marker not refreshed, so a >24h session that DID delegate "
            f"gets a false nudge: {err!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The heartbeat must never CREATE a marker. A bare touch would resurrect a
    # claim released by `spec-session.sh release` as an EMPTY file, which
    # silently skips every gate below it, and would invent a spawn marker for a
    # session that never delegated, silencing the nudge it exists to raise.
    work = _deleg_project(spec="spec-fixture.md", spawned=False)
    try:
        spawned = os.path.join(work, "tmp", "session", f".agent-spawned-{_DELEG_SID}")
        _, err = _run_stop_hook(work)
        results.check(
            "delegation-heartbeat-never-creates-spawn-marker",
            not os.path.isfile(spawned) and "Delegation:" in err,
            "the heartbeat invented a spawn marker, silencing the nudge",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# --------------------------------------------------------------------------- #
# session-state: what survives a rewrite of the per-spec state file
# --------------------------------------------------------------------------- #

_HANDOFF = """## Phase 4 handoff

Files changed:
- `internal/x/y.go` (380L): the reactor hook. Key: `Run()`, `handleOpen()`. <!-- doc-links: ignore (fixture handoff body, deliberately absent) -->

Acceptance criteria covered: AC-3, proven by TestForwardRail.

Do not assume: the stub in `z.go` is gone.
"""


def _snapshot(stamp: str, path: str = "a.go") -> str:
    """One block in the exact shape session-end-summary.sh writes."""
    return (
        f"## Session: {stamp}\n"
        "\n"
        "Branch: `main`\n"
        "Last commit: abc1234 fixture\n"
        "Spec: `spec-fixture.md`\n"
        "\n"
        "Uncommitted:\n"
        f"- `{path}`\n"
    )


def _state_project(body: str) -> tuple[str, str]:
    """A fixture project whose per-spec state file already holds `body`, with a
    dirty git tree so session-end-summary.sh runs its whole path instead of
    returning early on a clean tree."""
    work = _deleg_project(spec="spec-fixture.md")
    subprocess.run(["git", "init", "-q", "."], cwd=work, capture_output=True)
    with open(os.path.join(work, "dirty.txt"), "w") as fh:
        fh.write("uncommitted\n")
    state = os.path.join(
        _deleg_state_dir(work), f"session-state-fixture-{_DELEG_SID}.md"
    )
    with open(state, "w", encoding="utf-8") as fh:
        fh.write("# Session State\n\n" + body)
    return work, state


def _run_state_hook(work: str, hook: str) -> int:
    r = subprocess.run(
        ["bash", os.path.join(HOOKS, hook)],
        input="{}",
        text=True,
        capture_output=True,
        env=_deleg_env(work),
        timeout=60,
    )
    return r.returncode


def run_session_state(results: Results) -> None:
    """session-end-summary.sh rewrites the per-spec state file on every Stop.
    Its salvage used to be POSITIONAL -- print from the first `## Session:`
    line, stop at the third -- so once two snapshots existed, everything after
    them was outside the window and the rewrite deleted it. Phase agents append
    their handoffs at the END of that file (ai/skills/ze-implement.md) and
    .claude/rules/post-compaction.md makes it Tier 1 recovery, so on 2026-08-09
    a phase 4 handoff and a set of main-thread notes were destroyed and the next
    phase had to re-derive them from the tree."""
    print("session-state:")

    # The reported failure, in the shape the writer itself produces: this hook
    # keeps THREE snapshots, and the old window closed at the third `## Session:`
    # heading, so a handoff appended after them was outside it. Two snapshots
    # would NOT reproduce the loss -- with no third heading the old awk ran to
    # EOF and carried the handoff through, so a two-snapshot fixture passes with
    # the bug restored (ai/rules/interop-and-goal-validation.md).
    body = "\n---\n".join(
        [
            _snapshot("2026-08-09T09:00:00+01:00", "f1.go"),
            _snapshot("2026-08-09T08:00:00+01:00", "f2.go"),
            _snapshot("2026-08-09T07:00:00+01:00", "f3.go"),
            _HANDOFF,
        ]
    )
    work, state = _state_project(body)
    try:
        rc = _run_state_hook(work, "session-end-summary.sh")
        text = open(state, encoding="utf-8").read()
        results.check(
            "session-state-handoff-after-the-kept-snapshots-survives",
            rc == 0 and "## Phase 4 handoff" in text and "handleOpen()" in text,
            f"rc={rc} the Stop hook dropped the handoff:\n{text}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # Position must not decide either way: a handoff BETWEEN snapshots is kept
    # because it is not a snapshot, and the snapshots around it still rotate.
    # It sits after the third heading here for the reason above -- that is where
    # the old window closed, so this is the placement that discriminates.
    body = "\n---\n".join(
        [
            _snapshot("2026-08-09T09:00:00+01:00", "f1.go"),
            _snapshot("2026-08-09T08:00:00+01:00", "f2.go"),
            _snapshot("2026-08-09T07:00:00+01:00", "f3.go"),
            _HANDOFF,
            _snapshot("2026-08-09T06:00:00+01:00", "f4.go"),
        ]
    )
    work, state = _state_project(body)
    try:
        rc = _run_state_hook(work, "session-end-summary.sh")
        text = open(state, encoding="utf-8").read()
        results.check(
            "session-state-handoff-between-snapshots-survives",
            rc == 0
            and "## Phase 4 handoff" in text
            and "handleOpen()" in text
            and text.count("## Session:") == 3,
            f"rc={rc} snapshots={text.count('## Session:')}:\n{text}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # MUST-NOT-FIRE the other way: preserving everything is not the fix either.
    # Snapshots still rotate to the newest plus two, so four old ones leave two.
    body = "\n---\n".join(
        _snapshot(f"2026-08-09T0{i}:00:00+01:00", f"f{i}.go") for i in range(4, 0, -1)
    )
    work, state = _state_project(body)
    try:
        rc = _run_state_hook(work, "session-end-summary.sh")
        text = open(state, encoding="utf-8").read()
        kept = text.count("## Session:")
        results.check(
            "session-state-four-snapshots-rotate-to-three",
            rc == 0 and kept == 3 and "f1.go" not in text and "f2.go" not in text,
            f"rc={rc} kept={kept} (want the new snapshot plus the two newest):\n{text}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # pre-compact-save.sh writes the same header over the same file. It carries
    # the whole file through today; this pins that it keeps doing so, since a
    # PreCompact rewrite lands exactly when the handoff is most needed.
    body = "\n---\n".join([_snapshot("2026-08-09T09:00:00+01:00"), _HANDOFF])
    work, state = _state_project(body)
    try:
        rc = _run_state_hook(work, "pre-compact-save.sh")
        text = open(state, encoding="utf-8").read()
        results.check(
            "session-state-precompact-keeps-handoff",
            rc == 0
            and "## Phase 4 handoff" in text
            and "handleOpen()" in text
            and "## Last Compaction" in text,
            f"rc={rc} pre-compact-save.sh dropped the handoff:\n{text}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# --------------------------------------------------------------------------- #
# session-state-location: WHERE the digest lands, and where it is found
# (plan/spec-session-bin-directory.md AC-20, AC-21, AC-22)
# --------------------------------------------------------------------------- #


def _run_state_lib(work: str, snippet: str) -> subprocess.CompletedProcess:
    """Drive lib/state-file.sh directly, from the fixture project root.

    The functions are the producers AC-20 and AC-21 are about, so the fixture
    calls them rather than asserting over a path this file spelled itself.
    """
    return subprocess.run(
        ["bash", "-c", "source .claude/hooks/lib/state-file.sh\n" + snippet],
        cwd=work,
        text=True,
        capture_output=True,
        env=_deleg_env(work),
        timeout=60,
    )


def _plant(path: str, body: str = "# state\n", age_hours: float = 0.0) -> str:
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        fh.write(body)
    if age_hours:
        old = time.time() - age_hours * 3600
        os.utime(path, (old, old))
    return path


def run_session_state_location(results: Results) -> None:
    """The per-spec digest lives in the directory of the session that wrote it.

    It used to sit flat under tmp/session/, because _find_latest_state_for_spec
    reads it ACROSS sessions and a per-session directory looked like it would
    hide it. A glob that walks every session directory's state/ reads it equally
    well, so the digest joined bin/ and scratch/ under
    tmp/session/<YYYY-MM-DD>-<sid>/ (owner decision 2026-08-10).

    Digests written before that move still sit flat, this spec's own among them.
    The resolver reads both locations newest-first: deleting the flat branch
    makes every one of them unreachable, and a resolver that returns nothing
    looks exactly like a spec with no prior phase.
    """
    print("session-state-location:")

    # AC-20: the digest path names the session's own directory, under state/.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        r = _run_state_lib(work, "_state_file")
        path = r.stdout.strip()
        want = os.path.join(
            f"{datetime.date.today().isoformat()}-{_DELEG_SID}",
            "state",
            f"session-state-fixture-{_DELEG_SID}.md",
        )
        results.check(
            "session-state-digest-lands-in-the-session-directory",
            r.returncode == 0 and path.endswith(want),
            f"rc={r.returncode} path={path!r} want tail {want!r}",
        )
        results.check(
            "session-state-digest-directory-is-created",
            os.path.isdir(os.path.join(work, os.path.dirname(path))),
            f"_state_file named {path!r} without creating its directory",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # AC-20, the other half: the writers put NOTHING flat under tmp/session/.
    # A Stop and a PreCompact both rewrite the digest, so both are driven.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        subprocess.run(["git", "init", "-q", "."], cwd=work, capture_output=True)
        _plant(os.path.join(work, "dirty.txt"), "uncommitted\n")
        for hook in ("session-end-summary.sh", "pre-compact-save.sh"):
            _run_state_hook(work, hook)
        flat = glob.glob(os.path.join(work, "tmp", "session", "session-state-*.md"))
        nested = glob.glob(
            os.path.join(work, "tmp", "session", "*", "state", "session-state-*.md")
        )
        results.check(
            "session-state-nothing-is-written-flat",
            not flat and len(nested) == 1,
            f"flat={[os.path.basename(p) for p in flat]} nested={len(nested)}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # AC-13's rule applies here too: the directory is LOOKED UP. A session whose
    # directory was made on an earlier day keeps writing into that directory,
    # rather than starting a second one at midnight.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        os.makedirs(
            os.path.join(work, "tmp", "session", f"2026-01-02-{_DELEG_SID}", "state")
        )
        r = _run_state_lib(work, "_state_file")
        results.check(
            "session-state-digest-reuses-an-existing-dated-directory",
            r.returncode == 0 and f"2026-01-02-{_DELEG_SID}/state/" in r.stdout,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # AC-21: an earlier session's digest, inside that session's own directory,
    # is found by stem. The flat digest planted beside it is OLDER, so the
    # newest-first order is what decides -- and it must be the nested one.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        sess = os.path.join(work, "tmp", "session")
        _plant(
            os.path.join(sess, "session-state-fixture-oldsession.md"),
            "# flat digest\n",
            age_hours=48,
        )
        nested = _plant(
            os.path.join(
                sess, "2026-08-01-oldsession", "state", "session-state-fixture-old2.md"
            ),
            "# nested digest\n",
            age_hours=1,
        )
        r = _run_state_lib(work, '_find_latest_state_for_spec "fixture"')
        results.check(
            "session-state-resolver-walks-every-session-directory",
            r.returncode == 0
            and r.stdout.strip().endswith("session-state-fixture-old2.md"),
            f"want the nested digest {nested!r}, got {r.stdout.strip()!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # BACK-COMPATIBILITY, and the reason the flat branch is not deleted: a
    # digest written before the move is the ONLY one a resuming session has.
    # This case is what goes red if that branch goes away.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        flat = _plant(
            os.path.join(work, "tmp", "session", "session-state-fixture-oldsession.md"),
            "# the only digest this spec has\n",
        )
        r = _run_state_lib(work, '_find_latest_state_for_spec "fixture"')
        results.check(
            "session-state-resolver-still-finds-a-flat-digest",
            r.returncode == 0
            and r.stdout.strip().endswith("session-state-fixture-oldsession.md"),
            f"the pre-move digest {flat!r} is unreachable; the resolver said "
            f"{r.stdout.strip()!r}, which reads as a spec with no prior phase",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The reverse order proves the same branch is not simply first: a nested
    # digest alone resolves too.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        _plant(
            os.path.join(
                work,
                "tmp",
                "session",
                "2026-08-01-oldsession",
                "state",
                "session-state-fixture-old2.md",
            )
        )
        r = _run_state_lib(work, '_find_latest_state_for_spec "fixture"')
        results.check(
            "session-state-resolver-finds-a-nested-digest-alone",
            r.returncode == 0
            and r.stdout.strip().endswith("session-state-fixture-old2.md"),
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # The two .claude/ fallbacks predate tmp/session/ entirely and still resolve.
    for name, want in (
        ("session-state-fixture-ancientsid.md", "session-state-fixture-ancientsid.md"),
        ("session-state-fixture.md", "session-state-fixture.md"),
    ):
        work = _deleg_project(spec="spec-fixture.md")
        try:
            _plant(os.path.join(work, ".claude", name))
            r = _run_state_lib(work, '_find_latest_state_for_spec "fixture"')
            results.check(
                f"session-state-resolver-legacy-{name}",
                r.returncode == 0 and r.stdout.strip().endswith(want),
                r.stdout,
            )
        finally:
            shutil.rmtree(work, ignore_errors=True)

    # AC-22: the two files that CANNOT move stay flat, and nothing this change
    # touches puts them anywhere else. .sid-by-pid-<clipid> mints the id the
    # directory is named for, and .closure-ack-<stem> is keyed by spec stem, so
    # it outlives every session that reads it. The gate markers stay flat too
    # (2026-08-03 decision, unchanged): the whole session-start run is driven
    # here so a marker moved by any hook shows up.
    work = _deleg_project(spec="spec-fixture.md", spawned=True)
    try:
        sess = os.path.join(work, "tmp", "session")
        planted = [
            ".sid-by-pid-4242-99",
            ".closure-ack-fixture",
            f".lsp-loaded-{_DELEG_SID}",
            f".source-read-{_DELEG_SID}",
            f".model-ack-{_DELEG_SID}",
            f".compaction-detected-{_DELEG_SID}",
        ]
        for name in planted:
            _plant(os.path.join(sess, name), "planted\n")
        # _deleg_project wrote these two; re-planting .session-<sid> would
        # destroy the spec claim session-start.sh is about to read.
        flat_names = planted + [
            f".session-{_DELEG_SID}",
            f".agent-spawned-{_DELEG_SID}",
        ]
        subprocess.run(
            ["bash", os.path.join(HOOKS, "session-start.sh")],
            input="{}",
            text=True,
            capture_output=True,
            env=_deleg_env(work),
            timeout=60,
        )
        moved = [n for n in flat_names if not os.path.isfile(os.path.join(sess, n))]
        strays = [
            os.path.basename(p)
            for p in glob.glob(os.path.join(sess, "*", "state", ".*"))
        ]
        results.check(
            "session-state-flat-markers-do-not-move",
            not moved and not strays,
            f"missing from tmp/session/: {moved}; found under state/: {strays}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


# --------------------------------------------------------------------------- #
# subagent-context: what the spawn-time injection carries (AC-5)
# The delegation section above pins that the block names the parent's spec.
# This one pins its CONTENT: the context-economy directives, and the per-spec
# digest path when one exists.
# --------------------------------------------------------------------------- #


def _subagent_context(work: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        ["bash", os.path.join(HOOKS, "subagent-context.sh")],
        input=json.dumps({"session_id": _DELEG_SID}),
        text=True,
        capture_output=True,
        env=_deleg_env(work),
        timeout=30,
    )


def run_subagent_context(results: Results) -> None:
    """plan/spec-context-economy.md AC-5: every spawned agent is told to batch
    its tool calls and to read the range it was handed rather than a whole file,
    and is handed the per-spec digest when the parent session has one. R-4 caps
    this at the directives with measured value, so the rule itself stays a path,
    not a paste."""
    print("subagent-context:")

    # The directives reach every agent, spec or no spec, and they point at the
    # rule rather than restating it.
    work = _deleg_project(spec=None)
    try:
        r = _subagent_context(work)
        out = r.stdout
        results.check(
            "subagent-context-carries-economy",
            r.returncode == 0
            and "ONE message" in out
            and "Read the range" in out
            and "Grep" in out
            and "ai/rules/context-economy.md" in out,
            out,
        )
        # Whether a spawned agent's registry carries the LSP tool is a property
        # of the harness build and the machine, and both change: one dev machine
        # here answers "No matching deferred tools found", the other serves the
        # tool. So the injection must NOT assert absence ("NO LSP") -- an agent
        # told it has none stops looking, on the machine where it does. It
        # carries the two-step resolution order instead: query the tool, fall
        # back to gopls on PATH, which ze-setup guarantees. Both routes must be
        # named, and the range the prompt already carries stays preferred to
        # either.
        results.check(
            "subagent-context-carries-lsp-fallback-order",
            r.returncode == 0
            and "select:LSP" in out
            and "gopls symbols" in out
            and "gopls definition|references" in out
            and "line range" in out
            and "NO LSP" not in out,
            out,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # A claimed spec WITH a state file: the digest path is injected, resolved by
    # lib/state-file.sh, never spelled a second time by the hook.
    work = _deleg_project(spec="spec-fixture.md")
    state = os.path.join(
        _deleg_state_dir(work), f"session-state-fixture-{_DELEG_SID}.md"
    )
    try:
        with open(state, "w", encoding="utf-8") as fh:
            fh.write("# state\n\nPhase 1 digest.\n")
        r = _subagent_context(work)
        results.check(
            "subagent-context-carries-spec-digest",
            r.returncode == 0 and f"session-state-fixture-{_DELEG_SID}.md" in r.stdout,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)

    # No state file: silence, not an empty path. An injected "Digest: " with
    # nothing after it sends the agent looking for a file that does not exist.
    work = _deleg_project(spec="spec-fixture.md")
    try:
        r = _subagent_context(work)
        results.check(
            "subagent-context-no-digest-when-absent",
            "Digest of that spec" not in r.stdout and r.returncode == 0,
            r.stdout,
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_delegation_reminder(results: Results) -> None:
    """ai/rules/planning.md: the harness guard "Do not call the AgentTool
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
        "planning.md" in r.stdout,
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


# VALIDATES: a restricted fork selects the transcript named by the canonical
# session resolver, even when another transcript has the newest mtime.
# PREVENTS: an off-model neighboring session answering the review-model gate for
# the fork. A human invocation with no fork keeps the newest-file fallback.
def _run_fork_transcript_selection(results: Results) -> None:
    work = _deleg_project(spec=None)
    home = tempfile.mkdtemp(prefix="ze-model-home-", dir=_fixture_root())
    try:
        env = _fork_payload_env(work)
        env["HOME"] = home
        probe = (
            "import importlib.util,os,sys;"
            "sys.path.insert(0,sys.argv[1]);"
            "import running_model as rm;"
            "spec=importlib.util.spec_from_file_location('fixture_sid',sys.argv[2]);"
            "sidmod=importlib.util.module_from_spec(spec);"
            "spec.loader.exec_module(sidmod);"
            "sid=sidmod.session_id();"
            "tdir=rm.transcript_dir();"
            "os.makedirs(tdir,exist_ok=True);"
            "canonical=os.path.join(tdir,sid+'.jsonl');"
            "neighbor=os.path.join(tdir,'newest-neighbor.jsonl');"
            'open(canonical,\'w\').write(\'{"message":{"model":"claude-sonnet-5"}}\\n\');'
            'open(neighbor,\'w\').write(\'{"message":{"model":"claude-opus-5"}}\\n\');'
            "os.utime(canonical,(1,1));"
            "os.utime(neighbor,None);"
            "print(sid);"
            "print(canonical);"
            "print(rm.transcript_path());"
            "os.environ.pop('CLAUDE_CODE_FORK_SUBAGENT',None);"
            "print(rm.transcript_path())"
        )
        proc = subprocess.run(
            [
                sys.executable,
                "-c",
                probe,
                DEV,
                os.path.join(work, ".claude", "hooks", "lib", "session_id.py"),
            ],
            capture_output=True,
            text=True,
            env=env,
            timeout=60,
        )
        lines = proc.stdout.splitlines()
        sid, canonical, selected, human = lines if len(lines) == 4 else ("", "", "", "")
        results.check(
            "review-model-fork-transcript-uses-canonical-id-not-mtime",
            proc.returncode == 0
            and sid
            and os.path.basename(canonical) == f"{sid}.jsonl"
            and selected == canonical,
            f"rc={proc.returncode} out={proc.stdout!r} err={proc.stderr!r}",
        )
        results.check(
            "review-model-human-transcript-keeps-mtime-fallback",
            human.endswith("/newest-neighbor.jsonl"),
            f"out={proc.stdout!r}",
        )
    finally:
        shutil.rmtree(home, ignore_errors=True)
        shutil.rmtree(work, ignore_errors=True)


# VALIDATES: the model acknowledgement lookup for a restricted fork uses the
# same canonical identity as the shared session resolver.
# PREVENTS: fork acknowledgement checks reading a global or neighboring
# session's decision when no session environment variable is present.
def _run_fork_model_ack_identity(results: Results) -> None:
    work = _deleg_project(spec=None)
    try:
        env = _fork_payload_env(work)
        probe = (
            "import importlib.util,os,sys;"
            "spec=importlib.util.spec_from_file_location('agent_hook',sys.argv[1]);"
            "mod=importlib.util.module_from_spec(spec);"
            "spec.loader.exec_module(mod);"
            "sid=mod._ze_session_id.session_id();"
            "path=os.path.join(os.environ['CLAUDE_PROJECT_DIR'],'tmp','session',"
            "'.model-ack-'+sid);"
            "open(path,'w').write('operator approved fork identity');"
            "print(sid);"
            "print(path);"
            "print(mod._ack_recorded())"
        )
        proc = subprocess.run(
            [
                sys.executable,
                "-c",
                probe,
                os.path.join(HOOKS, "pretool-agent-skill.py"),
            ],
            capture_output=True,
            text=True,
            env=env,
            timeout=60,
        )
        lines = proc.stdout.splitlines()
        sid, path, recorded = lines if len(lines) == 3 else ("", "", "")
        results.check(
            "review-model-ack-uses-canonical-fork-id",
            proc.returncode == 0
            and recorded == "True"
            and os.path.basename(path) == f".model-ack-{sid}"
            and re.fullmatch(r"[A-Za-z0-9_-]+", sid) is not None,
            f"rc={proc.returncode} out={proc.stdout!r} err={proc.stderr!r}",
        )
    finally:
        shutil.rmtree(work, ignore_errors=True)


def run_phase_gates(results: Results) -> None:
    print("phase-gates:")
    """The two gates that stop a session doing the right work the wrong way.

    Both were added on 2026-07-31 after a session ran five hand-written agents
    where skills existed, and did every implementation edit on the review model.
    Neither rule had a gate; both said so in their own text.
    """
    # --- skills over raw agents (pretool-agent-skill.py) ---
    hook = os.path.join(ROOT, ".claude", "hooks", "pretool-agent-skill.py")
    results.check("agent-skill-hook-exists", os.path.isfile(hook), hook)

    def spawn(prompt, tool="Agent"):
        payload = json.dumps({"tool_name": tool, "tool_input": {"prompt": prompt}})
        return subprocess.run(
            [sys.executable, hook], input=payload, capture_output=True, text=True
        )

    r = spawn("Survey the rules and report which ones cause bloat")
    results.check("agent-skill-blocks-research", r.returncode == 2, repr(r.stdout))
    results.check(
        "agent-skill-names-explore", "/ze-explore" in r.stderr, repr(r.stderr)
    )

    r = spawn("Perform an independent critical review of the diff and find bugs")
    results.check("agent-skill-blocks-review", r.returncode == 2, repr(r.stderr))
    # Exact, not substring: "/ze-review" also matches /ze-review-spec and
    # /ze-review-deep, so a mis-route used to pass this fixture.
    results.check(
        "agent-skill-names-review",
        re.search(r"/ze-review\b(?!-)", r.stderr) is not None,
        repr(r.stderr),
    )

    # Naming the skill IS the routing, so it must pass.
    r = spawn("Follow /ze-explore and survey the rules")
    results.check("agent-skill-named-skill-passes", r.returncode == 0, repr(r.stderr))

    # Not every agent task has a covering skill. Over-blocking kills delegation,
    # which is the failure this must not trade for.
    r = spawn("Translate this YANG container into a Go struct")
    results.check(
        "agent-skill-uncovered-task-passes", r.returncode == 0, repr(r.stderr)
    )

    # The first version matched any ze-<word>, so the repo path and any
    # `make ze-precommit-verify` in a prompt switched the whole gate off.
    r = spawn(
        "Review the diff in /Users/x/Code/github.com/ze-software/ze/main and find bugs"
    )
    results.check(
        "agent-skill-repo-path-is-not-a-skill", r.returncode == 2, repr(r.stderr)
    )
    r = spawn(
        "Independently review this change for bugs. Run make ze-precommit-verify first."
    )
    results.check(
        "agent-skill-make-target-is-not-a-skill", r.returncode == 2, repr(r.stderr)
    )
    # A name that is not a skill on disk must not count as routing.
    r = spawn("Follow /ze-nonexistent and review the diff for bugs")
    results.check(
        "agent-skill-unknown-skill-name-blocks", r.returncode == 2, repr(r.stderr)
    )

    # --- the style-guide reminder on a brief that will produce Go.
    # docs/contributing/ze-style.md is read before any code, every session, and a
    # subagent gets it from the brief or not at all. The main thread cannot audit
    # it afterwards: subagent transcripts are under /tmp, which the Bash guard
    # refuses. Measured 2026-08-19: three fix agents carried the instruction under
    # a "Before you finish" heading, where it reads as a closing checklist item.
    # WARN (1), never block: the population is a heuristic over prose.
    r = spawn("Fix the port 0 handling in translate.go and add a test.")
    results.check("agent-style-guide-warns", r.returncode == 1, repr(r.stderr))
    results.check(
        "agent-style-guide-names-the-guide",
        "docs/contributing/ze-style.md" in r.stderr,
        repr(r.stderr)[:160],
    )
    results.check(
        "agent-style-guide-says-precondition",
        "PRECONDITION" in r.stderr and "before you finish" in r.stderr.lower(),
        repr(r.stderr)[:160],
    )

    # Naming the guide is the whole point, so it must silence the reminder.
    r = spawn(
        "Fix the port 0 handling in translate.go. Read docs/contributing/ze-style.md first."
    )
    results.check("agent-style-guide-named-passes", r.returncode == 0, repr(r.stderr))

    # The three shapes that must NOT warn, each one a real brief from this repo.
    for name, prompt in (
        ("docs-only", "Review the wording of docs/guide/firewall.md and report back"),
        (
            "python-only",
            "Fix scripts/dev/check_doc_links.py and go through every caller",
        ),
        (
            "read-only-go",
            "Explain how translate.go handles ranges. Do not change anything",
        ),
    ):
        r = spawn(prompt)
        results.check(
            f"agent-style-guide-quiet-{name}", r.returncode == 0, repr(r.stderr)
        )

    # A guard that wedges delegation on bad input is worse than no guard.
    r = subprocess.run(
        [sys.executable, hook], input="not json", capture_output=True, text=True
    )
    results.check(
        "agent-skill-malformed-input-passes", r.returncode == 0, repr(r.stderr)
    )

    # Present is not wired.
    with open(os.path.join(ROOT, ".claude", "settings.json"), encoding="utf-8") as fh:
        cfg = json.load(fh)
    pre = [
        entry.get("command", "")
        for group in cfg.get("hooks", {}).get("PreToolUse", [])
        if "Agent" in (group.get("matcher") or "")
        for entry in group.get("hooks", [])
    ]
    results.check(
        "agent-skill-registered",
        any(c.endswith("pretool-agent-skill.py") for c in pre),
        repr(pre),
    )

    # The per-turn reminder must NAME the skills, or the model delegates and
    # improvises. That is exactly what happened before this existed.
    reminder = os.path.join(ROOT, ".claude", "hooks", "delegation-reminder.sh")
    r = subprocess.run([reminder], input="{}", capture_output=True, text=True)
    results.check(
        "delegation-reminder-names-skills",
        "/ze-explore" in r.stdout and "/ze-review" in r.stdout,
        repr(r.stdout),
    )

    # --- review runs on Opus 5 (both enforcement points) ---
    #
    # A fake HOME gives a fake ~/.claude/projects/<slug>/, which is how the
    # shared reader finds a transcript. That is the only way to drive these two
    # gates from a model this session is not running.
    with tempfile.TemporaryDirectory() as home:
        # Ask the production code where transcripts live. Recomputing the
        # slug here would reproduce a slug bug in the test and stay green.
        sys.path.insert(0, os.path.join(ROOT, "scripts", "dev"))
        import running_model as _rm

        real_home = os.environ.get("HOME", "")
        os.environ["HOME"] = home
        os.environ["CLAUDE_PROJECT_DIR"] = ROOT
        try:
            tdir = _rm.transcript_dir()
        finally:
            if real_home:
                os.environ["HOME"] = real_home
        os.makedirs(tdir)
        transcript = os.path.join(tdir, "s.jsonl")

        def as_model(model):
            with open(transcript, "w", encoding="utf-8") as fh:
                fh.write(json.dumps({"message": {"model": model}}) + "\n")
            env = dict(os.environ, HOME=home, CLAUDE_PROJECT_DIR=ROOT)
            env.pop("CLAUDE_CODE_SESSION_ID", None)
            env.pop("CLAUDE_CODE_FORK_SUBAGENT", None)
            return env

        env = as_model("claude-sonnet-5")
        payload = json.dumps(
            {
                "tool_name": "Agent",
                "tool_input": {"prompt": "Follow /ze-review over the diff"},
            }
        )
        r = subprocess.run(
            [sys.executable, hook],
            input=payload,
            capture_output=True,
            text=True,
            env=env,
        )
        results.check(
            "review-model-blocks-spawn-off-opus-5", r.returncode == 2, repr(r.stderr)
        )
        results.check(
            "review-model-spawn-names-the-rule",
            "planning.md" in r.stderr,
            repr(r.stderr),
        )

        env5 = as_model("claude-opus-5")
        r = subprocess.run(
            [sys.executable, hook],
            input=payload,
            capture_output=True,
            text=True,
            env=env5,
        )
        results.check(
            "review-model-allows-spawn-on-opus-5", r.returncode == 0, repr(r.stderr)
        )

        # Recording the artifact is the moment a review is CLAIMED.
        gate = os.path.join(ROOT, "scripts", "dev", "review_gate.py")
        env = as_model("claude-sonnet-5")
        cmd = [
            sys.executable,
            gate,
            "record",
            "--spec",
            "fixture-model-probe",
            "--verdict",
            "clean",
            "--files",
            "scripts/dev/running_model.py",
            # Required since the review-rounds cap landed. Omitted, argparse
            # exits 2 for a missing argument -- the SAME code the model block
            # uses -- so the block case passed without the block ever running
            # and the override case could not pass at all.
            "--rounds",
            "1",
        ]
        r = subprocess.run(cmd, capture_output=True, text=True, env=env, cwd=ROOT)
        results.check(
            "review-model-record-blocked-off-opus-5",
            r.returncode == 2 and "planning.md" in (r.stdout + r.stderr),
            repr(r.stdout + r.stderr),
        )
        r = subprocess.run(
            cmd + ["--model-override", "fixture"],
            capture_output=True,
            text=True,
            env=env,
            cwd=ROOT,
        )
        results.check(
            "review-model-record-override-works",
            r.returncode == 0,
            repr(r.stdout + r.stderr),
        )
        for leftover in glob.glob(
            os.path.join(ROOT, "tmp", "review", "fixture-model-probe-*.md")
        ):
            os.remove(leftover)

        # BLOCKER: a session id whose transcript does not exist must answer
        # nothing. Falling back to the newest file returns a NEIGHBOUR session's
        # model, and this project directory holds several live transcripts.
        env_ghost = dict(os.environ, HOME=home, CLAUDE_PROJECT_DIR=ROOT)
        env_ghost["CLAUDE_CODE_SESSION_ID"] = "aaaaaaaa-0000-0000-0000-000000000000"
        probe_src = (
            "import sys;sys.path.insert(0, %r);import running_model as rm;"
            "print(rm.running_model() or 'unknown')"
            % os.path.join(ROOT, "scripts", "dev")
        )
        r = subprocess.run(
            [sys.executable, "-c", probe_src],
            capture_output=True,
            text=True,
            env=env_ghost,
            cwd=ROOT,
        )
        results.check(
            "review-model-missing-sid-file-is-unknown",
            r.stdout.strip() == "unknown",
            repr(r.stdout),
        )
        # An explicitly EMPTY path is not "work it out". The edit gate passes the
        # payload path, and it must not inherit the fallback.
        r = subprocess.run(
            [
                sys.executable,
                "-c",
                probe_src.replace("rm.running_model()", "rm.running_model('')"),
            ],
            capture_output=True,
            text=True,
            env=dict(os.environ, CLAUDE_PROJECT_DIR=ROOT),
            cwd=ROOT,
        )
        results.check(
            "review-model-empty-path-is-unknown",
            r.stdout.strip() == "unknown",
            repr(r.stdout),
        )

        # BLOCKER: mentioning a skill is not asking for a review. Fixing review
        # findings is implementation and belongs on the implementation model.
        env_non_review = as_model("claude-sonnet-5")
        for name, prompt, want in (
            (
                "review-model-allows-fixing-findings",
                "Apply the fixes that /ze-review reported in the artifact",
                0,
            ),
            (
                "review-model-allows-editing-a-skill-file",
                "Update ai/skills/ze-review.md to mention the new gate",
                0,
            ),
            (
                "review-model-blocks-routed-review",
                "Follow /ze-review over the uncommitted diff",
                2,
            ),
        ):
            rr = subprocess.run(
                [sys.executable, hook],
                input=json.dumps(
                    {"tool_name": "Agent", "tool_input": {"prompt": prompt}}
                ),
                capture_output=True,
                text=True,
                env=env_non_review,
            )
            results.check(
                name, rr.returncode == want, f"rc={rr.returncode} {rr.stderr[:120]}"
            )

        # Round 2: the routing regex was wrong in BOTH directions. These are the
        # exact prompts it got wrong. A review prompt does not announce itself in
        # a fixed shape, and mentioning a review is not performing one.
        env_non_review = as_model("claude-sonnet-5")
        for prompt, want, why in (
            ("Please follow /ze-review over the diff", 2, "polite lead-in"),
            ("/ze-review the uncommitted diff", 2, "skill first"),
            ("You are the /ze-review agent for this change", 2, "role form"),
            ("Round 2 of /ze-review over the fixes", 2, "'fixes' is a noun here"),
            ("Apply the fixes that /ze-review reported", 0, "implementation"),
            ("Per /ze-review findings, fix the parser bug", 0, "implementation"),
            ("Update ai/skills/ze-review.md to mention the gate", 0, "editing a file"),
        ):
            rr = subprocess.run(
                [sys.executable, hook],
                input=json.dumps(
                    {"tool_name": "Agent", "tool_input": {"prompt": prompt}}
                ),
                capture_output=True,
                text=True,
                env=env_non_review,
            )
            results.check(
                "review-model-verb-%s" % why.replace(" ", "-").replace("'", ""),
                rr.returncode == want,
                f"rc={rr.returncode} want={want}: {prompt}",
            )

        # An EMPTY ack is not a recorded decision.
        sid_env = dict(env_non_review)
        sid_env["CLAUDE_CODE_SESSION_ID"] = "fixture-ack-probe"
        ackp = os.path.join(ROOT, "tmp", "session", ".model-ack-fixture-ack-probe")
        os.makedirs(os.path.dirname(ackp), exist_ok=True)
        try:
            for body, want, label in (
                ("", 2, "empty"),
                ("operator said proceed", 0, "with-reason"),
            ):
                with open(ackp, "w", encoding="utf-8") as fh:
                    fh.write(body)
                rr = subprocess.run(
                    [sys.executable, hook],
                    input=json.dumps(
                        {
                            "tool_name": "Agent",
                            "transcript_path": transcript,
                            "tool_input": {"prompt": "Follow /ze-review over the diff"},
                        }
                    ),
                    capture_output=True,
                    text=True,
                    env=sid_env,
                )
                results.check(
                    f"review-model-ack-{label}",
                    rr.returncode == want,
                    f"rc={rr.returncode} want={want}",
                )
        finally:
            if os.path.isfile(ackp):
                os.remove(ackp)

        # VALIDATES: a safe top-level hook payload session_id selects the
        # parent session's non-empty review-model acknowledgement.
        # PREVENTS: a restricted fork ignoring the direct hook payload and
        # using a process fallback identity that cannot find the parent marker.
        safe_parent = "fixture-safe-parent"
        payload_ack = os.path.join(ROOT, "tmp", "session", f".model-ack-{safe_parent}")
        payload_env = _fork_payload_env(home, ROOT)
        payload_env["HOME"] = home
        with open(transcript, "w", encoding="utf-8") as fh:
            fh.write(json.dumps({"message": {"model": "claude-sonnet-5"}}) + "\n")
        try:
            with open(payload_ack, "w", encoding="utf-8") as fh:
                fh.write("operator approved parent review identity")
            rr = subprocess.run(
                [sys.executable, hook],
                input=json.dumps(
                    {
                        "tool_name": "Agent",
                        "session_id": safe_parent,
                        "agent_id": "fixture-fork-agent",
                        "transcript_path": transcript,
                        "tool_input": {"prompt": "Follow /ze-review over the diff"},
                    }
                ),
                capture_output=True,
                text=True,
                env=payload_env,
            )
            results.check(
                "review-model-ack-uses-safe-payload-id",
                rr.returncode == 0,
                f"rc={rr.returncode} err={rr.stderr!r}",
            )
        finally:
            if os.path.isfile(payload_ack):
                os.remove(payload_ack)

        # VALIDATES: an acknowledgement id is safe in its raw form. Whitespace
        # suffixes and dot entries do not select a file after normalization.
        # PREVENTS: _ack_recorded stripping an unsafe id into another session's
        # valid id, or accepting "." and ".." as identities.
        for label, bad_sid, misleading_name in (
            ("trailing-space-id", "fixture-ack-probe ", ".model-ack-fixture-ack-probe"),
            (
                "trailing-newline-id",
                "fixture-ack-probe\n",
                ".model-ack-fixture-ack-probe",
            ),
            ("dot-id", ".", ".model-ack-."),
            ("dot-dot-id", "..", ".model-ack-.."),
        ):
            misleading = os.path.join(ROOT, "tmp", "session", misleading_name)
            with open(misleading, "w", encoding="utf-8") as fh:
                fh.write("operator approved another identity")
            invalid_env = dict(env_non_review)
            invalid_env["CLAUDE_CODE_SESSION_ID"] = bad_sid
            try:
                rr = subprocess.run(
                    [sys.executable, hook],
                    input=json.dumps(
                        {
                            "tool_name": "Agent",
                            "transcript_path": transcript,
                            "tool_input": {"prompt": "Follow /ze-review over the diff"},
                        }
                    ),
                    capture_output=True,
                    text=True,
                    env=invalid_env,
                )
                results.check(
                    f"review-model-ack-rejects-{label}",
                    rr.returncode == 2,
                    f"rc={rr.returncode} err={rr.stderr!r}",
                )
            finally:
                if os.path.isfile(misleading):
                    os.remove(misleading)

        # Both gates must read the SAME transcript. The spawn gate threw the
        # payload path away and re-resolved, so the two disagreed.
        #
        # The env must NOT carry the live session id: this session has a real
        # .model-ack file, and it would disarm the gate under test.
        env_no_sid = dict(os.environ, CLAUDE_PROJECT_DIR=ROOT)
        env_no_sid.pop("CLAUDE_CODE_SESSION_ID", None)
        rr = subprocess.run(
            [sys.executable, hook],
            input=json.dumps(
                {
                    "tool_name": "Agent",
                    "transcript_path": transcript,
                    "tool_input": {"prompt": "Follow /ze-review over the diff"},
                }
            ),
            capture_output=True,
            text=True,
            env=env_no_sid,
        )
        results.check(
            "review-model-spawn-uses-payload-transcript",
            rr.returncode == 2,
            f"rc={rr.returncode}; the payload names a 4.8 transcript",
        )

        # Unreadable transcript: stand down, and say so rather than go quiet.
        env_blind = dict(
            os.environ, HOME=os.path.join(home, "nope"), CLAUDE_PROJECT_DIR=ROOT
        )
        env_blind.pop("CLAUDE_CODE_SESSION_ID", None)
        r = subprocess.run(
            [sys.executable, hook],
            input=payload,
            capture_output=True,
            text=True,
            env=env_blind,
        )
        results.check(
            "review-model-unknown-stands-down", r.returncode == 0, repr(r.stderr)
        )
        # A guard that cannot deny must SPEAK (ai/rules/evidence.md).
        results.check(
            "review-model-unknown-says-so",
            "UNCHECKED" in r.stderr,
            repr(r.stderr),
        )

    _run_fork_transcript_selection(results)
    _run_fork_model_ack_identity(results)


# --------------------------------------------------------------------------- #
# rfc-language: a rule directive states its obligation in RFC 2119 language
# --------------------------------------------------------------------------- #


def _find_point(kind: str) -> str | None:
    """A real point file of this `kind`, or None when the corpus holds none.

    DISCOVERED rather than hardcoded. The Edit branch reads the file on disk to
    learn its kind, so it needs a real point, and a hardcoded slug would rot the
    day somebody reclassifies or renames that one point -- silently, because the
    fixture would then be asserting the permit path against a `note`.
    """
    points = os.path.join(ROOT, "ai", "rules", "points")
    for rule in sorted(os.listdir(points)):
        rule_dir = os.path.join(points, rule)
        if not os.path.isdir(rule_dir):
            continue
        for section in sorted(os.listdir(rule_dir)):
            section_dir = os.path.join(rule_dir, section)
            if not os.path.isdir(section_dir):
                continue
            for slug in sorted(os.listdir(section_dir)):
                path = os.path.join(section_dir, slug)
                if not slug.endswith(".md"):
                    continue
                with open(path, encoding="utf-8", errors="replace") as fh:
                    if re.search(r"^kind:[ \t]*%s[ \t]*$" % kind, fh.read(400), re.M):
                        return path
    return None


def _point_body(kind: str, level: str, body: str) -> str:
    return f"---\nkind: {kind}\nlevel: {level}\nstage:\n---\n{body}\n"


def run_rfc_language(results: Results) -> None:
    print("rfc-language:")
    points = os.path.join(ROOT, "ai", "rules", "points")

    # A slug no file uses: c_point_overwrite runs FIRST and refuses a Write over
    # an existing point, so a fixture aimed at this check has to miss that one.
    free = os.path.join(points, "rule-format", "directives", "zz-fixture-probe.md")
    results.check(
        "rfc-language-probe-slug-is-free",
        not os.path.exists(free),
        f"{free} exists, so the Write fixtures below would measure c_point_overwrite",
    )

    # --- Write carries the WHOLE point, so a missing keyword is decidable -----
    code, err = _writeedit(
        free, tool="Write", content=_point_body("directive", "", "- Delete it first.")
    )
    results.check(
        "rfc-language-write-without-keyword-refused",
        code == 2 and "states no RFC 2119 level" in err,
        repr((code, err)),
    )

    code, err = _writeedit(
        free,
        tool="Write",
        content=_point_body("directive", "MUST", "- You MUST delete it first."),
    )
    results.check(
        "rfc-language-write-with-keyword-allowed", code == 0, repr((code, err))
    )

    # Scoped to `kind: directive`. A two-column lookup gains a word and no
    # obligation from being made to say MUST, so note and table are untouched.
    for kind in ("note", "table"):
        code, err = _writeedit(
            free, tool="Write", content=_point_body(kind, "", "- Delete it first.")
        )
        results.check(
            f"rfc-language-write-{kind}-allowed", code == 0, repr((code, err))
        )

    # Quoted text is not stated text. A keyword that appears ONLY inside a code
    # span or a fenced block leaves the point stating nothing, so it is refused
    # -- the mirror of the permit cases further down, and the pair is what shows
    # the strip is applied to one polarity as well as the other.
    code, err = _writeedit(
        free,
        tool="Write",
        content=_point_body("directive", "", "- The error reads `it MUST exist`."),
    )
    results.check(
        "rfc-language-write-keyword-only-in-code-span-refused",
        code == 2 and "states no RFC 2119 level" in err,
        repr((code, err)),
    )

    code, err = _writeedit(
        free,
        tool="Write",
        content=_point_body("directive", "", "- Run it.\n\n```\nit MUST exist\n```"),
    )
    results.check(
        "rfc-language-write-keyword-only-in-fence-refused",
        code == 2 and "states no RFC 2119 level" in err,
        repr((code, err)),
    )

    code, err = _writeedit(
        free,
        tool="Write",
        content=_point_body("directive", "", "- Run it.\n\n~~~~\nit MUST exist\n~~~~~"),
    )
    results.check(
        "rfc-language-write-keyword-only-in-tilde-fence-refused",
        code == 2 and "states no RFC 2119 level" in err,
        repr((code, err)),
    )

    code, err = _writeedit(
        free,
        tool="Write",
        content=_point_body(
            "directive", "", "- The quote follows.\n\n> It MUST exist."
        ),
    )
    results.check(
        "rfc-language-write-keyword-only-in-blockquote-refused",
        code == 2 and "states no RFC 2119 level" in err,
        repr((code, err)),
    )

    # --- Edit carries a FRAGMENT, so only the lowercase modal is decidable ----
    directive = _find_point("directive")
    note = _find_point("note")
    results.check(
        "rfc-language-corpus-has-both-kinds",
        bool(directive) and bool(note),
        f"directive={directive!r} note={note!r}",
    )
    if not (directive and note):
        return

    code, err = _writeedit(directive, tool="Edit", content="- You should also do it.")
    results.check(
        "rfc-language-edit-lowercase-modal-refused",
        code == 2 and "lowercase obligation word" in err,
        repr((code, err)),
    )

    code, err = _writeedit(
        directive, tool="Edit", content="- The error reads `it should exist`."
    )
    results.check(
        "rfc-language-edit-modal-in-code-span-allowed", code == 0, repr((code, err))
    )

    code, err = _writeedit(
        directive,
        tool="Edit",
        content="- Preserve this example.\n\n~~~~\nit should exist\n~~~~~",
    )
    results.check(
        "rfc-language-edit-modal-in-tilde-fence-allowed",
        code == 0,
        repr((code, err)),
    )

    code, err = _writeedit(
        directive,
        tool="Edit",
        content="- Preserve this quotation.\n\n> It should exist.",
    )
    results.check(
        "rfc-language-edit-modal-in-blockquote-allowed",
        code == 0,
        repr((code, err)),
    )

    code, err = _multiedit(
        directive,
        [
            {"old_string": "a", "new_string": "- You MUST act.\n\n~~~~"},
            {"old_string": "b", "new_string": "- You should also report.\n~~~~~"},
        ],
    )
    results.check(
        "rfc-language-multiedit-lowercase-modal-refused",
        code == 2 and "lowercase obligation word" in err,
        repr((code, err)),
    )

    # A fragment legitimately carries no keyword: the one that governs the point
    # can sit in the part the Edit does not touch. Refusing here would make the
    # check unusable for every ordinary wording fix.
    code, err = _writeedit(directive, tool="Edit", content="- Delete it first.")
    results.check(
        "rfc-language-edit-without-keyword-allowed", code == 0, repr((code, err))
    )

    # `must-fix` is a compound, not a modal. Without the trailing-hyphen guard
    # the word boundary after `must` matches and the check refuses real prose.
    code, err = _writeedit(
        directive, tool="Edit", content="- A must-fix defect MUST be fixed."
    )
    results.check(
        "rfc-language-edit-hyphenated-compound-allowed", code == 0, repr((code, err))
    )

    # The kind is read from the FILE for an Edit, not from the fragment, so a
    # note keeps its lowercase prose.
    code, err = _writeedit(note, tool="Edit", content="- You should also do it.")
    results.check("rfc-language-edit-note-allowed", code == 0, repr((code, err)))

    # --- Discrimination: nothing outside a point file is touched --------------
    code, err = _writeedit(
        os.path.join(points, "rule-format", "manifest.md"),
        tool="Edit",
        content="- You should also do it.",
    )
    results.check("rfc-language-manifest-allowed", code == 0, repr((code, err)))

    code, err = _writeedit(
        os.path.join(ROOT, "docs", "contributing", "writing-style.md"),
        tool="Edit",
        content="- You should also do it.",
    )
    results.check("rfc-language-unrelated-doc-allowed", code == 0, repr((code, err)))


def run_raw_job_admission(results: Results) -> None:
    """A heavy job is admitted by `make`; typed raw it is refused (AC-5).

    The parity harness records one exit code per command, and the whole-
    dispatcher code cannot say WHICH replacement the refusal named. The value
    of this guard is the replacement: a refusal that does not name the queued
    command teaches the agent nothing and gets routed around
    (plan/spec-shared-machine-job-admission.md, R-4).

    The positive cases are the point of the section. A guard that refuses
    everything passes every negative test and is useless, so the sanctioned
    path is pinned here first.
    """
    print("raw-job-admission:")
    check = _load_pretool_bash().check_raw_test_invocation
    # Assembled so this file's own text carries no runnable raw invocation: the
    # Bash hook matches command TEXT, and a fixture that spells one cannot be
    # grepped for from a session (ai/rules/commands.md, "The Bash Hook Matches
    # Your Command Text").
    go_test = "go" + " test"
    lint = "golangci" + "-lint"
    py_test = "_test" + ".py"

    def refused(cmd):
        return check(cmd, None)

    # --- the discriminator: the sanctioned path is never refused ------------
    for name, cmd in (
        (
            "test_the_make_target_itself_is_not_refused",
            "make ze-unit-pkg-test PKG=./internal/core/env",
        ),
        ("make-lint-changed-not-refused", "make ze-lint-changed"),
        ("make-functional-suite-not-refused", "make ze-functional-plugin-test"),
        ("make-full-verify-not-refused", "make ze-precommit-verify"),
        # A make target whose ARGUMENT spells the runner. The command word is
        # what decides, or the documented QEMU debug invocation would be lost.
        (
            "make-target-carrying-the-runner-in-an-argument-not-refused",
            "make ze-qemu-debug RUN='bin/ze-test-linux-arm64 bgp parse 91 -v'",
        ),
        # The rule stays auditable from Bash: a banned verb in a search PATTERN
        # is text, not a command.
        (
            "search-pattern-naming-a-raw-command-not-refused",
            f"grep -rn '{go_test}' ai/rules",
        ),
        (
            "git-log-pickaxe-naming-the-linter-not-refused",
            f"git log -S '{lint} run' -- Makefile",
        ),
        # Cheap subcommands read configuration and run no analysis.
        ("golangci-lint-config-verify-not-refused", f"{lint} config verify"),
        # `go build` has its own guard (check_root_build); this check is about
        # heavy jobs, and widening it would double-refuse with a worse message.
        ("go-build-not-this-check-s-business", "go bui" + "ld ./cmd/ze"),
        ("the-daemon-binary-is-not-the-runner", "bin/ze parse ./x"),
        (
            "a-non-test-python-tool-is-not-refused",
            "python3 scripts/dev/hook-fixture-check.py",
        ),
    ):
        r = refused(cmd)
        results.check(name, r is None, repr(r))

    # --- the four raw forms AC-5 names -------------------------------------
    for name, cmd, want in (
        (
            "test_a_raw_go_test_is_refused_and_names_the_make_target",
            f"{go_test} ./internal/component/bgp/...",
            "make ze-unit-pkg-test PKG=./internal/component/bgp/...",
        ),
        # The suggestion carries the packages and the -run filter, so the
        # replacement is runnable rather than a shape to fill in.
        (
            "raw-go-test-replacement-carries-run-filter",
            f"{go_test} -run TestFoo ./internal/core/env",
            "make ze-unit-pkg-test PKG=./internal/core/env RUN=TestFoo",
        ),
        ("raw-golangci-lint-is-refused", f"{lint} run ./...", "make ze-lint-changed"),
        (
            "raw-ze-test-runner-is-refused",
            "bin/ze-test bgp plugin",
            "make ze-functional-plugin-test",
        ),
        (
            "raw-ze-test-runner-with-dot-slash-is-refused",
            "./bin/ze-test editor --all",
            "make ze-functional-editor-test",
        ),
        # The same runner in this session's own directory (mk/session.mk
        # ZE_BIN_DIR) is the same producer.
        (
            "session-directory-runner-is-refused",
            "tmp/session/2026-08-10-abc123/bin/ze-test bgp encode",
            "make ze-functional-encode-test",
        ),
        (
            "raw-python-test-is-refused",
            f"python3 scripts/dev/commit_helper{py_test}",
            "make ze-unit-pkg-test PKG=./scripts/dev RUN=TestPythonUnitTests",
        ),
        # A launcher in front changes who runs it, not what it costs.
        (
            "a-launcher-does-not-hide-the-raw-job",
            f"timeout 300 {go_test} ./internal/core/env",
            "make ze-unit-pkg-test PKG=./internal/core/env",
        ),
        # Judged per statement: a sanctioned command first does not buy the
        # raw one after it.
        (
            "a-raw-job-after-a-make-target-is-still-refused",
            f"make ze-lint && {go_test} ./...",
            "make ze-unit-pkg-test PKG=./...",
        ),
    ):
        r = refused(cmd)
        results.check(
            name,
            r is not None and r[0] == 2 and want in r[1],
            repr(r),
        )

    # --- the escape (R-4): cheap, honest, and visible in the transcript -----
    r = refused(
        f'ZE_ADMIT_RAW="bisecting one 2s case" {go_test} -run TestX ./internal/core/env'
    )
    results.check("a-declared-reason-admits-the-raw-form", r is None, repr(r))

    # An empty reason is no reason. Without this the escape is a bare flag, and
    # the transcript records nothing a reviewer can read.
    r = refused(f"ZE_ADMIT_RAW= {go_test} ./internal/core/env")
    results.check(
        "an-empty-reason-does-not-admit", r is not None and r[0] == 2, repr(r)
    )

    # An unrelated assignment is not the escape.
    r = refused(f"GOFLAGS=-count=1 {go_test} ./internal/core/env")
    results.check(
        "an-unrelated-assignment-does-not-admit",
        r is not None and r[0] == 2,
        repr(r),
    )

    # The wrapper itself IS the queue, so a raw command handed to it is
    # admitted: that is the escape for work no make target expresses.
    r = refused(f"scripts/dev/ze-run.sh gotest {go_test} ./internal/core/env")
    results.check("the-wrapper-admits-the-command-it-queues", r is None, repr(r))

    # Every refusal names the wrapper and the reason escape. A refusal with no
    # way through is what R-4 says produces an invented workaround.
    r = refused(f"{go_test} ./...")
    named = r is not None and "ze-run.sh" in r[1] and "ZE_ADMIT_RAW" in r[1]
    results.check("every-refusal-names-a-way-through", named, repr(r))


def run_governed_doc_edit(results: Results) -> None:
    """check_governed_doc_edit: a shell write to plan/ or ai/rules/ is refused.

    pretool-writeedit.py owns those trees, and settings.json wires it to
    Write|Edit|MultiEdit|NotebookEdit only, so a shell write ran none of its
    checks. Auto mode prefers Bash for file changes, which made the bypass the
    default route rather than an unusual one.

    The pass cases are the load-bearing half. Binding on the write VERB and not
    on the path is what keeps the sanctioned commit path working: commit_helper
    names these paths on every `--file`, and a check that read the path alone
    would refuse the one route the repository requires.
    """
    print("governed-doc-edit:")
    check = _load_pretool_bash().check_governed_doc_edit

    def blocked(cmd):
        return check(cmd, {"dir": "/repo"}) is not None

    for name, cmd in (
        ("redirect", "echo x > plan/spec-foo.md"),
        ("append", "cat rows >> ai/rules/points/commands/manifest.md"),
        ("sed-i", "sed -i 's/a/b/' plan/spec-foo.md"),
        ("tee", "echo x | tee plan/spec-foo.md"),
        ("cp-into", "cp build/x.md plan/spec-foo.md"),
    ):
        results.check(f"governed-{name}-blocks", blocked(cmd), cmd)

    # The shape that produced the finding: a path held in a loop variable, which
    # no literal-path pattern sees.
    heredoc = (
        "python3 - <<'PY'\n"
        "import pathlib\n"
        "pathlib.Path('plan/spec-foo.md').write_text('x')\n"
        "PY"
    )
    results.check("governed-heredoc-literal-blocks", blocked(heredoc), heredoc)
    loop = (
        "python3 - <<'PY'\n"
        "import pathlib\n"
        "for src in ('plan/spec-a.md', 'plan/spec-b.md'):\n"
        "    pathlib.Path(src).write_text('x')\n"
        "PY"
    )
    results.check("governed-heredoc-variable-blocks", blocked(loop), loop)
    # The shape that slipped through: the script is WRITTEN first and the
    # interpreter runs LAST, so an ordered interpreter-path-write pattern sees
    # nothing although every element is a literal. Tier one stays silent on
    # purpose here, because the redirect target is the scratch path.
    write_then_run = (
        "cat > \"$scratch/edit.py\" <<'PY'\n"
        "import pathlib\n"
        "pathlib.Path('plan/spec-foo.md').write_text('x')\n"
        "PY\n"
        'python3 "$scratch/edit.py"'
    )
    results.check(
        "governed-write-script-then-run-blocks",
        blocked(write_then_run),
        write_then_run.replace("\n", "\\n"),
    )
    # The same order with no interpreter anywhere is not tier two's business:
    # the payload is never executed as a script by this command.
    no_interpreter = (
        "cat > \"$scratch/notes.txt\" <<'TXT'\n"
        "pathlib.Path('plan/spec-foo.md').write_text('x')\n"
        "TXT"
    )
    results.check(
        "governed-write-script-without-interpreter-passes",
        not blocked(no_interpreter),
        no_interpreter.replace("\n", "\\n"),
    )

    # Reads. A guard that refused these would stop the tree being read at all.
    for name, cmd in (
        ("grep", "grep -n Status plan/spec-foo.md"),
        ("sed-n", "sed -n '1,40p' plan/spec-foo.md"),
        ("cat", "cat ai/rules/commands.md"),
        ("wc", "wc -l plan/spec-foo.md"),
    ):
        results.check(f"governed-{name}-passes", not blocked(cmd), cmd)

    commit = (
        "scripts/dev/commit_helper.py create --subject x "
        "--file plan/spec-foo.md --file ai/rules/commands.md"
    )
    results.check("governed-commit-helper-passes", not blocked(commit), commit)

    scratch = 'grep Status plan/spec-foo.md > "$dir/out.log"'
    results.check("governed-scratch-redirect-passes", not blocked(scratch), scratch)

    # The escape is the answer to the deliberate over-match: a payload that only
    # READS plan/ and writes elsewhere is refused, and states its reason to land.
    reads = (
        "python3 - <<'PY'\n"
        "import pathlib\n"
        "rows = pathlib.Path('plan/spec-foo.md').read_text()\n"
        "pathlib.Path('out.txt').write_text(rows)\n"
        "PY"
    )
    results.check("governed-read-only-payload-blocks", blocked(reads), reads)
    admitted = 'ZE_ADMIT_GOVERNED_WRITE="reads plan, writes scratch" ' + reads
    results.check("governed-escape-admits", not blocked(admitted), admitted[:60])
    # Every spelling of an empty reason, because the reason is the only thing
    # that makes the bypass auditable. The bare form was the one case this
    # fixture held, and it was the one case the old `["']?\S` test got right:
    # the optional quote ate the opening quote and \S matched the closing one,
    # so `=""` admitted -- and `=""` is the spelling the refusal message prints.
    for name, prefix in (
        ("bare", "ZE_ADMIT_GOVERNED_WRITE= "),
        ("double-quoted", 'ZE_ADMIT_GOVERNED_WRITE="" '),
        ("single-quoted", "ZE_ADMIT_GOVERNED_WRITE='' "),
        ("whitespace-only", 'ZE_ADMIT_GOVERNED_WRITE=" " '),
    ):
        results.check(
            f"governed-escape-needs-a-reason-{name}",
            blocked(prefix + reads),
            f"{name} empty reason must not admit",
        )

    # Two ordinary ways to spell a command that 83d426a0a stopped seeing when it
    # anchored the verbs: a command substitution opens with `(` or a backtick,
    # and a backslash-continued command carries a newline mid-argument. Both
    # blocked before that commit and passed after it, with no fixture to say so.
    for name, cmd in (
        ("subst-paren", "out=$(cp build/x.md plan/spec-foo.md)"),
        ("subst-backtick", "out=`cp build/x.md plan/spec-foo.md`"),
        ("continuation-trailing", "cp build/x.md \\\n plan/spec-foo.md"),
        ("continuation-leading", "cp \\\n build/x.md plan/spec-foo.md"),
        ("continuation-sed-i", "sed -i \\\n 's/a/b/' plan/spec-foo.md"),
    ):
        results.check(f"governed-{name}-blocks", blocked(cmd), cmd.replace("\n", "\\n"))

    # The newline anchor the continuation cases must not cost: a BARE newline
    # still ends the span, so a verb on one line cannot reach the next command's
    # path. Only a backslash before the newline continues the command.
    split = "cp build/x.md /tmp/out.md\ncat plan/spec-foo.md"
    results.check(
        "governed-bare-newline-passes", not blocked(split), split.replace("\n", "\\n")
    )


SECTIONS = {
    "format-alloc": run_format_alloc,
    "design-ref": run_design_ref,
    "rendered-rule": run_rendered_rule,
    "rfc-language": run_rfc_language,
    "validate-spec": run_validate_spec,
    "commit-gate": run_commit_gate,
    "session-id": run_session_id,
    "rfc-test-guard": run_rfc_test_guard,
    "weakened-hatch": run_weakened_hatch,
    "rfc-changed-ledger": run_rfc_changed_ledger,
    "draft-incubator": run_draft_incubator,
    "governed-doc-edit": run_governed_doc_edit,
    "mark-source-read": run_mark_source_read,
    "design-gate": run_design_gate,
    "delegation": run_delegation,
    "session-state": run_session_state,
    "session-state-location": run_session_state_location,
    "subagent-context": run_subagent_context,
    "delegation-reminder": run_delegation_reminder,
    "phase-gates": run_phase_gates,
    "raw-job-admission": run_raw_job_admission,
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
