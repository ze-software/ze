#!/usr/bin/env python3
"""Tests for the report vocabulary.

The verdict is what the whole setup run is judged by, and the shell version got
it wrong in one specific way: a tool that installed into a directory off PATH
printed `[installed]`, appended to no failure list, and the run ended "Setup
complete" with exit 0 while a probe-only run on the same box exited 1. These
tests pin the property that makes that impossible -- the label and the verdict
come from one value.
"""

from __future__ import annotations

import io
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.console import Outcome, Report, State


def _summarise(report: Report) -> tuple[int, str]:
    """Run the closing verdict with its output captured."""
    buffer = io.StringIO()
    with redirect_stdout(buffer):
        code = report.summarise()
    return code, buffer.getvalue()


def _add(report: Report, *outcomes: Outcome) -> None:
    """Record outcomes without their lines reaching the test's output."""
    with redirect_stdout(io.StringIO()):
        for outcome in outcomes:
            report.add(outcome)


class TestStateBlocking(unittest.TestCase):
    def test_pending_blocks(self) -> None:
        """A tool on the disk but not on PATH must fail the run.

        This is the exact state the shell version reported success for.
        """
        assert State.PENDING.blocking

    def test_missing_blocks(self) -> None:
        assert State.MISSING.blocking

    def test_present_installed_skipped_do_not_block(self) -> None:
        for state in (State.PRESENT, State.INSTALLED, State.SKIPPED):
            assert not state.blocking, state


class TestVerdict(unittest.TestCase):
    def test_all_present_succeeds(self) -> None:
        report = Report()
        _add(report, Outcome('go', State.PRESENT), Outcome('git', State.PRESENT))
        code, out = _summarise(report)
        assert code == 0
        assert 'All tools already present' in out

    def test_pending_fails_and_names_the_tool(self) -> None:
        report = Report()
        _add(
            report,
            Outcome('go', State.PRESENT),
            Outcome('ruff', State.PENDING, 'installed, not on PATH'),
        )
        code, out = _summarise(report)
        assert code == 1
        assert 'ruff' in out
        # "Steps", not "install commands": the install worked, the PATH did not.
        assert 'Finish the steps above' in out

    def test_missing_is_reported_before_pending(self) -> None:
        """Missing is the harder failure, so it is the one a reader sees first."""
        report = Report()
        _add(
            report,
            Outcome('ruff', State.PENDING, 'installed, not on PATH'),
            Outcome('go', State.MISSING, 'required'),
        )
        code, out = _summarise(report)
        assert code == 1
        assert 'Missing required tools: go' in out
        assert 'Finish the steps above' not in out

    def test_skipped_alone_still_succeeds(self) -> None:
        report = Report()
        _add(report, Outcome('colima', State.SKIPPED, 'macOS Docker runtime'))
        code, out = _summarise(report)
        assert code == 0
        assert 'skipped (optional): colima' in out

    def test_installed_is_summarised(self) -> None:
        report = Report()
        _add(report, Outcome('ruff', State.INSTALLED))
        code, out = _summarise(report)
        assert code == 0
        assert 'installed: ruff' in out


class TestCheckVerdict(unittest.TestCase):
    """A probe-only run reaches the same verdict as an install run would.

    The two modes ask the same questions of the same machine, so a state that
    fails one must fail the other. The shell version let them disagree
    permanently.
    """

    def test_missing_fails(self) -> None:
        report = Report()
        _add(report, Outcome('go', State.MISSING, 'REQUIRED'))
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = report.check_verdict()
        assert code == 1
        assert 'go' in buffer.getvalue()

    def test_pending_is_reported_apart_from_missing(self) -> None:
        """Their fixes differ, so calling a plugin a missing tool misdirects.

        A probe-only run reaches PENDING for what only a human can do: install
        a plugin, log back in. Sending that reader to the tool table wastes the
        one line they read.
        """
        report = Report()
        _add(
            report,
            Outcome('go', State.MISSING, 'REQUIRED'),
            Outcome('pyright-lsp-installed', State.PENDING, 'the LSP tool refuses .py'),
        )
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            code = report.check_verdict()
        out = buffer.getvalue()
        assert code == 1
        assert 'Missing required tools: go' in out
        assert 'Needs a step only you can take: pyright-lsp-installed' in out

    def test_every_blocking_state_fails_both_verdicts(self) -> None:
        for state in (State.PENDING, State.MISSING):
            check = Report()
            _add(check, Outcome('t', state, ''))
            install = Report()
            _add(install, Outcome('t', state, ''))
            with redirect_stdout(io.StringIO()):
                assert check.check_verdict() == 1, state
                assert install.summarise() == 1, state


class TestOutcomeLine(unittest.TestCase):
    def test_detail_is_shown_when_present(self) -> None:
        line = Outcome('gopls-answers', State.PRESENT, '30 symbols').line()
        assert 'gopls-answers' in line
        assert '30 symbols' in line

    def test_no_empty_parentheses_without_detail(self) -> None:
        assert '()' not in Outcome('go', State.PRESENT).line()


if __name__ == '__main__':
    unittest.main()
