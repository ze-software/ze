#!/usr/bin/env python3
"""Wiring tests for the learned-summary staleness gate.

These are the Step 1 (wiring) rows of the Wiring Test table in
`plan/spec-knowledge-1-corpus.md`. They are deliberately split:

  * `test_target_declared_in_doc_test` proves the ENTRY POINT exists -- the make
    target is declared and `make ze-verify` reaches it through `ze-doc-test`.
    This passes now.
  * `test_gate_reports_dead_path` proves the FEATURE behind that entry point.
    `scripts/dev/learned_staleness.py` `check` is still a stub, so this is RED
    until Step 5 implements it. That red is the point of the wiring phase: the
    target is proven reachable before the logic it will reach exists.
"""

from __future__ import annotations

import contextlib
import datetime
import io
import json
import os
import re
import shutil
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from learned_staleness import (
    DRAIN_REL,
    DrainError,
    check,
    check_drain,
    drain_anchor,
    load_baseline,
    main,
    parse_drain_budget,
    required_drain,
    summary_files,
    write_baseline,
)

REPO_ROOT = Path(__file__).resolve().parents[2]
INVENTORY_MK = REPO_ROOT / "mk" / "inventory.mk"
TARGET = "ze-learned-staleness"


def write(root: Path, rel: str, body: str) -> None:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


def recipe_of(text: str, target: str) -> str:
    """The recipe body of one make target: every line after `<target>:` up to
    the first line that is neither a tab-indented command nor blank."""
    out: list[str] = []
    inside = False
    for line in text.splitlines():
        if line.startswith(target + ":"):
            inside = True
            continue
        if inside:
            if line.startswith("\t") or not line.strip():
                out.append(line)
                continue
            break
    return "\n".join(out)


class TargetWiringTest(unittest.TestCase):
    """`make ze-verify` -> the gate declared in mk/inventory.mk ze-doc-test."""

    def setUp(self) -> None:
        self.mk = INVENTORY_MK.read_text(encoding="utf-8")

    def test_target_declared_in_doc_test(self):
        # Declared .PHONY, so a same-named file could never shadow it.
        phony = "\n".join(
            line for line in self.mk.splitlines() if line.startswith(".PHONY:")
        )
        self.assertIn(TARGET, phony, ".PHONY does not declare " + TARGET)

        # Has its own target block, so it is runnable on its own (AC-3's entry).
        self.assertRegex(
            self.mk,
            r"(?m)^" + re.escape(TARGET) + r":",
            "no `" + TARGET + ":` target block in mk/inventory.mk",
        )

        # AC-5: reached from ze-verify and ze-verify-changed, both of which run
        # ze-doc-test (scripts/status/verify_run.go stagesForMode).
        doc_test = recipe_of(self.mk, "ze-doc-test")
        self.assertIn(
            "learned_staleness.py",
            doc_test,
            "ze-doc-test does not invoke the staleness checker, so the gate "
            "never runs inside make ze-verify",
        )
        # `|| FAIL=1` is what makes a red gate fail the whole doc-test stage
        # rather than being swallowed by the next command in the chain.
        staleness_line = next(
            line for line in doc_test.splitlines() if "learned_staleness.py" in line
        )
        self.assertIn("|| FAIL=1", staleness_line)

    def test_checker_script_exists(self):
        self.assertTrue(
            (REPO_ROOT / "scripts" / "dev" / "learned_staleness.py").is_file()
        )


