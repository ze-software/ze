#!/usr/bin/env python3
"""Tests for language-server plugin detection.

The gap these exist for was real and it was invisible: on this machine `gopls`
and `pyright` were both installed, both answered when run directly, and every
LSP call on a `.py` file was refused anyway, because the harness had the
`gopls-lsp` plugin and not the `pyright-lsp` one. A binary probe cannot see
that, and neither can an answering probe. Only the installed-plugin record can.
"""

from __future__ import annotations

import io
import json
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application import setup
from le.console import State
from le.devtools import editor


def _written(text: str) -> Path:
    """A file holding `text`, in a directory this test run owns."""
    path = Path(tempfile.mkdtemp()) / 'installed_plugins.json'
    path.write_text(text)
    return path


def _record(*qualified: str) -> Path:
    """A written installed-plugins record naming exactly these plugins."""
    installs: dict[str, list[object]] = {name: [] for name in qualified}
    return _written(json.dumps({'version': 2, 'plugins': installs}))


class TestInstalledPlugins(unittest.TestCase):
    def test_reads_the_recorded_names(self) -> None:
        record = _record('gopls-lsp@claude-plugins-official')
        assert editor.installed_plugins(record) == {'gopls-lsp@claude-plugins-official'}

    def test_a_missing_record_is_nothing_installed(self) -> None:
        """A machine being set up may have no record yet, which is not an error."""
        assert editor.installed_plugins(Path('/nonexistent/plugins.json')) == frozenset()

    def test_an_unreadable_record_is_nothing_installed(self) -> None:
        assert editor.installed_plugins(_written('not json at all')) == frozenset()

    def test_a_record_without_a_plugins_map_is_nothing_installed(self) -> None:
        assert editor.installed_plugins(_written('{"version": 2}')) == frozenset()


class TestMissingLspPlugins(unittest.TestCase):
    def test_the_exact_state_this_machine_was_in(self) -> None:
        """gopls-lsp installed, pyright-lsp not: Go answered, Python was refused."""
        missing = editor.missing_lsp_plugins(_record('gopls-lsp@claude-plugins-official'))
        names = [plugin.plugin for plugin in missing]
        assert names == ['pyright-lsp']

    def test_nothing_installed_means_every_plugin_is_missing(self) -> None:
        missing = editor.missing_lsp_plugins(_record())
        assert len(missing) == len(editor.LSP_PLUGINS)

    def test_all_installed_means_none_missing(self) -> None:
        record = _record(*(plugin.qualified for plugin in editor.LSP_PLUGINS))
        assert editor.missing_lsp_plugins(record) == []


class TestPluginNaming(unittest.TestCase):
    def test_python_is_covered_for_both_extensions(self) -> None:
        pyright = next(p for p in editor.LSP_PLUGINS if p.plugin == 'pyright-lsp')
        assert '.py' in pyright.extensions
        assert '.pyi' in pyright.extensions

    def test_the_install_command_is_the_slash_command(self) -> None:
        """`claude plugin ...` from a shell inside a session does not return.

        Measured: it hangs until killed. The reader of this line is usually in
        a session, so the command offered must be the one that works there.
        """
        pyright = next(p for p in editor.LSP_PLUGINS if p.plugin == 'pyright-lsp')
        assert pyright.install_command.startswith('/plugin install')
        assert 'claude plugin' not in pyright.install_command

    def test_every_plugin_is_qualified_by_its_marketplace(self) -> None:
        for plugin in editor.LSP_PLUGINS:
            assert plugin.qualified.endswith(f'@{editor.OFFICIAL}')


class TestVisitLspPlugin(unittest.TestCase):
    def test_an_installed_plugin_is_present(self) -> None:
        plugin = editor.LSP_PLUGINS[0]
        with (
            mock.patch.object(editor, 'missing_lsp_plugins', return_value=[]),
            mock.patch.object(setup, 'missing_lsp_plugins', return_value=[]),
            redirect_stdout(io.StringIO()),
        ):
            outcome = setup._visit_lsp_plugin(plugin)
        assert outcome.state is State.PRESENT

    def test_a_missing_plugin_is_pending_not_missing(self) -> None:
        """Nothing this program runs can install it, and PENDING says exactly that.

        It must still block: the silent version of this cost weeks of
        whole-file reads with a working server sitting on PATH.
        """
        plugin = next(p for p in editor.LSP_PLUGINS if p.plugin == 'pyright-lsp')
        buffer = io.StringIO()
        with (
            mock.patch.object(setup, 'missing_lsp_plugins', return_value=[plugin]),
            redirect_stdout(buffer),
        ):
            outcome = setup._visit_lsp_plugin(plugin)
        assert outcome.state is State.PENDING
        assert outcome.state.blocking
        # The report must carry the command, not just the complaint.
        assert '/plugin install pyright-lsp' in buffer.getvalue()


if __name__ == '__main__':
    unittest.main()
