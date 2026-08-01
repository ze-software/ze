#!/usr/bin/env python3
"""Tests for the learned-summary path repair tool.

The tool rewrites citations inside `plan/learned/`, so the property that
matters is not how many it repairs but what it REFUSES to touch. A citation
rewritten to the wrong file reads as true and is undetectable; the dead one it
replaced was at least caught by `scripts/dev/learned_staleness.py`. Most of the
tests below therefore assert that nothing happened.

Each test builds a throwaway git repository, because the resolvers read git:
`git ls-files` for what exists and `git log --diff-filter=R` for what moved.
The fixture isolates git config completely (`GIT_CONFIG_GLOBAL`) so the running
user's signing key, hooks and identity cannot reach it.
"""

from __future__ import annotations

import contextlib
import io
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))
from learned_repath import Resolver, main, run, write_atomic

GIT_ENV = {
    **os.environ,
    "GIT_CONFIG_GLOBAL": os.devnull,
    "GIT_CONFIG_SYSTEM": os.devnull,
    "GIT_AUTHOR_NAME": "repath test",
    "GIT_AUTHOR_EMAIL": "repath@example.invalid",
    "GIT_COMMITTER_NAME": "repath test",
    "GIT_COMMITTER_EMAIL": "repath@example.invalid",
}

SUMMARY = """# 900 -- fixture

## Context

Nothing.

## Files

{body}
"""


def git(root: Path, *args: str) -> None:
    subprocess.run(
        ["git", "-C", str(root), *args],
        check=True,
        env=GIT_ENV,
        capture_output=True,
        text=True,
    )


class RepathCase(unittest.TestCase):
    """A git fixture plus helpers to write a summary and read it back."""

    def setUp(self) -> None:
        self.root = Path(tempfile.mkdtemp(prefix="learned-repath-")).resolve()
        self.addCleanup(shutil.rmtree, self.root, ignore_errors=True)
        git(self.root, "init", "-q", "-b", "main")

    def add(self, rel: str, body: str = "package x\n") -> None:
        p = self.root / rel
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(body, encoding="utf-8")
        git(self.root, "add", rel)

    def commit(self, message: str) -> None:
        git(self.root, "commit", "-q", "-m", message)

    def move(self, src: str, dst: str) -> None:
        """`git mv`, creating the destination's parent, which git will not."""
        (self.root / dst).parent.mkdir(parents=True, exist_ok=True)
        git(self.root, "mv", src, dst)

    def summary(self, body: str, number: str = "900") -> Path:
        p = self.root / "plan" / "learned" / f"{number}-fixture.md"
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(SUMMARY.format(body=body), encoding="utf-8")
        git(self.root, "add", str(p.relative_to(self.root)))
        return p

    def apply(self) -> dict:
        return run(self.root, apply=True)


class TestUniqueSuccessorIsRewritten(RepathCase):
    """VALIDATES: a dead path whose successor is unambiguous is repointed.
    PREVENTS: the corpus keeping a dead citation for code that merely moved."""

    def test_recorded_rename_is_followed(self) -> None:
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move isis to plugins")

        path = self.summary("- `internal/component/isis/spf/ipv6.go` -- the walker\n")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("renamed"), 1)
        self.assertIn(
            "`internal/plugins/isis/spf/ipv6.go`", path.read_text(encoding="utf-8")
        )

    def test_line_suffix_and_prose_survive_the_rewrite(self) -> None:
        """The token is replaced inside its own span; `:42` and the trailing
        sentence are not part of the path and must come through untouched."""
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move isis to plugins")

        path = self.summary(
            "- `internal/component/isis/spf/ipv6.go:42` -- the walker, see above\n"
        )
        self.apply()

        text = path.read_text(encoding="utf-8")
        self.assertIn("`internal/plugins/isis/spf/ipv6.go:42`", text)
        self.assertIn("-- the walker, see above", text)

    def test_unique_three_segment_suffix_is_followed(self) -> None:
        """No rename was recorded (the file was deleted and re-added), so the
        suffix resolver carries it on `bfd/transport/udp_linux.go`."""
        self.add("internal/plugins/bfd/transport/udp_linux.go")
        self.commit("add")

        path = self.summary("- `internal/component/bfd/transport/udp_linux.go`\n")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("moved"), 1)
        self.assertIn(
            "`internal/plugins/bfd/transport/udp_linux.go`",
            path.read_text(encoding="utf-8"),
        )