class DeadPathTest(unittest.TestCase):
    """AC-3: a summary citing a path that does not exist is named.

    RED until Step 5. The stub `check` returns no findings.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="learned-staleness-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        return Path(d)

    def test_gate_reports_dead_path(self):
        root = self._root()
        write(
            root,
            "plan/learned/001-foo.md",
            "# 001 -- Foo\n"
            "\n"
            "## Files\n"
            "\n"
            "- `internal/component/bgp/gone.go` - deleted when the reactor moved\n"
            "- `internal/component/bgp/here.go` - still present\n",
        )
        write(root, "internal/component/bgp/here.go", "package bgp\n")

        findings = check(root)
        self.assertTrue(
            findings, "expected a finding naming the dead path, got none (stub?)"
        )
        dead = [f for f in findings if "gone.go" in f["token"]]
        self.assertEqual(len(dead), 1, findings)
        self.assertEqual(dead[0]["summary"], "plan/learned/001-foo.md")
        self.assertEqual(dead[0]["line"], 5)
        self.assertNotIn(
            "here.go",
            " ".join(f["token"] for f in findings),
            "a path that exists must not be reported",
        )

    def test_summary_files_enumerates_numbered_summaries_only(self):
        # Not the gate itself, but the enumeration the gate walks: README and
        # METHODOLOGY carry no `## Files` section and are not summaries.
        root = self._root()
        write(root, "plan/learned/001-foo.md", "# 001 -- Foo\n")
        write(root, "plan/learned/1284-bar.md", "# 1284 -- Bar\n")
        write(root, "plan/learned/README.md", "# readme\n")
        write(root, "plan/learned/METHODOLOGY.md", "# methodology\n")
        self.assertEqual(
            [p.name for p in summary_files(root)], ["001-foo.md", "1284-bar.md"]
        )


class GateTestCase(unittest.TestCase):
    """A throwaway repository root the gate can be pointed at.

    Fixtures live under the OS temp dir rather than the project `tmp/`: the
    symlink-escape test needs a second tree genuinely outside the root under
    test, and a fixture inside the repo would be walked by the real-corpus test.
    """

    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="learned-staleness-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        root = Path(d)
        # A root the gate JUDGES carries the drain policy, because an absent one
        # fails closed: "no schedule authored" must never be reachable by
        # deleting the file once a rate is armed (`parse_drain_budget`). The
        # fixture ships the inert rate, so these tests judge the ceiling only.
        write(root, DRAIN_REL, "start 2026-08-07\nrate 0\n")
        return root

    def findings(self, root: Path) -> list[dict]:
        return check(root)


class DeadCitationTest(GateTestCase):
    """A `plan/learned/NNN` citation naming a retired summary is reported."""

    def test_gate_reports_dead_citation(self):
        root = self._root()
        write(
            root,
            "plan/learned/500-alive.md",
            "# 500 -- Alive\n\n## Files\n\nNone recorded.\n",
        )
        write(
            root,
            "plan/learned/501-citer.md",
            "# 501 -- Citer\n"
            "\n"
            "## Decisions\n"
            "\n"
            "- Supersedes `plan/learned/500`, keeps `plan/learned/017`\n"
            "\n"
            "## Files\n"
            "\n"
            "None recorded.\n",
        )

        findings = self.findings(root)
        dead = [f for f in findings if f["token"] == "plan/learned/017"]
        self.assertEqual(len(dead), 1, findings)
        self.assertEqual(dead[0]["summary"], "plan/learned/501-citer.md")
        self.assertEqual(dead[0]["line"], 5)
        self.assertIn("no longer exists", dead[0]["problem"])
        self.assertNotIn(
            "plan/learned/500",
            [f["token"] for f in findings],
            "a citation whose summary survives must not be reported",
        )

    def test_zero_padded_and_bare_numbers_resolve_the_same_summary(self):
        # The corpus writes both `plan/learned/59` and `plan/learned/059`.
        root = self._root()
        write(
            root,
            "plan/learned/059-pool.md",
            "# 059 -- Pool\n\n## Files\n\nNone recorded.\n",
        )
        write(
            root,
            "plan/learned/060-citer.md",
            "# 060 -- Citer\n\n## Context\n\nSee `plan/learned/59`.\n"
            "\n## Files\n\nNone recorded.\n",
        )
        self.assertEqual(self.findings(root), [])


class UnreadableSummaryTest(GateTestCase):
    """Fail closed: a summary the gate cannot read is a finding, never a skip.

    An empty finding list must mean "every summary was read and every reference
    resolved", never "nothing could be read"
    (`ai/rules/evidence.md`).
    """

    def test_gate_skips_missing_files_section(self):
        # Named for the spec's TDD table row. The BEHAVIOUR is the opposite of
        # "skips": a summary with no `## Files` section is REPORTED, because a
        # silent skip is exactly the vacuous pass this gate exists to prevent.
        root = self._root()
        write(
            root,
            "plan/learned/002-no-files.md",
            "# 002 -- No Files\n\n## Context\n\nA summary that never listed files.\n",
        )

        findings = self.findings(root)
        self.assertEqual(len(findings), 1, findings)
        self.assertEqual(findings[0]["summary"], "plan/learned/002-no-files.md")
        self.assertEqual(findings[0]["token"], "## Files")
        self.assertIn("no `## Files` section", findings[0]["problem"])

    def test_unparseable_summary_is_reported(self):
        root = self._root()
        (root / "plan" / "learned").mkdir(parents=True, exist_ok=True)
        # Invalid UTF-8: read_text raises, and the gate must say so.
        (root / "plan" / "learned" / "003-binary.md").write_bytes(b"\xff\xfe\x00bad")

        findings = self.findings(root)
        self.assertEqual(len(findings), 1, findings)
        self.assertIn("could not be read", findings[0]["problem"])


class QualifiedSectionTest(GateTestCase):
    """`## Files <qualifier>` sections are read, not skipped.

    Three summaries (677, 678, 817) keep a second `## Files Modified` section
    holding 12 paths. The fix belongs in this parser rather than in those three
    files (`ai/rules/completion.md`).
    """

    def test_files_modified_section_is_read(self):
        root = self._root()
        write(
            root,
            "plan/learned/004-two-sections.md",
            "# 004 -- Two Sections\n"
            "\n"
            "## Files\n"
            "\n"
            "- `internal/component/bgp/here.go` - present\n"
            "\n"
            "## Gotchas\n"
            "\n"
            "- prose mentioning `internal/component/bgp/prose-only.go`\n"
            "\n"
            "## Files Modified\n"
            "\n"
            "- `internal/component/bgp/qualified-gone.go` - retired\n",
        )
        write(root, "internal/component/bgp/here.go", "package bgp\n")

        findings = self.findings(root)
        tokens = [f["token"] for f in findings]
        self.assertIn(
            "internal/component/bgp/qualified-gone.go",
            tokens,
            "a `## Files Modified` section must be read; reading only the exact "
            "`## Files` heading silently skips its paths",
        )
        self.assertEqual(len(findings), 1, findings)
        self.assertNotIn(
            "internal/component/bgp/prose-only.go",
            tokens,
            "paths outside a Files section are prose, not a manifest claim",
        )


class PathSafetyTest(GateTestCase):
    """Spec Security Review: never resolve outside the repository root."""

    def test_parent_traversal_is_reported_not_resolved(self):
        root = self._root()
        write(
            root,
            "plan/learned/005-traversal.md",
            "# 005 -- Traversal\n\n## Files\n\n- `../gh-pages/docs/x.md` - sibling\n",
        )
        findings = self.findings(root)
        self.assertEqual(len(findings), 1, findings)
        self.assertEqual(findings[0]["token"], "../gh-pages/docs/x.md")
        self.assertIn("traversal", findings[0]["problem"])

    def test_symlink_escaping_the_tree_is_reported(self):
        outside = tempfile.mkdtemp(prefix="learned-staleness-outside-")
        self.addCleanup(lambda: shutil.rmtree(outside, ignore_errors=True))
        Path(outside, "secret.go").write_text("package x\n", encoding="utf-8")

        root = self._root()
        (root / "internal").mkdir(parents=True, exist_ok=True)
        try:
            os.symlink(outside, root / "internal" / "escape")
        except (OSError, NotImplementedError):  # pragma: no cover - platform guard
            self.skipTest("symlinks unavailable on this platform")
        write(
            root,
            "plan/learned/006-symlink.md",
            "# 006 -- Symlink\n\n## Files\n\n- `internal/escape/secret.go` - escapes\n",
        )

        findings = self.findings(root)
        self.assertEqual(len(findings), 1, findings)
        self.assertIn("outside the repository root", findings[0]["problem"])

    def test_brace_range_is_a_template_not_traversal(self):
        # `test/firewall/{001..011}.ci` is a range, and calling its `..`
        # traversal would send a reader after the wrong bug
        # (`ai/rules/cli.md`).
        root = self._root()
        write(
            root,
            "plan/learned/008-range.md",
            "# 008 -- Range\n\n## Files\n\n- `test/firewall/{001..011}.ci` - a range\n",
        )
        self.assertEqual(self.findings(root), [])

    def test_ellipsis_and_slash_command_are_not_paths(self):
        root = self._root()
        write(
            root,
            "plan/learned/007-tokens.md",
            "# 007 -- Tokens\n"
            "\n"
            "## Files\n"
            "\n"
            "- `.../spf/install.go` - an abbreviation, not a path\n"
            "- `/ze-implement` - a slash command, not a path\n"
            "- `SomeType.Method` - a symbol, not a path\n",
        )
        self.assertEqual(self.findings(root), [])


class BaselineTest(GateTestCase):
    """The ceiling only ever tightens: over it fails, under it rewrites."""

    def _seed(self, root: Path, dead: int) -> None:
        """A summary citing `dead` paths that do not exist."""
        body = "# 010 -- Seed\n\n## Files\n\n"
        for i in range(dead):
            body += f"- `internal/component/bgp/gone{i}.go` - retired\n"
        write(root, "plan/learned/010-seed.md", body)

    def test_baseline_is_shrink_only(self):
        root = self._root()
        self._seed(root, 3)
        write_baseline(root, 3)
        self.assertEqual(load_baseline(root), 3)

        # At the ceiling: green, ceiling untouched.
        self.assertEqual(main(["--repo", str(root)]), 0)
        self.assertEqual(load_baseline(root), 3)

        # Above the ceiling: RED. A regression must fail, never re-bless.
        self._seed(root, 4)
        self.assertEqual(main(["--repo", str(root)]), 1)
        self.assertEqual(
            load_baseline(root), 3, "a failing run must not rewrite the ceiling"
        )

        # Below the ceiling: green, and the ceiling is left ALONE. A plain check
        # used to rewrite it here. This one runs inside ze-doc-test inside
        # ze-verify, so that rewrite made `make ze-verify` modify a tracked file,
        # and in a checkout several sessions share it landed in whichever
        # session committed next. Tightening is now an explicit request.
        self._seed(root, 1)
        before = (root / "plan" / ".learned-staleness-baseline").read_bytes()
        self.assertEqual(main(["--repo", str(root)]), 0)
        self.assertEqual(load_baseline(root), 3, "a check must not write")
        self.assertEqual(
            (root / "plan" / ".learned-staleness-baseline").read_bytes(),
            before,
            "the baseline file must be byte-identical after a plain check",
        )

        # --write-baseline is the request, and it applies the tightening.
        self.assertEqual(main(["--repo", str(root), "--write-baseline"]), 0)
        self.assertEqual(load_baseline(root), 1)

    def test_write_baseline_refuses_to_raise_the_ceiling(self):
        """One command must not be able to re-bless a regression.

        VALIDATES: --write-baseline over a RISEN count refuses, says why, and
        leaves the ceiling where it was.
        PREVENTS: the shrink-only rule living in a docstring that named a caller
        as its enforcer while no caller enforced it. With a ceiling of 0 and one
        dead reference, --write-baseline wrote 1 and exited 0 in silence, which
        is the entire ratchet defeated by the command documented to maintain it.
        """
        root = self._root()
        self._seed(root, 1)
        write_baseline(root, 0)

        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root), "--write-baseline"])
        self.assertEqual(code, 1, "raising the ceiling must fail")
        self.assertEqual(load_baseline(root), 0, "the ceiling must not move")
        self.assertIn("refusing to raise", buf.getvalue())
        self.assertIn("--raise-baseline", buf.getvalue())  # the stated next step

        # A reason is the deliberate override, and it is recorded in the file so
        # a later reader can tell a re-blessing from rot that arrived on its own.
        code = main(
            ["--repo", str(root), "--raise-baseline", "band 401-800 retired today"]
        )
        self.assertEqual(code, 0)
        self.assertEqual(load_baseline(root), 1)
        self.assertIn(
            "band 401-800 retired today",
            (root / "plan" / ".learned-staleness-baseline").read_text(encoding="utf-8"),
        )

    def test_raise_reason_is_not_recorded_over_a_tightening(self):
        """The note records a RAISE, so a shrink must not carry one.

        VALIDATES: --raise-baseline over a count BELOW the ceiling tightens the
        ceiling and writes no `# Raised deliberately` line.
        PREVENTS: the note being keyed on the flag rather than on the write's
        effect. The operator passes the flag when a rise is expected; the count
        decides whether one happened. Stamping the flag's reason onto a shrink
        leaves a later reader a raise record sitting over a tightened ceiling,
        which is the opposite of what the file says occurred.
        """
        root = self._root()
        self._seed(root, 1)
        write_baseline(root, 3)

        code = main(["--repo", str(root), "--raise-baseline", "band 401-800 retired"])
        self.assertEqual(code, 0)
        recorded = (root / "plan" / ".learned-staleness-baseline").read_text(
            encoding="utf-8"
        )
        self.assertEqual(load_baseline(root), 1, "the ceiling must tighten to 1")
        self.assertNotIn("Raised deliberately", recorded)
        self.assertNotIn("band 401-800 retired", recorded)

    def test_raise_baseline_reason_must_say_something(self):
        """An override whose reason is a shrug is not a record."""
        root = self._root()
        self._seed(root, 1)
        write_baseline(root, 0)
        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root), "--raise-baseline", "later"])
        self.assertEqual(code, 1)
        self.assertEqual(load_baseline(root), 0)
        self.assertIn("too short", buf.getvalue())

    def test_unrecorded_baseline_says_so_rather_than_passing_silently(self):
        root = self._root()
        self._seed(root, 2)
        self.assertIsNone(load_baseline(root))
        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root)])
        self.assertEqual(code, 0, "no ceiling means the gate cannot deny")
        self.assertIn("NO BASELINE RECORDED", buf.getvalue())
        self.assertIn("2 dead reference(s)", buf.getvalue())

    def test_corrupt_baseline_reads_as_unrecorded(self):
        root = self._root()
        (root / "plan").mkdir(parents=True, exist_ok=True)
        (root / "plan" / ".learned-staleness-baseline").write_text(
            "# header\nnot-a-number\n", encoding="utf-8"
        )
        self.assertIsNone(
            load_baseline(root),
            "a corrupt ceiling must never be mistaken for a tight one",
        )

    def test_json_report_exits_nonzero_over_the_baseline(self):
        root = self._root()
        self._seed(root, 2)
        write_baseline(root, 1)
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = main(["--repo", str(root), "--json"])
        self.assertEqual(code, 1)
        payload = json.loads(buf.getvalue())
        self.assertEqual(payload["dead"], 2)
        self.assertEqual(payload["baseline"], 1)
        self.assertTrue(payload["implemented"])
        self.assertEqual(len(payload["findings"]), 2)


class ShippedBaselineTest(unittest.TestCase):
    """The committed baseline is armed, and the ratchet only ever tightens."""

    def test_shipped_baseline_is_recorded(self):
        # This replaces test_shipped_baseline_is_unrecorded, which pinned the
        # pre-retirement state and instructed its own replacement once a ceiling
        # was deliberately written. Band 1-400 was retired on 2026-08-01, which
        # removed 848 dead references, and the ceiling was recorded against the
        # tree that remained.
        self.assertTrue((REPO_ROOT / "plan" / ".learned-staleness-baseline").is_file())
        ceiling = load_baseline(REPO_ROOT)
        self.assertIsNotNone(
            ceiling,
            "the ceiling vanished; an unrecorded baseline does not enforce, so "
            "the gate would measure and let any regression through",
        )
        self.assertGreater(ceiling, 0)

    def test_shipped_baseline_is_not_slack(self):
        # A ceiling above the real count is permanent slack: the ratchet only
        # tightens, so it never self-corrects downward on its own. The recorded
        # number must be what the tree actually carries, never a rounded-up
        # cushion someone left room in.
        ceiling = load_baseline(REPO_ROOT)
        dead = len(check(REPO_ROOT))
        self.assertLessEqual(
            dead,
            ceiling,
            "the tree carries more dead references than the ceiling allows",
        )
        self.assertEqual(
            dead,
            ceiling,
            f"ceiling {ceiling} exceeds the real count {dead}: that gap is slack "
            "a regression can grow into unnoticed. Re-run --write-baseline",
        )


class DrainPolicyTest(unittest.TestCase):
    """AC-18: the ceiling gains a schedule, and the schedule ships inert.

    The ceiling is shrink-only, so the only way it ends is by being drained. The
    policy says how fast, in two fields and nothing else. Rate 0 makes it inert:
    the mechanism exists so the answer to "this tax is permanent" is "arm the
    drain", never "delete the gate" (`ai/rules/planning.md`).
    """

    START = datetime.date(2026, 8, 7)

    def _root(self, body: str | None) -> Path:
        d = Path(tempfile.mkdtemp(prefix="learned-drain-"))
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        if body is not None:
            write(d, DRAIN_REL, body)
        return d

    def test_the_shipped_policy_is_inert(self):
        """The committed file parses, ships at rate 0, and demands nothing."""
        self.assertTrue((REPO_ROOT / DRAIN_REL).is_file())
        budget = parse_drain_budget(REPO_ROOT)
        self.assertEqual(budget.rate, 0)
        lines, code = check_drain(REPO_ROOT, len(check(REPO_ROOT)))
        self.assertEqual(code, 0)
        self.assertIn("INERT", lines[0])
        self.assertIn("floor 0", lines[0])

    def test_the_policy_carries_policy_only(self):
        """A count, a per-summary row, or a third key is refused at the parser."""
        for body, why in (
            ("start 2026-08-07\nrate 0\nsummaries 934\n", "a third key"),
            ("start 2026-08-07\nrate 0\nplan/learned/983 1\n", "a per-item row"),
            ("start 2026-08-07\n", "a missing rate"),
            ("rate 0\n", "a missing start"),
            ("start 2026-08-07\nrate 0\nrate 3\n", "a repeated key"),
            ("start 7/8/2026\nrate 0\n", "a date that is not YYYY-MM-DD"),
            ("start 2026-08-07\nrate -1\n", "a negative rate"),
            ("start 2026-08-07\nrate nan\n", "a rate arithmetic cannot compare"),
        ):
            with self.subTest(why=why):
                with self.assertRaises(DrainError):
                    parse_drain_budget(self._root(body))

    def test_a_missing_policy_is_never_a_silent_pass(self):
        """An absent policy does not mean nothing owed (`ai/rules/evidence.md`)."""
        lines, code = check_drain(self._root(None), 0)
        self.assertEqual(code, 1)
        self.assertIn("does NOT mean", lines[0])

    def test_required_drain_counts_whole_calendar_months(self):
        """ceil over whole months, capped at the anchor, clamped anniversary."""
        start = datetime.date(2026, 1, 31)
        # Nothing owed before the first whole month has elapsed.
        self.assertEqual(required_drain(start, 10, 500, datetime.date(2026, 2, 1)), 0)
        # February is short, so the anniversary clamps to the 28th rather than
        # dropping the month. Comparing raw day numbers would lose it.
        self.assertEqual(required_drain(start, 10, 500, datetime.date(2026, 2, 28)), 10)
        self.assertEqual(required_drain(start, 10, 500, datetime.date(2026, 4, 30)), 30)
        # ceil, not floor: the first month already owes one at half a reference.
        self.assertEqual(required_drain(start, 0.5, 500, datetime.date(2026, 2, 28)), 1)
        # Capped at the anchor, which is what retires the schedule.
        self.assertEqual(required_drain(start, 100, 7, datetime.date(2027, 1, 31)), 7)

    def test_an_armed_schedule_reds_only_when_the_corpus_does_not_repair(self):
        """The mutation proof: the rate is the only thing that moves.

        Same tree, same count, same anchor. At rate 0 the schedule passes; armed,
        it fails; and it goes green again when the repairs are made. Nothing else
        in the fixture can be what changed the exit code.
        """
        later = datetime.date(2026, 9, 7)
        inert = self._root("start 2026-08-07\nrate 0\n")
        lines, code = check_drain(inert, 95, anchor=100, today=later)
        self.assertEqual(code, 0, "rate 0 must demand nothing")

        armed = self._root("start 2026-08-07\nrate 10\n")
        lines, code = check_drain(armed, 95, anchor=100, today=later)
        self.assertEqual(code, 1, "5 repaired against a floor of 10 must fail")
        self.assertIn("requires 10 repaired", lines[0])

        lines, code = check_drain(armed, 90, anchor=100, today=later)
        self.assertEqual(code, 0, "10 repaired against a floor of 10 must pass")
        self.assertIn("10 of 100", lines[0])

    def test_an_armed_schedule_with_no_anchor_refuses(self):
        """A schedule that cannot measure progress must not report success."""
        armed = self._root("start 2026-08-07\nrate 10\n")
        lines, code = check_drain(armed, 95, today=datetime.date(2026, 9, 7))
        self.assertEqual(code, 1)
        self.assertIn("cannot be read from git", lines[0])

    def test_the_anchor_is_read_from_history(self):
        """The anchor comes from git, so the policy file holds no count."""
        anchor = drain_anchor(REPO_ROOT, self.START)
        self.assertTrue(
            anchor is None or anchor > 0,
            f"the anchor must be a real ceiling or absent, got {anchor}",
        )
        # A date before this repository existed has no commit to read.
        self.assertIsNone(drain_anchor(REPO_ROOT, datetime.date(1999, 1, 1)))


class RealCorpusTest(unittest.TestCase):
    """The gate reads the tree it ships with, not a fixture."""

    def test_every_summary_is_read(self):
        # Fail-closed's other half: the count of summaries walked must match the
        # corpus on disk, so a parser that silently gave up cannot look green.
        summaries = summary_files(REPO_ROOT)
        self.assertGreater(len(summaries), 100)
        unreadable = [
            f for f in check(REPO_ROOT) if "could not be read" in f["problem"]
        ]
        self.assertEqual(unreadable, [], "some summary in the tree cannot be parsed")


if __name__ == "__main__":
    unittest.main()
