"""Language-server plugins the agent's LSP tool needs, and whether they are installed.

The LSP tool does not carry a hardcoded set of languages. Each one arrives as a
plugin that tells the harness which server binary to run for which file
extensions, and a language with no plugin installed answers every call with
"No LSP server available for file type: <ext>".

That is the state Python was in on this machine while Go worked: `gopls-lsp` was
installed and `pyright-lsp` was not, so `.go` questions were answered and `.py`
questions were refused. Both servers were on PATH. Neither the binary probe nor
the answering probe in `servers.py` could see it, because the missing piece was
not the server -- it was the harness being told the server exists.

Two things this module deliberately does NOT do:

  It does not install the plugin. `claude plugin ...` does not return when it is
  run from inside a session -- measured, it hangs until killed -- so a setup
  program that shelled out to it would hang the run it exists to finish. It
  reports and prints the command instead, which is what the KVM group and every
  PATH fix already do.

  It does not write a project-local plugin to work around that. A plugin still
  has to be enabled to take effect, so the workaround would need the same manual
  step while adding a second definition of a plugin the marketplace already
  publishes.
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

__all__ = [
    'LSP_PLUGINS',
    'LspPlugin',
    'installed_plugins',
    'missing_lsp_plugins',
]

# Where the harness records what is installed, for every scope.
INSTALLED_PLUGINS = Path.home() / '.claude' / 'plugins' / 'installed_plugins.json'

# The marketplace the official language-server plugins are published in.
OFFICIAL = 'claude-plugins-official'


@dataclass(frozen=True)
class LspPlugin:
    """One language-server plugin, and what it makes answerable.

    `binary` is the server the plugin runs. It is listed so that a report can
    tell apart the two ways this fails: the plugin is missing, or the plugin is
    there and the binary it names is not. They have different fixes, and the
    tool table already installs the binaries.
    """

    plugin: str
    binary: str
    extensions: tuple[str, ...]
    why: str

    @property
    def qualified(self) -> str:
        """The name an install command takes."""
        return f'{self.plugin}@{OFFICIAL}'

    @property
    def install_command(self) -> str:
        """What a human runs to get it.

        The slash command, not the shell one: `claude plugin ...` from a shell
        inside a session does not return, and the reader of this line is
        usually in a session.
        """
        return f'/plugin install {self.qualified}'


LSP_PLUGINS: tuple[LspPlugin, ...] = (
    LspPlugin(
        plugin='gopls-lsp',
        binary='gopls',
        extensions=('.go',),
        why='every LSP call on a .go file is refused without it',
    ),
    LspPlugin(
        plugin='pyright-lsp',
        binary='pyright',
        extensions=('.py', '.pyi'),
        why=(
            'without it the LSP tool refuses every .py file and a session reads'
            ' whole scripts to find one symbol (ai/rules/context-economy.md)'
        ),
    ),
)


def installed_plugins(record: Path = INSTALLED_PLUGINS) -> frozenset[str]:
    """Every installed plugin, by its `name@marketplace` key.

    An unreadable or absent record yields the empty set rather than raising:
    this runs on a machine being set up, where the harness may never have
    written one, and "nothing installed" is the right reading of that.
    """
    try:
        loaded = json.loads(record.read_text())
    except (OSError, ValueError):
        return frozenset()
    plugins = loaded.get('plugins')
    if not isinstance(plugins, dict):
        return frozenset()
    return frozenset(str(name) for name in plugins)


def missing_lsp_plugins(record: Path = INSTALLED_PLUGINS) -> list[LspPlugin]:
    """The language-server plugins this machine does not have."""
    have = installed_plugins(record)
    return [plugin for plugin in LSP_PLUGINS if plugin.qualified not in have]
