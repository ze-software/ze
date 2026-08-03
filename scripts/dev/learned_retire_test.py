#!/usr/bin/env python3
"""Tests for the band-limited learned-summary retirement tool.

The band guard is the security-critical surface: `RETIREMENT_CEILING` encodes an
operator permission boundary (`ai/rules/never-destroy-work.md`, granted
2026-08-01 for band 1-400 only), so the tests that matter most are the refusals.
`BandGuardTest` is AC-9 of `plan/spec-knowledge-1-corpus.md`.

Fixtures live under the OS temp dir rather than the project `tmp/`: the
symlink-escape test needs a tree genuinely outside the root under test.
"""

from __future__ import annotations

import contextlib
import io
import json
import os
import shutil
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
from learned_retire import (
    PROTECTED,
    RETIREMENT_CEILING,
    RetireRefused,
    cross_check,
    load_expected,
    main,
    parse_band,
    remove,
    scan_citations,
    select,
    summary_number,
)

REPO_ROOT = Path(__file__).resolve().parents[2]


def write(root: Path, rel: str, body: str) -> None:
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


class RetireTestCase(unittest.TestCase):
    def _root(self) -> Path:
        d = tempfile.mkdtemp(prefix="learned-retire-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        return Path(d)

    def _corpus(self, root: Path, numbers) -> None:
        for n in numbers:
            write(
                root,
                f"plan/learned/{n:03d}-summary-{n}.md",
                f"# {n:03d} -- Summary {n}\n\n## Files\n\nNone recorded.\n",
            )


class BandGuardTest(RetireTestCase):
    """AC-9: the tool refuses any band it was not authorised for."""

    def test_retire_refuses_band_above_400(self):
        # The single most important test in this file. 401 is one past the
        # operator's grant, so it must refuse rather than widen.
        with self.assertRaises(RetireRefused) as ctx:
            parse_band("1-401")
        message = str(ctx.exception)
        self.assertIn("401", message)
        self.assertIn(str(RETIREMENT_CEILING), message)
        self.assertIn("never-destroy-work", message)

    def test_retire_refuses_band_far_above_400(self):
        for spec in ("1-1289", "401-402", "500-600", "1-10000"):
            with self.subTest(spec=spec), self.assertRaises(RetireRefused):
                parse_band(spec)

    def test_band_boundary_400_is_accepted_401_is_not(self):
        # Last valid / first invalid above (spec Boundary Tests table).
        self.assertEqual(parse_band("1-400"), (1, 400))
        self.assertEqual(parse_band("400-400"), (400, 400))
        with self.assertRaises(RetireRefused):
            parse_band("1-401")

    def test_retire_refuses_lower_bound_below_one(self):
        # First invalid below.
        for spec in ("0-400", "0-0"):
            with self.subTest(spec=spec):
                with self.assertRaises(RetireRefused) as ctx:
                    parse_band(spec)
                self.assertIn("below 1", str(ctx.exception))

    def test_retire_refuses_inverted_band(self):
        with self.assertRaises(RetireRefused) as ctx:
            parse_band("400-1")
        self.assertIn("inverted", str(ctx.exception))

    def test_retire_refuses_non_numeric_band(self):
        for spec in ("", "abc", "1", "1-", "-400", "1-4-0", "1..400", "1-40a", None):
            with self.subTest(spec=spec), self.assertRaises(RetireRefused):
                parse_band(spec)

    def test_negative_band_does_not_parse_as_a_range(self):
        # `-5-400` must not be read as (-5, 400): a negative low would sail
        # past the floor check if the regex admitted a sign.
        with self.assertRaises(RetireRefused):
            parse_band("-5-400")

    def test_cli_refuses_the_band_and_exits_nonzero(self):
        root = self._root()
        self._corpus(root, [1, 500])
        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root), "--band", "1-401", "--apply"])
        self.assertNotEqual(code, 0)
        self.assertIn("learned retire refused", buf.getvalue())
        # The refusal happened before any unlink.
        self.assertTrue((root / "plan/learned/001-summary-1.md").is_file())


