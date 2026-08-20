#!/usr/bin/env python3
"""Tests for scripts/dev/journal.py.

The acceptance tests drive `main()` over a REAL git fixture, because the HEAD
read is the whole design point: an earlier version of these tests passed a
directory straight to `report()`, so `read_journal_at_head()` had no coverage
at all and its fail-open branch reported zero rows over 28 files.  The fixture
pattern is the one `scripts/dev/commit_helper_test.py` `_git()` uses:
`tempfile.TemporaryDirectory` plus `git init`.
"""

from __future__ import annotations

import io
import subprocess
import sys
import tempfile
import textwrap
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest import TestCase, main as unittest_main

sys.path.insert(0, str(Path(__file__).resolve().parent))

from journal import (  # noqa: E402
    MALFORMED,
    JournalError,
    journal_row_cells,
    journal_spec_stems,
    main,
    read_journal_at_head,
)

TABLE_HEAD = "| Date | Spec | Surface | Symptom | Fix |\n|------|------|---------|---------|-----|\n"


def _git(root: Path, *args: str) -> None:
    subprocess.run(["git", *args], cwd=root, check=True, capture_output=True, text=True)


def _fixture(tmp: str, files: dict[str, str], *, commit: bool = True) -> Path:
    """A git repo holding plan/journal/<name>.md files, committed by default."""
    root = Path(tmp)
    _git(root, "init", "-q", "-b", "main")
    _git(root, "config", "user.email", "test@example.com")
    _git(root, "config", "user.name", "Ze Test")
    _git(root, "config", "commit.gpgsign", "false")
    journal = root / "plan" / "journal"
    journal.mkdir(parents=True)
    for name, body in files.items():
        (journal / name).write_text(body)
    (root / "README.md").write_text("fixture\n")
    if commit:
        _git(root, "add", "-A")
        _git(root, "commit", "-q", "-m", "seed journal")
    else:
        # Commit something else, so HEAD exists but carries no journal.
        _git(root, "add", "--", "README.md")
        _git(root, "commit", "-q", "-m", "no journal")
    return root


def _run(root: Path) -> tuple[int, str, str]:
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = main(["--repo", str(root)])
    return code, out.getvalue(), err.getvalue()


class TestJournalRowCells(TestCase):
    """Unit tests for the cell parser."""

    def test_data_row(self) -> None:
        line = "| 2026-07-15 | some-spec | reactor | symptom text | fix text |"
        cells = journal_row_cells(line)
        assert cells is not None
        self.assertEqual(len(cells), 5)
        self.assertEqual(cells[0], "2026-07-15")
        self.assertEqual(cells[1], "some-spec")

    def test_header_row_returns_none(self) -> None:
        line = "| Date | Spec | Surface | Symptom | Fix |"
        self.assertIsNone(journal_row_cells(line))

    def test_separator_row_returns_none(self) -> None:
        line = "|------|------|---------|---------|-----|"
        self.assertIsNone(journal_row_cells(line))

    def test_prose_returns_none(self) -> None:
        self.assertIsNone(journal_row_cells("Some prose line"))
        self.assertIsNone(journal_row_cells(""))

    def test_wrong_column_count_returns_malformed(self) -> None:
        line = "| a | b | c |"
        self.assertEqual(journal_row_cells(line), [MALFORMED])

    def test_six_columns_returns_malformed(self) -> None:
        line = "| a | b | c | d | e | f |"
        self.assertEqual(journal_row_cells(line), [MALFORMED])

    # VALIDATES: AC-4 -- a row that lost its leading pipe is named, not skipped.
    # PREVENTS: a row vanishing silently inside a table, which drops one
    # occurrence from the count that IS the recurrence signal.
    def test_missing_leading_pipe_returns_malformed(self) -> None:
        line = "2026-07-15 | some-spec | reactor | symptom | fix |"
        self.assertEqual(journal_row_cells(line), [MALFORMED])


