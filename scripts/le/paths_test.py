#!/usr/bin/env python3
"""Tests for finding the checkout.

This is what makes a file movable. 94 of the 172 files being brought under `le`
locate the repository by counting directories up from their own `__file__`:
`parents[2]` means the checkout in `scripts/dev/foo.py` and means `scripts/` in
`scripts/le/dev/foo.py`. Nothing raises when that changes. The path is simply
wrong, and a gate reads the wrong tree.

So the root is answered once, three ways, in a fixed order of authority.
"""

from __future__ import annotations

import io
import os
import sys
import tempfile
import tokenize
import unittest
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le import paths
from le.paths import MARKERS, ROOT_ENV, relative_to_root, repo_root


def _checkout() -> Path:
    """A directory that looks like a Ze checkout: both markers, nothing else."""
    root = Path(tempfile.mkdtemp())
    for marker in MARKERS:
        (root / marker).write_text('')
    return root


class TestTheEnvironmentWins(unittest.TestCase):
    """Set means set. The environment knows things the filesystem cannot.

    A container that mounted the tree elsewhere, a worktree, a fixture standing
    in for a checkout. `scripts/evidence/qemu-all-tests.sh` exports
    `/workspace` for exactly this reason, and `zeRepoRootEnv`
    (internal/test/runner/runner_exec_util.go) passes it to every test
    subprocess.
    """

    def test_a_named_root_is_used(self) -> None:
        with mock.patch.dict(os.environ, {ROOT_ENV: '/workspace'}):
            assert repo_root() == Path('/workspace')

    def test_it_beats_discovery(self) -> None:
        """Even standing in a real checkout, the named root wins."""
        with mock.patch.dict(os.environ, {ROOT_ENV: '/elsewhere'}):
            assert repo_root() == Path('/elsewhere')

    def test_an_empty_value_is_not_a_root(self) -> None:
        """`ZE_REPO_ROOT=` is an unset variable spelled differently."""
        with mock.patch.dict(os.environ, {ROOT_ENV: ''}):
            assert repo_root() == paths.REPO_ROOT


class TestDiscovery(unittest.TestCase):
    """With nothing set, the root is found by walking up for the markers.

    This is the half that removes the positional dependency: a file can move to
    any depth and still find the checkout.
    """

    def test_the_real_root_is_found(self) -> None:
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(ROOT_ENV, None)
            found = repo_root()
        assert (found / 'go.mod').is_file()
        assert (found / 'feature-gates.txt').is_file()

    def test_both_markers_are_required(self) -> None:
        """`go.mod` alone is not a checkout: a vendored module has one.

        Accepting it would make a directory under `vendor/` answer as the root.
        """
        half = Path(tempfile.mkdtemp())
        (half / 'go.mod').write_text('')
        assert paths._discovered(half) is None

    def test_a_directory_with_both_markers_is_a_checkout(self) -> None:
        root = _checkout()
        assert paths._discovered(root) == root

    def test_it_is_found_from_any_depth(self) -> None:
        """The property the whole design exists for.

        `parents[2]` is right at one depth and silently wrong at every other.
        Walking up is right at all of them.
        """
        root = _checkout()
        deep = root / 'scripts' / 'le' / 'dev' / 'nested'
        deep.mkdir(parents=True)
        assert paths._discovered(deep) == root

    def test_a_tree_with_no_marker_yields_nothing(self) -> None:
        assert paths._discovered(Path(tempfile.mkdtemp())) is None


class TestExport(unittest.TestCase):
    """`le` settles the root once so every gate inherits one answer."""

    def test_export_writes_the_variable(self) -> None:
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(ROOT_ENV, None)
            found = repo_root(export=True)
            assert os.environ[ROOT_ENV] == str(found)

    def test_without_export_the_environment_is_untouched(self) -> None:
        """Reading the root must not be a side effect on the process."""
        with mock.patch.dict(os.environ, {}, clear=False):
            os.environ.pop(ROOT_ENV, None)
            repo_root()
            assert ROOT_ENV not in os.environ

    def test_exporting_a_named_root_keeps_it(self) -> None:
        with mock.patch.dict(os.environ, {ROOT_ENV: '/workspace'}):
            repo_root(export=True)
            assert os.environ[ROOT_ENV] == '/workspace'


def _counts_depth(source: str) -> bool:
    """Whether `source` rediscovers the root by counting directories, in CODE.

    Comments and string literals are stripped first. The check exists to stop
    the habit, and a comment explaining why a module no longer has the habit is
    the opposite of it: matching prose made the explanation the offence and
    made deleting the explanation the fix.

    `tokenize` rather than `ast`: the two names can be an arbitrary expression
    apart (`here = Path(__file__).parent` then `here.parents[1]` later), so
    what is wanted is "both names appear in executable text", not one shape.
    """
    kept: list[str] = []
    try:
        for token in tokenize.generate_tokens(io.StringIO(source).readline):
            if token.type in (tokenize.COMMENT, tokenize.STRING):
                continue
            kept.append(token.string)
    except (tokenize.TokenError, IndentationError, SyntaxError):
        # Unparseable is not a pass. Fall back to the whole text, which is
        # the stricter answer and the one the check had before.
        return 'parents[' in source and '__file__' in source
    code = ' '.join(kept)
    return 'parents' in code and '__file__' in code


class TestRelativeToRoot(unittest.TestCase):
    def test_a_path_inside_the_tree_loses_the_prefix(self) -> None:
        assert relative_to_root(paths.REPO_ROOT / 'scripts' / 'le') == 'scripts/le'

    def test_a_path_outside_keeps_its_absolute_form(self) -> None:
        """Shortening it would be a lie about where it is."""
        assert relative_to_root(Path('/etc/hosts')) == '/etc/hosts'


class TestNoModuleCountsItsOwnDepth(unittest.TestCase):
    """No file under `le` may rediscover the root by counting directories.

    That is the habit this module exists to replace, and it reappears the
    moment somebody writes `parents[2]` in a new gate. A test file is exempt:
    each one puts `scripts/` on `sys.path` before it can import anything at
    all, which is a bootstrap rather than a root.
    """

    def test_only_paths_itself_walks_from_file(self) -> None:
        offenders = []
        for path in (paths.REPO_ROOT / 'scripts' / 'le').rglob('*.py'):
            if path.name.endswith('_test.py') or path.name == 'paths.py':
                continue
            if _counts_depth(path.read_text(encoding='utf-8', errors='replace')):
                offenders.append(str(path.relative_to(paths.REPO_ROOT)))
        assert not offenders, f'these count their own depth: {offenders}'

    def test_a_comment_about_the_habit_is_not_the_habit(self) -> None:
        """The check reads CODE. Prose that names the pattern must not trip it.

        This was a live false positive: a module that had just been converted
        to REPO_ROOT explained the conversion in a comment quoting the
        `parents[2]` it replaced, and the grep-based check failed it. The
        cheapest way to green was to delete the explanation, which would have
        removed the one line telling the next reader why the module does not
        count directories.
        """
        assert not _counts_depth(
            '# the old version used Path(__file__).parents[2] and this does not\n'
            'from le.paths import REPO_ROOT\n'
        )
        assert not _counts_depth('"""Docstring naming __file__ and parents[2]."""\n')

    def test_it_still_catches_the_real_thing(self) -> None:
        """Non-vacuity: the relaxation above must not disarm the check."""
        assert _counts_depth('root = Path(__file__).resolve().parents[2]\n')
        assert _counts_depth('here = Path(__file__).parent\nroot = here.parents[1]\n')


if __name__ == '__main__':
    unittest.main()