class DryRunTest(RetireTestCase):
    """The default mode reports and deletes nothing."""

    def test_dry_run_deletes_nothing(self):
        root = self._root()
        self._corpus(root, [1, 200, 400, 401, 900])
        before = sorted(p.name for p in (root / "plan/learned").iterdir())

        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = main(["--repo", str(root), "--band", "1-400"])
        self.assertEqual(code, 0)

        after = sorted(p.name for p in (root / "plan/learned").iterdir())
        self.assertEqual(before, after, "a dry run must not remove a file")
        self.assertIn("would remove 3 summary(ies)", buf.getvalue())
        self.assertIn("nothing was deleted", buf.getvalue())

    def test_explicit_dry_run_flag_also_deletes_nothing(self):
        root = self._root()
        self._corpus(root, [7])
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(
                main(["--repo", str(root), "--band", "1-400", "--dry-run"]), 0
            )
        self.assertTrue((root / "plan/learned/007-summary-7.md").is_file())

    def test_apply_and_dry_run_together_are_refused(self):
        root = self._root()
        self._corpus(root, [7])
        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(
                ["--repo", str(root), "--band", "1-400", "--apply", "--dry-run"]
            )
        self.assertEqual(code, 2)
        self.assertTrue((root / "plan/learned/007-summary-7.md").is_file())


class SelectionTest(RetireTestCase):
    """Only numbered summaries inside the band are ever candidates."""

    def test_selects_only_the_band(self):
        root = self._root()
        self._corpus(root, [1, 399, 400, 401, 1284])
        names = [p.name for p in select(root, 1, 400)]
        self.assertEqual(
            names,
            ["001-summary-1.md", "399-summary-399.md", "400-summary-400.md"],
        )

    def test_named_meta_summaries_are_never_selected(self):
        root = self._root()
        self._corpus(root, [1, 2])
        for name in PROTECTED:
            write(root, f"plan/learned/{name}", f"# {name}\n\nprose\n")
        selected = {p.name for p in select(root, 1, 400)}
        for name in PROTECTED:
            self.assertNotIn(name, selected, f"{name} must never be retired")
        self.assertEqual(len(selected), 2)

    def test_design_history_survives_an_apply(self):
        # The end-to-end version of the guard above: DESIGN-HISTORY.md holds the
        # consolidated knowledge and losing it would defeat the whole spec.
        root = self._root()
        self._corpus(root, [1, 400])
        write(root, "plan/learned/DESIGN-HISTORY.md", "# history\n")
        write(root, "plan/learned/METHODOLOGY.md", "# methodology\n")
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(
                main(["--repo", str(root), "--band", "1-400", "--apply"]), 0
            )
        self.assertTrue((root / "plan/learned/DESIGN-HISTORY.md").is_file())
        self.assertTrue((root / "plan/learned/METHODOLOGY.md").is_file())
        self.assertFalse((root / "plan/learned/001-summary-1.md").exists())

    def test_defect_records_are_out_of_reach(self):
        # `plan/known-failures/` and `plan/deferrals/` hold DEFECT records,
        # which `ai/rules/completion.md` forbids pruning. They are a different
        # directory, so no band can select them.
        root = self._root()
        self._corpus(root, [1])
        write(root, "plan/known-failures/001-flake.md", "# a red test\n")
        write(root, "plan/deferrals/001-thing.md", "# a deferral\n")
        with contextlib.redirect_stdout(io.StringIO()):
            main(["--repo", str(root), "--band", "1-400", "--apply"])
        self.assertTrue((root / "plan/known-failures/001-flake.md").is_file())
        self.assertTrue((root / "plan/deferrals/001-thing.md").is_file())

    def test_missing_learned_directory_is_refused(self):
        root = self._root()
        with self.assertRaises(RetireRefused):
            select(root, 1, 400)


class FailClosedTest(RetireTestCase):
    """An unparseable filename is a refusal, never a silent skip."""

    def test_unparseable_filename_refuses(self):
        for name in ("12abc.md", "007.md", "0-.md", "42-no-extension"):
            with self.subTest(name=name), self.assertRaises(RetireRefused):
                summary_number(name)

    def test_unparseable_file_in_the_directory_stops_the_whole_run(self):
        root = self._root()
        self._corpus(root, [1, 2])
        write(root, "plan/learned/12abc.md", "# unparseable\n")
        with self.assertRaises(RetireRefused) as ctx:
            select(root, 1, 400)
        self.assertIn("12abc.md", str(ctx.exception))
        self.assertIn("Nothing was deleted", str(ctx.exception))

    def test_unparseable_file_prevents_any_deletion_through_the_cli(self):
        root = self._root()
        self._corpus(root, [1, 2])
        write(root, "plan/learned/12abc.md", "# unparseable\n")
        buf = io.StringIO()
        with contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root), "--band", "1-400", "--apply"])
        self.assertEqual(code, 2)
        self.assertTrue((root / "plan/learned/001-summary-1.md").is_file())
        self.assertTrue((root / "plan/learned/002-summary-2.md").is_file())