class TestJournalSpecStems(TestCase):
    """Unit tests for the Spec cell parser."""

    def test_a_stem_is_itself(self) -> None:
        self.assertEqual(journal_spec_stems("some-spec"), ["some-spec"])

    def test_dash_and_none_name_no_spec(self) -> None:
        for cell in ("-", "none", "None", "n/a", ""):
            self.assertEqual(journal_spec_stems(cell), [], cell)

    # VALIDATES: a trailing note is the author writing to the next reader, so it
    # is stripped and the stem still answers.
    def test_a_trailing_note_is_not_part_of_the_stem(self) -> None:
        self.assertEqual(
            journal_spec_stems("verify-scope-1 (measurement only)"), ["verify-scope-1"]
        )

    # VALIDATES: the shape that sent the review gate to an unwritable path.
    # PREVENTS: `none (walked into during <spec> closure)` reading as a stem, so
    # the gate demands tmp/review/none (walked into ...)-<session>.md from a
    # commit that closes no spec.
    def test_none_with_a_note_names_no_spec(self) -> None:
        self.assertEqual(
            journal_spec_stems("none (walked into during other-spec closure)"), []
        )

    def test_a_comma_list_names_both_specs(self) -> None:
        self.assertEqual(journal_spec_stems("spec-a, spec-b"), ["spec-a", "spec-b"])

    # VALIDATES: unreadable is not "no spec". None is what
    # `commit_helper.journal_row_problems` blocks on, and reading it as [] would
    # take the review gate off the commit carrying the code.
    def test_prose_is_unreadable(self) -> None:
        self.assertIsNone(journal_spec_stems("the spec that found it"))
        self.assertIsNone(journal_spec_stems("spec-a (note (nested))"))


class TestReadJournalAtHead(TestCase):
    """BLOCKER A: an empty result, a missing journal, and a git failure differ."""

    # VALIDATES: no journal at HEAD and none in the working tree is a legitimate
    # empty result.
    def test_no_journal_anywhere_is_empty(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _git(root, "init", "-q", "-b", "main")
            _git(root, "config", "user.email", "test@example.com")
            _git(root, "config", "user.name", "Ze Test")
            _git(root, "config", "commit.gpgsign", "false")
            (root / "README.md").write_text("fixture\n")
            _git(root, "add", "-A")
            _git(root, "commit", "-q", "-m", "no journal")
            self.assertEqual(read_journal_at_head(root), ({}, []))
            self.assertEqual(_run(root), (0, "", ""))

    # VALIDATES: a journal on disk that HEAD does not carry is an ERROR.
    # PREVENTS: the detector reporting "nothing to see" over an uncommitted
    # journal, which is how it stayed green across 28 files and 52 rows.
    def test_uncommitted_journal_is_an_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {"a-class.md": TABLE_HEAD + "| 2026-07-15 | - | x | y | z |\n"},
                commit=False,
            )
            with self.assertRaises(JournalError) as caught:
                read_journal_at_head(root)
            self.assertIn("HEAD carries no journal class file", str(caught.exception))
            code, _, err = _run(root)
            self.assertEqual(code, 2)
            self.assertIn("journal:", err)

    # VALIDATES: HEAD carrying ONLY plan/journal/README.md is the same "no
    # journal at HEAD" state as HEAD carrying nothing, so a class file on disk
    # is still an error.
    # PREVENTS: the on-disk guard keying on the raw HEAD listing, which counts
    # README.md.  With the README committed and the class files not, the guard
    # was skipped, the class-file loop ran over an empty list, and `make
    # ze-journal-report` exited 0 printing nothing over a full journal.  This is the
    # ordering the real repository passes through: README.md lands with the
    # directory, class files land with the work.
    def test_head_with_only_readme_is_an_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(tmp, {"README.md": "# Problem Journal\n"}, commit=True)
            (root / "plan" / "journal" / "a-class.md").write_text(
                TABLE_HEAD
                + "| 2026-07-15 | - | x | y | z |\n"
                + "| 2026-07-16 | - | x | y | z |\n"
            )
            with self.assertRaises(JournalError) as caught:
                read_journal_at_head(root)
            self.assertIn("HEAD carries no journal class file", str(caught.exception))
            code, out, err = _run(root)
            self.assertEqual(code, 2)
            self.assertEqual(out, "")
            self.assertIn("journal:", err)

    # VALIDATES: with one class at HEAD and another only on disk, the second is
    # NAMED on stderr.  The count is over HEAD, so its rows are not in it.
    # PREVENTS: the guard's docstring promising more than the guard did.  The
    # on-disk check sat inside `if not paths:`, so it fired only when HEAD
    # carried ZERO class files; with one class committed the second class was
    # absent from the report with nothing said, which is how a recurrence the
    # detector exists to raise stays invisible.
    def test_a_class_file_not_at_head_is_named(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {
                    "committed.md": TABLE_HEAD
                    + "| 2026-07-15 | - | x | y | z |\n"
                    + "| 2026-07-16 | - | x | y | z |\n"
                },
                commit=True,
            )
            (root / "plan" / "journal" / "uncommitted.md").write_text(
                TABLE_HEAD
                + "| 2026-08-01 | - | x | y | z |\n"
                + "| 2026-08-02 | - | x | y | z |\n"
            )
            classes, unread = read_journal_at_head(root)
            self.assertEqual(sorted(classes), ["committed"])
            self.assertEqual(unread, ["plan/journal/uncommitted.md"])
            code, out, err = _run(root)
            self.assertEqual(code, 0, "an uncommitted class file is not a failure")
            self.assertIn("committed: 2 rows", out)
            self.assertIn("NOT AT HEAD: plan/journal/uncommitted.md", err)

    # VALIDATES: a git invocation that fails exits non-zero and says so.
    # PREVENTS: "fails open", which made a broken git indistinguishable from a
    # clean journal.
    def test_git_failure_is_an_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)  # not a git repository at all
            (root / "plan" / "journal").mkdir(parents=True)
            with self.assertRaises(JournalError):
                read_journal_at_head(root)
            code, _, err = _run(root)
            self.assertEqual(code, 2)
            self.assertIn("journal:", err)


