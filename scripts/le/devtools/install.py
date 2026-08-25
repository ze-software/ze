"""Putting a tool on the machine, and vendoring the Go dependencies.

One `Installer` per run rather than module-level state. The shell version kept
a `global _apt_updated`, which is a fact about one run stored where every run
shares it: a test that installed anything left the flag set for the next one.
"""

from __future__ import annotations

import os
import platform
from dataclasses import dataclass, field
from pathlib import Path

from le.console import echo
from le.devtools.tools import PackageManager, Tool
from le.paths import REPO_ROOT
from le.process import Privilege, privilege, run, run_privileged, which

__all__ = ['Installer', 'detect_package_manager', 'vendor_go_deps']


def detect_package_manager() -> PackageManager | None:
    """The package manager this host installs system packages with.

    Gated on the PLATFORM, not on what happens to be on PATH. macOS takes
    Homebrew and Linux takes apt, which is what `detect_os` did before the
    port. An earlier version of this asked PATH alone and returned BREW first
    on any platform, so a Linux box carrying Linuxbrew would have installed
    system packages through it instead of apt -- a silent change of which
    package manager owns the machine.

    None means neither route exists, which is not a failure of this program:
    it is a platform it cannot install for, and the caller prints the manual
    list.
    """
    system = platform.system()
    if system == 'Darwin':
        return PackageManager.BREW if which('brew') else None
    if system == 'Linux':
        return PackageManager.APT if which('apt-get') else None
    return None


@dataclass
class Installer:
    """Installs tools, remembering what it already did this run.

    `manager` is the system package manager; the `go install` and `pipx` routes
    do not use it and work on either platform.
    """

    manager: PackageManager
    _apt_updated: bool = field(default=False, init=False)

    def install(self, tool: Tool) -> bool:
        """Put `tool` on the machine by whichever route it declares.

        Order matters: `go install` and pipx come first because they work the
        same on both platforms, so a tool declaring one gets the same version
        everywhere. The system package manager is the fallback.
        """
        if tool.go_install:
            return self._go_install(tool, tool.go_install)
        if tool.pipx_install:
            return self._pipx_install(tool, tool.pipx_install)

        package = tool.package_for(self.manager)
        if package is None:
            if tool.note:
                echo(f'  SKIP {tool.name}: {tool.note}')
            return False

        if self.manager is PackageManager.BREW:
            return self._brew_install(tool, package)
        return self._apt_install(tool, package)

    def _go_install(self, tool: Tool, target: str) -> bool:
        if not which('go'):
            echo(f'  SKIP {tool.name}: go not available yet')
            return False
        echo(f'  CGO_ENABLED=0 go install {target}')
        env = os.environ.copy()
        env['CGO_ENABLED'] = '0'
        result = run(['go', 'install', target], env=env)
        if not result.ok:
            echo(f'  FAIL {tool.name}: {result.complaint()}')
            return False
        return True

    def _pipx_install(self, tool: Tool, target: str) -> bool:
        if not which('pipx'):
            echo(f'  SKIP {tool.name}: pipx not available yet')
            return False
        echo(f'  pipx install {target}')
        result = run(['pipx', 'install', '--force', target])
        if not result.ok:
            echo(f'  FAIL {tool.name}: {result.complaint()}')
            return False
        return True

    def _brew_install(self, tool: Tool, package: str) -> bool:
        echo(f'  brew install {package}')
        result = run(['brew', 'install', package])
        if not result.ok:
            echo(f'  FAIL {tool.name}: {result.complaint()}')
            return False
        return True

    def _apt_install(self, tool: Tool, package: str) -> bool:
        """Install one Debian package, or print the command when root is out of reach.

        `DEBIAN_FRONTEND=noninteractive` rides in the argv rather than in the
        environment because sudo resets it (`env_reset` is the sudoers
        default), so an exported value does not reach apt-get. Without it a
        package carrying a debconf prompt stops the run dead.
        """
        # The echoed line must be copyable on the box that printed it: a root
        # container usually has no sudo at all, so naming one there gives the
        # reader a command they cannot run.
        mode = privilege()
        manual = f'{mode.prefix}apt-get install -y {package}'
        if mode is Privilege.NONE:
            echo(f'  Run: {manual}')
            return False

        self._apt_update()

        ok, detail = run_privileged(
            ['env', 'DEBIAN_FRONTEND=noninteractive', 'apt-get', 'install', '-y', package]
        )
        if not ok:
            echo(f'  FAIL {tool.name}: {detail}')
            echo(f'  Run: {manual}')
            return False
        return True

    def _apt_update(self) -> None:
        """One `apt-get update` per run, taken before the first install.

        A container image ships no package lists at all, and an install against
        an empty list fails with "Unable to locate package <x>", which reads as
        a wrong package name rather than a missing index. Per-tool it would be
        ~20s of network each, for an index that does not change during one run.

        One attempt, success or not: a stale index still installs most
        packages, and re-running a failing update per tool helps nobody.
        """
        if self._apt_updated:
            return
        self._apt_updated = True
        ok, detail = run_privileged(['apt-get', 'update'])
        if not ok:
            echo(f'  WARN apt-get update: {detail}')


def vendor_go_deps(root: Path = REPO_ROOT) -> bool:
    """Bring vendor/ back in step with go.mod."""
    if not which('go'):
        echo('  SKIP vendoring: go not available')
        return False
    echo('  go mod tidy && go mod vendor')
    for argv in (['go', 'mod', 'tidy'], ['go', 'mod', 'vendor']):
        result = run(argv, cwd=root)
        if not result.ok:
            echo(f'  FAIL {" ".join(argv)}: {result.complaint()}')
            return False
    return True
