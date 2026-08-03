#!/usr/bin/env python3
"""Tests for deferral_orphans.py.

The script exists because three hand-counts of the same measurement were wrong,
and `ai/rules/planning.md` now cites it as the authority for a number a
BLOCKING rule quotes. So the pairing rule it implements is the thing under test:
getting THAT wrong is what produced the 40/71 miscount it was written to replace.
"""

from __future__ import annotations

import io
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
import deferral_orphans as do  # noqa: E402

HEADER = (
    "| Date | Source | What | Reason | Destination | Status |\n"
    "|------|--------|------|--------|-------------|--------|\n"
)


def _row(status: str, what: str = "work") -> str:
    return f"| 2026-08-03 | spec-x | {what} | reason | plan/spec-home.md | {status} |\n"


class TestSpecForShard(unittest.TestCase):
    """A shard pairs with its spec by stem, in either spelling."""

    def test_conventional_stem_pairs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-foo.md").write_text("# Spec\n")
            self.assertIsNotNone(do.spec_for_shard(root, "foo"))

    def test_doubled_spec_prefix_still_pairs(self) -> None:
        # The exact defect: plan/deferrals/spec-fixit-rs-community-strip-arity.md
        # pairs with plan/spec-fixit-rs-community-strip-arity.md, NOT with
        # plan/spec-spec-fixit-... . Reading it as orphaned inflated the count
        # by one shard and three live rows, in a rule that quotes the number.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "spec-foo.md").write_text("# Spec\n")
            self.assertIsNotNone(
                do.spec_for_shard(root, "spec-foo"),
                "a shard whose stem already carries `spec-` must still find its spec",
            )

    def test_genuinely_absent_spec_is_orphaned(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            self.assertIsNone(do.spec_for_shard(root, "gone"))


class TestLiveRows(unittest.TestCase):
    def test_terminal_rows_are_not_live(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            shard = Path(tmp) / "s.md"
            shard.write_text(
                HEADER + _row("done") + _row("cancelled") + _row("resolved")
            )
            self.assertEqual(do.live_rows(shard), [])

    def test_deferred_and_open_are_both_live(self) -> None:
        # `open` and `deferred` are synonyms in the vocabulary, and a counter
        # that reads only one of them under-reports by most of the corpus.
        with tempfile.TemporaryDirectory() as tmp:
            shard = Path(tmp) / "s.md"
            shard.write_text(HEADER + _row("deferred", "a") + _row("open", "b"))
            self.assertEqual(len(do.live_rows(shard)), 2)

    def test_header_and_separator_are_not_rows(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            shard = Path(tmp) / "s.md"
            shard.write_text(HEADER)
            self.assertEqual(do.live_rows(shard), [])


class TestClassify(unittest.TestCase):
    """The partition main() prints, over trees whose answer is known by construction."""

    def test_classify_partitions_a_known_mixed_tree(self) -> None:
        # Drives classify(), the function main() prints. A test that re-walks the
        # corpus the way the script does proves nothing about the script: the
        # first version of this test incremented a total and one of two buckets
        # in the same branch, then asserted the total equalled their sum, which
        # no tree can falsify.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "spec-alive.md").write_text("# Spec\n")

            def shard(name: str, body: str) -> None:
                (root / "plan" / "deferrals" / name).write_text(HEADER + body)

            shard("alive.md", _row("deferred"))  # spec exists -> not an orphan
            shard("gone-live.md", _row("deferred") + _row("done"))  # orphan, live
            shard("gone-live2.md", _row("open"))  # orphan, live
            shard("gone-residue.md", _row("done") + _row("cancelled"))  # residue
            shard("ad-hoc-2026-08-03-aa.md", _row("deferred"))  # never an orphan

            found = do.classify(root)

            self.assertEqual(
                sorted(rel for rel, _ in found.live_bearing),
                ["plan/deferrals/gone-live.md", "plan/deferrals/gone-live2.md"],
            )
            self.assertEqual(found.residue, ["plan/deferrals/gone-residue.md"])
            self.assertEqual(found.live_row_count, 2)
            self.assertEqual(found.misnamed, [])

    def test_classify_reports_a_doubled_prefix_shard_as_misnamed(self) -> None:
        # The misnamed shard is reported SEPARATELY from the orphan split: it
        # pairs with its spec, so it is not an orphan, but every gate keyed on
        # plan/spec-<stem>.md is blind to it and that must be visible.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "spec-foo.md").write_text("# Spec\n")
            (root / "plan" / "deferrals" / "spec-foo.md").write_text(
                HEADER + _row("deferred")
            )
            found = do.classify(root)
            self.assertEqual(found.misnamed, ["plan/deferrals/spec-foo.md"])
            self.assertEqual(found.live_bearing, [], "its spec is alive, so not orphaned")
            self.assertEqual(found.residue, [])

    def test_a_shard_is_not_paired_with_an_unrelated_plan_file(self) -> None:
        # `plan/<stem>.md` is accepted only for a doubled `spec-` prefix. Tried
        # for every stem, a shard named after any file directly under plan/
        # would silently leave the orphan count.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan").mkdir()
            (root / "plan" / "implementation-order.md").write_text("# notes\n")
            self.assertIsNone(
                do.spec_for_shard(root, "implementation-order"),
                "a plain plan/ document is not a spec and must not adopt a shard",
            )

    def test_a_misnamed_shard_can_also_be_orphaned(self) -> None:
        # The two classes are not exclusive and are not meant to be. `misnamed`
        # answers "is this shard invisible to stem-keyed gates", the orphan split
        # answers "is its source spec gone". A shard with a doubled prefix AND no
        # spec at either spelling is both, and must appear in both.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            (root / "plan" / "deferrals" / "spec-nospec.md").write_text(
                HEADER + _row("deferred")
            )
            found = do.classify(root)
            self.assertEqual(found.misnamed, ["plan/deferrals/spec-nospec.md"])
            self.assertEqual(
                [rel for rel, _ in found.live_bearing],
                ["plan/deferrals/spec-nospec.md"],
            )

    def test_main_prints_the_counts_classify_produced(self) -> None:
        # main() is a printer, but it unpacks Orphans POSITIONALLY. A field-order
        # swap there would misreport every count while classify() stayed correct
        # and every test above stayed green.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "plan" / "deferrals").mkdir(parents=True)
            # ASYMMETRIC on purpose: 2 live-bearing shards against 1 residue
            # shard. With one of each, a field-order swap prints identical
            # numbers and the test cannot see it -- the same blindness that let
            # a working-tree read pass the gate's own suite.
            (root / "plan" / "deferrals" / "gone-live.md").write_text(
                HEADER + _row("deferred", "a") + _row("open", "b")
            )
            (root / "plan" / "deferrals" / "gone-live2.md").write_text(
                HEADER + _row("deferred", "c")
            )
            (root / "plan" / "deferrals" / "gone-residue.md").write_text(
                HEADER + _row("done")
            )
            out = io.StringIO()
            argv = ["deferral_orphans.py", "--repo", str(root)]
            with mock.patch.object(sys, "argv", argv), redirect_stdout(out):
                self.assertEqual(do.main(), 0)
            text = out.getvalue()
            self.assertIn("orphaned, live-bearing: 2 shards, 3 live rows", text)
            self.assertIn("orphaned, all-terminal (residue): 1 shards", text)


class TestTheRealCorpus(unittest.TestCase):
    """A ratchet over the live tree, not a unit test.

    Deliberately NOT asserting the figure `ai/rules/planning.md` quotes.
    That figure is a DATED measurement, so it is a historical fact and stops
    reproducing the moment somebody closes a spec. Pinning it would turn every
    unrelated closure red, which is how a gate teaches people to route around it.
    The shape below is different: it can only go red if somebody re-creates the
    defect, which is exactly the signal wanted.
    """

    def test_no_shard_carries_a_doubled_spec_prefix(self) -> None:
        # A shard named `spec-<stem>.md` is invisible to every gate that pairs a
        # shard with `plan/spec-<stem>.md`. One existed and hid three live rows
        # from the count. This keeps the corpus clean rather than only teaching
        # the reader to work around it.
        repo = Path(__file__).resolve().parents[2]
        doubled = [
            p.relative_to(repo).as_posix()
            for p in do.deferral_shard_paths(repo)
            if p.stem.startswith("spec-")
        ]
        self.assertEqual(
            doubled,
            [],
            "a deferral shard drops the `spec-` prefix: plan/deferrals/<stem>.md"
            " pairs with plan/spec-<stem>.md",
        )


if __name__ == "__main__":
    unittest.main()