class PathTraversalTest(RetireTestCase):
    """Spec Security Review: never act on a path that escapes plan/learned/."""

    def test_symlinked_summary_escaping_the_tree_is_refused(self):
        outside = tempfile.mkdtemp(prefix="learned-retire-outside-")
        self.addCleanup(lambda: shutil.rmtree(outside, ignore_errors=True))
        victim = Path(outside, "precious.md")
        victim.write_text("# not ours to delete\n", encoding="utf-8")

        root = self._root()
        self._corpus(root, [1])
        try:
            os.symlink(victim, root / "plan/learned/002-escape.md")
        except (OSError, NotImplementedError):  # pragma: no cover - platform guard
            self.skipTest("symlinks unavailable on this platform")

        with self.assertRaises(RetireRefused) as ctx:
            select(root, 1, 400)
        self.assertIn("outside plan/learned/", str(ctx.exception))
        self.assertTrue(victim.is_file(), "the file outside the tree must survive")

    def test_symlinked_learned_directory_is_refused(self):
        outside = tempfile.mkdtemp(prefix="learned-retire-outside-dir-")
        self.addCleanup(lambda: shutil.rmtree(outside, ignore_errors=True))
        Path(outside, "001-precious.md").write_text("# theirs\n", encoding="utf-8")

        root = self._root()
        (root / "plan").mkdir(parents=True, exist_ok=True)
        try:
            os.symlink(outside, root / "plan" / "learned")
        except (OSError, NotImplementedError):  # pragma: no cover - platform guard
            self.skipTest("symlinks unavailable on this platform")

        with self.assertRaises(RetireRefused) as ctx:
            select(root, 1, 400)
        self.assertIn("symlink escape", str(ctx.exception))
        self.assertTrue(Path(outside, "001-precious.md").is_file())


class ApplyTest(RetireTestCase):
    """--apply removes the band and reports the names for the commit script."""

    def test_apply_removes_the_band_and_leaves_the_rest(self):
        root = self._root()
        self._corpus(root, [1, 400, 401, 900])
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = main(["--repo", str(root), "--band", "1-400", "--apply", "--json"])
        self.assertEqual(code, 0)
        payload = json.loads(buf.getvalue())
        self.assertEqual(
            payload["removed"],
            ["plan/learned/001-summary-1.md", "plan/learned/400-summary-400.md"],
        )
        self.assertTrue(payload["applied"])
        self.assertEqual(payload["ceiling"], RETIREMENT_CEILING)
        self.assertFalse((root / "plan/learned/001-summary-1.md").exists())
        self.assertTrue((root / "plan/learned/401-summary-401.md").is_file())
        self.assertTrue((root / "plan/learned/900-summary-900.md").is_file())

    def test_no_git_rm_shellout(self):
        # The commit runs through commit_helper.py --remove
        # (`ai/rules/git-safety.md`), so this tool must not invoke git itself.
        source = (REPO_ROOT / "scripts" / "dev" / "learned_retire.py").read_text(
            encoding="utf-8"
        )
        for banned in ("subprocess", "os.system", "git rm"):
            self.assertNotIn(
                banned,
                source.replace("`git rm`", ""),
                f"{banned} must not appear: removal is a working-tree unlink",
            )