class TestAmbiguousIsLeftAlone(RepathCase):
    """VALIDATES: several plausible successors means no rewrite at all.
    PREVENTS: a citation silently repointed at the wrong file, which reads as
    true and which no gate can detect."""

    def test_two_owners_of_the_same_suffix_block_the_rewrite(self) -> None:
        """Both candidates share the full three-segment suffix, so the suffix
        resolver has two answers and is required to give none."""
        self.add("internal/component/a/shared/transport/udp.go")
        self.add("internal/plugins/b/shared/transport/udp.go")
        self.commit("add both")

        body = "- `internal/component/gone/shared/transport/udp.go`\n"
        path = self.summary(body)
        before = path.read_text(encoding="utf-8")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("ambiguous"), 1)
        self.assertNotIn("moved", report["verdicts"])
        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_bare_basename_is_never_evidence(self) -> None:
        """One tracked `orchestrator.go` exists, but it shares no two-segment
        suffix with the citation. A basename alone must not carry a rewrite:
        that match once pointed `cmd/ze-chaos/orchestrator.go` at an unrelated
        `config/transaction/orchestrator.go`."""
        self.add("internal/component/config/transaction/orchestrator.go")
        self.commit("add")

        path = self.summary("- `cmd/ze-chaos/orchestrator.go`\n")
        before = path.read_text(encoding="utf-8")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("gone"), 1)
        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_split_rename_is_refused(self) -> None:
        """git recorded the file moving to two places. The reader must choose,
        so the tool does not, and it does not fall through to the weaker suffix
        resolver either: the stronger evidence already said "do not guess"."""
        self.add("internal/new/one/split.go")
        self.add("internal/new/two/split.go")
        self.commit("add both")

        resolver = Resolver(self.root)
        resolver.renames["internal/old/pkg/split.go"] = {
            "internal/new/one/split.go",
            "internal/new/two/split.go",
        }
        verdict, successor = resolver.resolve("internal/old/pkg/split.go")

        self.assertEqual(verdict, "ambiguous")
        self.assertIsNone(successor)


class TestLearnedDirectoryRule(RepathCase):
    """VALIDATES: a subtree move carries files git never recorded moving.
    PREVENTS: the rule inventing a destination for a file that did not survive
    the move."""

    def move_the_subtree(self) -> None:
        self.add("internal/plugins/fibkernel/a.go")
        self.add("internal/plugins/fibkernel/b.go")
        self.commit("add")
        self.move("internal/plugins/fibkernel", "internal/plugins/fib/kernel")
        self.commit("split fib by backend")

    def test_rule_carries_an_unrecorded_file(self) -> None:
        self.move_the_subtree()
        self.add("internal/plugins/fib/kernel/c.go")
        self.commit("add c under the new name")

        path = self.summary("- `internal/plugins/fibkernel/c.go`\n")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("relocated"), 1)
        self.assertIn(
            "`internal/plugins/fib/kernel/c.go`", path.read_text(encoding="utf-8")
        )

    def test_rule_does_not_invent_a_file_that_never_arrived(self) -> None:
        """The directory move is proven; this file's survival is not. A rule
        that rewrote regardless would produce a citation that looks live and
        points at nothing."""
        self.move_the_subtree()

        path = self.summary("- `internal/plugins/fibkernel/deleted.go`\n")
        before = path.read_text(encoding="utf-8")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("gone"), 1)
        self.assertNotIn("relocated", report["verdicts"])
        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_split_subtree_yields_no_rule(self) -> None:
        """One directory whose files went two ways has no single successor, so
        the rule is dropped rather than decided by majority."""
        self.add("internal/old/a.go")
        self.add("internal/old/b.go")
        self.commit("add")
        (self.root / "internal/one").mkdir(parents=True, exist_ok=True)
        (self.root / "internal/two").mkdir(parents=True, exist_ok=True)
        git(self.root, "mv", "internal/old/a.go", "internal/one/a.go")
        git(self.root, "mv", "internal/old/b.go", "internal/two/b.go")
        self.commit("split")

        self.assertNotIn("internal/old", Resolver(self.root).rules)


