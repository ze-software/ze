#!/usr/bin/env python3
"""Tests for the committed-tree import check.

The failure this guards against is recorded in `le/devtools/tracked.py`: a
clean archive of HEAD failed to load 21 of 21 areas while the working tree ran
perfectly, and three individually-correct commits produced it.

The case that matters most here is `test_it_catches_the_commit_that_was_broken`,
which points the check at that real commit. A guard that has never been shown
to fail is a guard nobody has tested.
"""

from __future__ import annotations

import io
import subprocess
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application import tracked as app
from le.devtools.tracked import check_tracked_import
from le.paths import REPO_ROOT

# The commit that was live and broken on 2026-08-25: it carried `gate.py`
# importing `stream` without the `le/process.py` that defines it.
BROKEN_COMMIT = 'ce47db3bf'


def _commit_exists(revision: str) -> bool:
    return (
        subprocess.run(
            ['git', 'cat-file', '-e', f'{revision}^{{commit}}'],
            cwd=str(REPO_ROOT),
            capture_output=True,
            check=False,
        ).returncode
        == 0
    )


class TestAgainstTheRealHistory(unittest.TestCase):
    def test_head_loads_every_area(self) -> None:
        """If this fails, `le` is broken for everyone but this working tree."""
        verdict = check_tracked_import('HEAD')
        assert verdict.ok, f'HEAD does not load: {verdict.broken or verdict.detail}'
        assert verdict.areas > 0

    def test_it_catches_the_commit_that_was_broken(self) -> None:
        """The guard must be able to FAIL, shown against the real failure.

        `ce47db3bf` carried `gate.py` importing `stream` from `le.process`
        without the commit that added it. 18 of 21 areas do not load there.
        """
        if not _commit_exists(BROKEN_COMMIT):
            self.skipTest(f'{BROKEN_COMMIT} is not in this checkout')
        verdict = check_tracked_import(BROKEN_COMMIT)
        assert not verdict.ok, 'the known-broken commit must not pass'
        assert verdict.broken, 'a failure must name the areas that did not load'
        assert any('stream' in line for line in verdict.broken), (
            'the report must name the missing symbol, not merely that something failed'
        )

    def test_the_report_names_every_broken_area(self) -> None:
        if not _commit_exists(BROKEN_COMMIT):
            self.skipTest(f'{BROKEN_COMMIT} is not in this checkout')
        verdict = check_tracked_import(BROKEN_COMMIT)
        assert len(verdict.broken) > 1, 'one line per broken area, not a summary'


class TestItReadsGitAndNotTheTree(unittest.TestCase):
    """The whole point: a file present here and absent from the commit."""

    def test_an_unknown_revision_is_refused(self) -> None:
        verdict = check_tracked_import('no-such-revision-exists')
        assert not verdict.ok
        assert verdict.detail, 'it must say why rather than report zero broken areas'

    def test_a_refusal_is_not_a_pass(self) -> None:
        """A check that cannot read the tree must never answer green."""
        verdict = check_tracked_import('no-such-revision-exists')
        assert not verdict.ok


class TestExitCodes(unittest.TestCase):
    def test_a_clean_head_is_zero(self) -> None:
        with redirect_stdout(io.StringIO()):
            assert app.action(app.Options(revision='HEAD')) == 0

    def test_a_broken_commit_is_one(self) -> None:
        if not _commit_exists(BROKEN_COMMIT):
            self.skipTest(f'{BROKEN_COMMIT} is not in this checkout')
        with redirect_stdout(io.StringIO()):
            assert app.action(app.Options(revision=BROKEN_COMMIT)) == 1

    def test_an_unreadable_revision_is_two(self) -> None:
        """Distinct from 1: the tree is broken, versus the question is unanswerable."""
        with redirect_stdout(io.StringIO()):
            assert app.action(app.Options(revision='no-such-revision-exists')) == 2

    def test_the_failure_says_what_to_understand(self) -> None:
        if not _commit_exists(BROKEN_COMMIT):
            self.skipTest(f'{BROKEN_COMMIT} is not in this checkout')
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            app.action(app.Options(revision=BROKEN_COMMIT))
        said = buffer.getvalue()
        assert 'working tree' in said, 'the reader must learn which tree disagreed'


if __name__ == '__main__':
    unittest.main()