class CitationScanTest(RetireTestCase):
    """The dangling-citation surface is reported, never repointed."""

    def test_reports_citations_that_would_dangle(self):
        root = self._root()
        self._corpus(root, [59, 500])
        write(root, "docs/architecture/x.md", "See `plan/learned/59` for why.\n")
        write(root, "docs/architecture/y.md", "See `plan/learned/500` for why.\n")

        rows = scan_citations(
            root, set(range(1, 401)), {"plan/learned/059-summary-59.md"}
        )
        by_path = {r["path"]: r for r in rows}
        self.assertIn("docs/architecture/x.md", by_path)
        self.assertEqual(by_path["docs/architecture/x.md"]["numbers"], [59])
        self.assertNotIn(
            "docs/architecture/y.md",
            by_path,
            "a citation outside the band is not dangling",
        )
        self.assertNotIn(
            "plan/learned/059-summary-59.md",
            by_path,
            "a file being removed cannot dangle against itself",
        )

    def test_tool_source_is_classified_apart_from_documents(self):
        # A tool cites summary numbers as fixtures it writes into a throwaway
        # repo, or as docstring examples. Neither dangles. Classified, not
        # dropped: the row is still reported under its own kind.
        root = self._root()
        self._corpus(root, [59])
        write(root, "scripts/dev/some_test.py", 'write("plan/learned/059-x.md")\n')
        write(root, "docs/architecture/real.md", "See `plan/learned/59`.\n")
        rows = scan_citations(root, set(range(1, 401)), set())
        kinds = {r["path"]: r["kind"] for r in rows}
        self.assertEqual(kinds["scripts/dev/some_test.py"], "tool-source")
        self.assertEqual(kinds["docs/architecture/real.md"], "document")
        # Only the document is a repoint target, so only it drives the check.
        checks = cross_check(rows, {"docs/architecture/real.md": [59]})
        self.assertEqual(checks["missing-from-list"], [])

    def test_generated_files_are_flagged_not_listed_as_repoint_targets(self):
        root = self._root()
        self._corpus(root, [59])
        write(
            root,
            "ai/INDEX-GEN.md",
            "<!-- GENERATED by scripts/dev/x.py -- do not edit -->\n"
            "`plan/learned/59`\n",
        )
        write(root, "ai/hand.md", "`plan/learned/59`\n")
        rows = scan_citations(root, set(range(1, 401)), set())
        kinds = {r["path"]: r["kind"] for r in rows}
        self.assertEqual(kinds["ai/INDEX-GEN.md"], "generated")
        self.assertEqual(kinds["ai/hand.md"], "document")

    def test_cross_check_names_both_directions(self):
        rows = [
            {"path": "a.md", "numbers": [1], "kind": "document"},
            {"path": "b.md", "numbers": [2], "kind": "document"},
            {"path": "gen.md", "numbers": [3], "kind": "generated"},
            {"path": "scripts/t.py", "numbers": [4], "kind": "tool-source"},
        ]
        checks = cross_check(rows, {"a.md": [1], "c.md": [9]})
        self.assertEqual(checks["missing-from-list"], ["b.md"])
        self.assertEqual(checks["stale-list-entries"], ["c.md"])
        self.assertEqual(checks["numbers-differ"], [])
        self.assertNotIn("gen.md", checks["missing-from-list"])
        self.assertNotIn("scripts/t.py", checks["missing-from-list"])

    def test_cross_check_catches_a_changed_number_set(self):
        rows = [{"path": "a.md", "numbers": [1, 2], "kind": "document"}]
        self.assertEqual(cross_check(rows, {"a.md": [1]})["numbers-differ"], ["a.md"])

    def test_expect_list_parse_failure_is_refused(self):
        root = self._root()
        write(root, "list.tsv", "no-tab-here\n")
        with self.assertRaises(RetireRefused):
            load_expected(root / "list.tsv")

    def test_cli_exits_nonzero_when_the_cross_check_disagrees(self):
        root = self._root()
        self._corpus(root, [59])
        write(root, "docs/x.md", "`plan/learned/59`\n")
        write(root, "list.tsv", "docs/other.md\t[59]\n")
        out, err = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            code = main(
                [
                    "--repo",
                    str(root),
                    "--band",
                    "1-400",
                    "--expect",
                    str(root / "list.tsv"),
                ]
            )
        self.assertEqual(code, 1)
        self.assertIn("DISAGREES", err.getvalue())
        # Still a dry run: disagreement must not have deleted anything.
        self.assertTrue((root / "plan/learned/059-summary-59.md").is_file())