class TestReportFlagsSecondOccurrence(TestCase):
    """AC-1: a class with 2+ rows is printed with count and date span."""

    def test_report_flags_second_occurrence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {
                    "reactor-fsm-error.md": textwrap.dedent("""\
                        # reactor-fsm-error

                        | Date | Spec | Surface | Symptom | Fix |
                        |------|------|---------|---------|-----|
                        | 2026-06-10 | fixit-a | reactor | wrong state | fixed table |
                        | 2026-07-15 | fixit-b | reactor | same class | fixed again |
                    """),
                },
            )
            code, out, err = _run(root)
            self.assertEqual(code, 0, err)
            self.assertIn("reactor-fsm-error", out)
            self.assertIn("2 rows", out)
            # Date span: 2026-06-10 to 2026-07-15 = 35 days.
            self.assertIn("35d span", out)


class TestReportSilentOnSingletons(TestCase):
    """AC-2: a class with 1 row produces no output and exit 0."""

    def test_report_silent_on_singletons(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {
                    "one-off-problem.md": TABLE_HEAD
                    + "| 2026-07-15 | fixit-x | config | some symptom | some fix |\n",
                },
            )
            code, out, err = _run(root)
            self.assertEqual(code, 0, err)
            self.assertEqual(out, "")


class TestMalformedTableIsNamed(TestCase):
    """AC-4: a malformed table names the file and exits non-zero."""

    def test_malformed_table_is_named(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {"bad-format.md": TABLE_HEAD + "| 2026-07-15 | fixit-y | config |\n"},
            )
            code, _, err = _run(root)
            self.assertEqual(code, 1)
            self.assertIn("plan/journal/bad-format.md", err)
            self.assertIn("MALFORMED", err)


class TestUnparseableDateIsAnError(TestCase):
    """BLOCKER C: a Date the span cannot be computed from is an error."""

    # VALIDATES: a row whose Date is not YYYY-MM-DD names the file and exits
    # non-zero, instead of printing "no parseable dates" and returning 0.
    # PREVENTS: a seed of dateless rows passing every gate.
    def test_dash_date_is_an_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture(
                tmp,
                {
                    "undated.md": TABLE_HEAD
                    + "| - | - | config | one | fix |\n"
                    + "| - | - | config | two | fix |\n",
                },
            )
            code, out, err = _run(root)
            self.assertEqual(code, 1)
            self.assertIn("plan/journal/undated.md", err)
            self.assertIn("UNPARSEABLE DATE", err)
            self.assertEqual(out, "")


if __name__ == "__main__":
    unittest_main()