class TestCitedAsDeleted(RepathCase):
    """VALIDATES: a citation the summary calls deleted is never repointed.
    PREVENTS: turning a true sentence false. Repointing `x.go -- deleted` at
    the live successor leaves the summary saying a file that exists was
    deleted, which is worse than the dead path it replaced."""

    def setUp(self) -> None:
        super().setUp()
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")

    def test_bare_deletion_marker_protects_the_line(self) -> None:
        path = self.summary(
            "- `internal/component/isis/spf/ipv6.go` -- deleted (moved)\n"
        )
        before = path.read_text(encoding="utf-8")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("cited-as-deleted"), 1)
        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_deletion_label_protects_the_line(self) -> None:
        path = self.summary("- Deleted: `internal/component/isis/spf/ipv6.go`\n")
        before = path.read_text(encoding="utf-8")
        self.apply()

        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_code_removed_from_a_living_file_is_still_repaired(self) -> None:
        """ "removed X" describes code taken OUT of a file that still exists.
        The citation is stale, the sentence stays true, so it is repaired."""
        path = self.summary(
            "- `internal/component/isis/spf/ipv6.go` -- removed the v4 branch\n"
        )
        report = self.apply()

        self.assertEqual(report["verdicts"].get("renamed"), 1)
        text = path.read_text(encoding="utf-8")
        self.assertIn("`internal/plugins/isis/spf/ipv6.go`", text)
        self.assertIn("-- removed the v4 branch", text)


class TestNoSuccessorIsLeftAlone(RepathCase):
    """VALIDATES: a genuinely deleted file keeps its citation.
    PREVENTS: deleting the reference to make the number look better, which is
    the failure `plan/spec-knowledge-1-corpus.md` exists to prevent."""

    def test_deleted_file_keeps_its_line(self) -> None:
        self.add("internal/component/bgp/live.go")
        self.commit("add")

        body = "- `internal/component/mcp/session.go` -- deleted with the plugin\n"
        path = self.summary(body)
        before = path.read_text(encoding="utf-8")
        report = self.apply()

        self.assertEqual(report["verdicts"].get("gone"), 1)
        self.assertEqual(path.read_text(encoding="utf-8"), before)
        self.assertIn("internal/component/mcp/session.go", path.read_text("utf-8"))


class TestApplyIsIdempotent(RepathCase):
    """VALIDATES: a second --apply changes nothing.
    PREVENTS: a repair that walks a citation further on every run."""

    def test_second_pass_is_a_no_op(self) -> None:
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")

        path = self.summary("- `internal/component/isis/spf/ipv6.go`\n")
        first = self.apply()
        after_first = path.read_text(encoding="utf-8")

        second = run(self.root, apply=True)

        self.assertEqual(first["summaries_rewritten"], 1)
        self.assertEqual(second["summaries_rewritten"], 0)
        self.assertEqual(second["verdicts"], {})
        self.assertEqual(path.read_text(encoding="utf-8"), after_first)


class TestCheckWritesNothing(RepathCase):
    """VALIDATES: --check reports a repair it does not perform.
    PREVENTS: a dry run that mutates the corpus anyway."""

    def test_check_leaves_bytes_identical(self) -> None:
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")

        path = self.summary("- `internal/component/isis/spf/ipv6.go`\n")
        before = path.read_bytes()

        report = run(self.root, apply=False)

        self.assertEqual(report["verdicts"].get("renamed"), 1)
        self.assertEqual(report["summaries_rewritten"], 1)
        self.assertFalse(report["applied"])
        self.assertEqual(path.read_bytes(), before)

    def test_default_invocation_does_not_apply(self) -> None:
        """No flag at all is the safe mode, not a silent --apply."""
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")

        path = self.summary("- `internal/component/isis/spf/ipv6.go`\n")
        before = path.read_bytes()

        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = main(["--repo", str(self.root)])

        self.assertEqual(code, 0)
        self.assertEqual(path.read_bytes(), before)
        self.assertIn("nothing was written", buf.getvalue())