class CeilingIsModuleWideTest(RetireTestCase):
    """AC-9's other half: the ceiling is not a property of `parse_band` alone.

    `parse_band` guards the CLI string. `select` and `remove` take integers and
    paths, so a caller that builds either itself once reached straight past 400,
    and `remove` unlinked whatever list it was handed."""

    def test_select_refuses_a_band_above_the_ceiling(self):
        root = self._root()
        self._corpus(root, [1, 500])
        with self.assertRaises(RetireRefused) as ctx:
            select(root, 1, 500)  # bypasses parse_band entirely
        self.assertIn("exceeds the retirement ceiling", str(ctx.exception))
        self.assertTrue((root / "plan/learned/500-summary-500.md").is_file())

    def test_select_refuses_a_lower_bound_below_the_floor(self):
        root = self._root()
        self._corpus(root, [1])
        with self.assertRaises(RetireRefused) as ctx:
            select(root, 0, 400)
        self.assertIn("below 1", str(ctx.exception))

    def test_remove_refuses_a_summary_above_the_ceiling(self):
        root = self._root()
        self._corpus(root, [401])
        victim = root / "plan/learned/401-summary-401.md"
        with self.assertRaises(RetireRefused) as ctx:
            remove(root, [victim])
        self.assertIn("exceeds the retirement ceiling", str(ctx.exception))
        self.assertTrue(victim.is_file(), "the file must survive the refusal")

    def test_remove_refuses_a_path_outside_the_learned_directory(self):
        # `plan/known-failures/` holds DEFECT records (`ai/rules/completion.md`).
        root = self._root()
        write(root, "plan/known-failures/001-flake.md", "# a red test\n")
        victim = root / "plan/known-failures/001-flake.md"
        with self.assertRaises(RetireRefused):
            remove(root, [victim])
        self.assertTrue(victim.is_file())

    def test_remove_refuses_a_path_outside_the_repository(self):
        root = self._root()
        outside = Path(tempfile.mkdtemp(prefix="learned-retire-outside-"))
        self.addCleanup(lambda: shutil.rmtree(outside, ignore_errors=True))
        victim = outside / "001-precious.md"
        victim.write_text("# theirs\n", encoding="utf-8")
        with self.assertRaises(RetireRefused):
            remove(root, [victim])
        self.assertTrue(victim.is_file())

    def test_remove_checks_every_path_before_unlinking_any(self):
        # The in-band file is FIRST, so a per-path check-then-unlink loop would
        # already have deleted it by the time it reached the refusal.
        root = self._root()
        self._corpus(root, [1, 401])
        keep = root / "plan/learned/001-summary-1.md"
        with self.assertRaises(RetireRefused):
            remove(root, [keep, root / "plan/learned/401-summary-401.md"])
        self.assertTrue(keep.is_file(), "a refusal left a partial deletion behind")


class PartialFailureTest(RetireTestCase):
    """A run that dies mid-unlink still names what it already removed.

    The removed list is the tool's only record: those files are gone from the
    working tree, and `commit_helper.py --remove` needs their names."""

    def test_remove_fills_the_out_list_before_it_raises(self):
        root = self._root()
        self._corpus(root, [1, 2])
        good = root / "plan/learned/001-summary-1.md"
        vanished = root / "plan/learned/002-summary-2.md"
        vanished.unlink()  # gone between selection and unlink

        out: list[str] = []
        with self.assertRaises(OSError):
            remove(root, [good, vanished], out)

        self.assertEqual(out, ["plan/learned/001-summary-1.md"])
        self.assertFalse(good.exists(), "the first unlink did happen")

    def test_the_cli_reports_the_files_it_already_removed(self):
        root = self._root()
        self._corpus(root, [1, 2, 3])

        real_unlink = Path.unlink
        calls: list[Path] = []

        def flaky(self, *args, **kwargs):
            calls.append(self)
            if len(calls) == 2:
                raise OSError("input/output error")
            return real_unlink(self, *args, **kwargs)

        buf = io.StringIO()
        with mock.patch.object(Path, "unlink", flaky), contextlib.redirect_stderr(buf):
            code = main(["--repo", str(root), "--band", "1-400", "--apply"])

        err = buf.getvalue()
        self.assertEqual(code, 1)
        # Previously: "Removed 0 file(s)", and no list at all.
        self.assertIn("Removed 1 file(s)", err)
        self.assertIn("removed: plan/learned/001-summary-1.md", err)


class OrderingTest(RetireTestCase):
    """VALIDATES: the selection is ordered by NUMBER, not by filename.

    Replaces a real-corpus test that self-skipped once band 1-400 was retired
    and so could never run again. A permanently-skipped test reads as coverage
    and supplies none."""

    def test_selection_is_ordered_numerically_not_lexicographically(self):
        # Unpadded names: by filename "10-" sorts before "9-".
        root = self._root()
        for n in (9, 10, 100, 2):
            write(root, f"plan/learned/{n}-summary-{n}.md", f"# {n} -- S{n}\n")
        numbers = [int(p.name.split("-")[0]) for p in select(root, 1, 400)]
        self.assertEqual(numbers, [2, 9, 10, 100])

    def test_selection_stays_inside_the_band_and_the_ceiling(self):
        root = self._root()
        self._corpus(root, [1, 200, 400, 401, 900])
        numbers = [int(p.name.split("-")[0]) for p in select(root, 1, 400)]
        self.assertEqual(numbers, [1, 200, 400])
        self.assertLessEqual(max(numbers), RETIREMENT_CEILING)


class RealCorpusTest(unittest.TestCase):
    """The band the operator authorised matches the tree it ships with."""

    def test_real_corpus_parses_without_refusal(self):
        # Fail-closed's other half: every filename in the real directory must
        # parse, or the tool would refuse the whole retirement run.
        select(REPO_ROOT, 1, RETIREMENT_CEILING)


if __name__ == "__main__":
    unittest.main()