class TestBraceSpans(RepathCase):
    """VALIDATES: a `{a,b}` span is repaired only as one agreed head swap.
    PREVENTS: a substring rewrite that mangles a template into a path naming
    neither of the files it stood for."""

    def test_members_that_moved_together_are_repaired_as_one_swap(self) -> None:
        self.add("internal/component/isis/a.go")
        self.add("internal/component/isis/b.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")

        path = self.summary("- `internal/component/isis/{a,b}.go` -- both\n")
        self.apply()

        text = path.read_text(encoding="utf-8")
        self.assertIn("`internal/plugins/isis/{a,b}.go`", text)
        self.assertIn("-- both", text)

    def test_a_live_member_blocks_the_swap(self) -> None:
        """`a.go` still resolves where it is cited. Moving the head would break
        a citation that was correct, to repair one that was not."""
        self.add("internal/component/isis/a.go")
        self.add("internal/plugins/isis/b.go")
        self.commit("add")

        path = self.summary("- `internal/component/isis/{a,b}.go`\n")
        before = path.read_text(encoding="utf-8")
        self.apply()

        self.assertEqual(path.read_text(encoding="utf-8"), before)

    def test_members_that_went_separate_ways_block_the_swap(self) -> None:
        """Both expansions resolve, but to different directories, so no single
        head substitution describes the move."""
        self.add("internal/old/a.go")
        self.add("internal/old/b.go")
        self.commit("add")
        (self.root / "internal/one").mkdir(parents=True, exist_ok=True)
        (self.root / "internal/two").mkdir(parents=True, exist_ok=True)
        git(self.root, "mv", "internal/old/a.go", "internal/one/a.go")
        git(self.root, "mv", "internal/old/b.go", "internal/two/b.go")
        self.commit("split")

        path = self.summary("- `internal/old/{a,b}.go`\n")
        before = path.read_text(encoding="utf-8")
        self.apply()

        self.assertEqual(path.read_text(encoding="utf-8"), before)


class TestAtomicWrite(RepathCase):
    """VALIDATES: the replacement keeps the summary's mode and leaves no debris.
    PREVENTS: two failures the corpus cannot show you. A narrowed mode is
    invisible in `git diff`, which tracks the exec bit alone, and a fixed temp
    name lets two concurrent `--apply` runs interleave into one file and rename
    each other's half into place."""

    def _repairable(self) -> Path:
        self.add("internal/component/isis/spf/ipv6.go")
        self.commit("add")
        self.move("internal/component/isis", "internal/plugins/isis")
        self.commit("move")
        return self.summary("- `internal/component/isis/spf/ipv6.go`\n")

    def test_apply_preserves_the_file_mode(self):
        path = self._repairable()
        os.chmod(path, 0o644)

        report = self.apply()

        self.assertEqual(report["summaries_rewritten"], 1, "nothing was rewritten")
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o644)

    def test_a_non_baseline_mode_is_carried_across_unchanged(self):
        # Preserved, not reset to a hardcoded 0644.
        path = self._repairable()
        os.chmod(path, 0o640)
        self.apply()
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o640)

    def test_the_temp_name_is_unique_per_call(self):
        # A fixed `<name>.repath.tmp` is the collision. Two writes to the same
        # target must not be able to pick the same scratch file.
        path = self._repairable()
        seen: set[str] = set()
        real_mkstemp = tempfile.mkstemp

        def record(*args, **kwargs):
            fd, name = real_mkstemp(*args, **kwargs)
            seen.add(name)
            return fd, name

        with mock.patch("learned_repath.tempfile.mkstemp", record):
            write_atomic(path, ["a\n"])
            write_atomic(path, ["b\n"])

        self.assertEqual(len(seen), 2, f"the temp name was reused: {seen}")
        self.assertEqual(path.read_text(encoding="utf-8"), "b\n")

    def test_a_failed_write_orphans_no_temp_file(self):
        path = self._repairable()
        learned = path.parent
        before = sorted(p.name for p in learned.iterdir())

        with (
            mock.patch(
                "learned_repath.os.replace", side_effect=OSError("cross-device")
            ),
            self.assertRaises(OSError),
        ):
            write_atomic(path, ["repaired\n"])

        self.assertEqual(
            sorted(p.name for p in learned.iterdir()),
            before,
            "a temp file was left beside the summary",
        )


if __name__ == "__main__":
    unittest.main()
