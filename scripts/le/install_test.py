#!/usr/bin/env python3
"""Tests for putting a tool on the machine.

Ported from scripts/dev/dev_setup_test.py when `le setup` replaced that script.

The class these belong to: macOS ran `brew install` while Linux printed
`sudo apt-get install ...` and returned False, so one setup run prepared the
machine and the other produced homework. Every tool row already carried its apt
package.
"""

from __future__ import annotations

import io
import os
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le import process
from le.devtools import install
from le.devtools.install import Installer, detect_package_manager
from le.devtools.tools import PackageManager, Tool
from le.process import Privilege, Result

XORRISO = Tool(name='xorriso', probe=('xorriso',), brew='xorriso', apt='xorriso')
GOPLS = Tool(name='gopls', probe=('gopls',), go_install='golang.org/x/tools/gopls@latest')
RUFF = Tool(name='ruff', probe=('ruff',), pipx_install='ruff')


class TestDetectPackageManager(unittest.TestCase):
    """Which manager, asked of the machine rather than of the platform name.

    A prefix-free path on purpose: this only asks whether `brew` was found,
    never where, and naming a prefix here would be a second place that believes
    Homebrew lives at one (scripts/dev/homebrew_prefix_test.py).
    """

    def _detect(self, found: str | None) -> PackageManager | None:
        with mock.patch.object(
            install, 'which', side_effect=lambda name: Path(name) if found == name else None
        ):
            return detect_package_manager()

    def test_brew_is_taken_on_macos(self) -> None:
        with mock.patch('le.devtools.install.platform.system', return_value='Darwin'):
            assert self._detect('brew') is PackageManager.BREW

    def test_apt_is_taken_on_linux(self) -> None:
        with mock.patch('le.devtools.install.platform.system', return_value='Linux'):
            assert self._detect('apt-get') is PackageManager.APT

    def test_neither_is_no_manager(self) -> None:
        """Not a failure of this program: a platform it cannot install for."""
        for system in ('Darwin', 'Linux'):
            with mock.patch('le.devtools.install.platform.system', return_value=system):
                assert self._detect(None) is None

    def test_an_unsupported_platform_has_no_manager(self) -> None:
        with mock.patch('le.devtools.install.platform.system', return_value='Windows'):
            assert self._detect('brew') is None

    def test_linuxbrew_does_not_displace_apt(self) -> None:
        """The discriminating case, and why detection is gated on the PLATFORM.

        A Linux box carrying Linuxbrew has both binaries. Asking PATH alone
        would answer BREW and hand that machine's system packages to Homebrew;
        `detect_os` answered apt on Linux whatever else was installed.
        """
        with (
            mock.patch('le.devtools.install.platform.system', return_value='Linux'),
            mock.patch.object(install, 'which', return_value=Path('/anywhere')),
        ):
            assert detect_package_manager() is PackageManager.APT


class TestAptInstalls(unittest.TestCase):
    def _install(
        self,
        package: str = 'xorriso',
        *,
        mode: Privilege = Privilege.SUDO,
        code: int = 0,
    ) -> tuple[bool, list[list[str]], str]:
        buffer = io.StringIO()
        installer = Installer(PackageManager.APT)
        with (
            mock.patch.object(process, 'privilege', return_value=mode),
            mock.patch.object(install, 'privilege', return_value=mode),
            mock.patch.object(process, 'run') as run,
            redirect_stdout(buffer),
        ):
            run.return_value = Result([], code, '', 'E: nope\n')
            tool = Tool(name=package, probe=(package,), apt=package)
            ok = installer.install(tool)
        return ok, [call.args[0] for call in run.call_args_list], buffer.getvalue()

    def test_the_install_carries_a_noninteractive_frontend(self) -> None:
        """sudo resets the environment, so an exported value never arrives.

        A package with a debconf prompt stops the run dead without this, and
        the tool rows already include ones that have had them (ppp, docker.io).
        """
        _, calls, _ = self._install()
        assert calls[-1] == [
            'sudo',
            '-n',
            'env',
            'DEBIAN_FRONTEND=noninteractive',
            'apt-get',
            'install',
            '-y',
            'xorriso',
        ]

    def test_update_runs_once_before_the_first_install(self) -> None:
        """A container image ships no package lists at all.

        Without the update, apt-get answers "Unable to locate package", which
        reads as a wrong package name rather than a missing index.

        One Installer per run holds this, where the shell version used module
        state: a test that installed anything left the flag set for the next.
        """
        installer = Installer(PackageManager.APT)
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.SUDO),
            mock.patch.object(install, 'privilege', return_value=Privilege.SUDO),
            mock.patch.object(process, 'run', return_value=Result([], 0, '', '')) as run,
            redirect_stdout(io.StringIO()),
        ):
            installer.install(Tool(name='a', probe=('a',), apt='a'))
            first = [call.args[0] for call in run.call_args_list]
            run.reset_mock()
            installer.install(Tool(name='b', probe=('b',), apt='b'))
            second = [call.args[0] for call in run.call_args_list]

        assert first[0] == ['sudo', '-n', 'apt-get', 'update']
        assert ['sudo', '-n', 'apt-get', 'update'] not in second

    def test_a_fresh_installer_updates_again(self) -> None:
        """The guard is per run, not per process."""
        _, first, _ = self._install('xorriso')
        _, second, _ = self._install('e2fsprogs')
        assert first[0] == ['sudo', '-n', 'apt-get', 'update']
        assert second[0] == ['sudo', '-n', 'apt-get', 'update']

    def test_a_stale_index_still_installs(self) -> None:
        """`apt-get update` failing is a warning, not the end of the install.

        A machine behind a broken mirror still has the index it fetched last
        week, and the package is usually in it.
        """
        buffer = io.StringIO()
        installer = Installer(PackageManager.APT)
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.SUDO),
            mock.patch.object(install, 'privilege', return_value=Privilege.SUDO),
            mock.patch.object(process, 'run') as run,
            redirect_stdout(buffer),
        ):
            run.side_effect = [Result([], 1, '', 'E: mirror\n'), Result([], 0, '', '')]
            assert installer.install(XORRISO)
        assert 'WARN apt-get update' in buffer.getvalue()

    def test_no_root_prints_the_command_and_runs_nothing(self) -> None:
        """The old behaviour, kept as the fallback rather than the rule."""
        ok, calls, said = self._install(mode=Privilege.NONE)
        assert not ok
        assert calls == []
        assert 'sudo apt-get install -y xorriso' in said

    def test_a_failed_install_says_what_to_run(self) -> None:
        ok, _, said = self._install(code=100)
        assert not ok
        assert 'sudo apt-get install -y xorriso' in said

    def test_the_echoed_command_is_copyable_on_the_box_that_printed_it(self) -> None:
        """A root container usually has no sudo binary at all, so naming one
        gives the reader a command they cannot run."""
        _, _, said = self._install(mode=Privilege.ROOT, code=100)
        assert 'Run: apt-get install -y xorriso' in said
        assert 'sudo' not in said


class TestGoInstall(unittest.TestCase):
    def test_the_go_route_is_taken_on_every_package_manager(self) -> None:
        """Asserting `installable_by` here would prove nothing: it returns True
        for any tool carrying `go_install`, before it consults brew or apt.

        What CAN fail is the command that runs, so that is what is checked --
        give gopls an apt package and drop `go_install`, and this becomes a
        printed sudo line and a False.
        """
        for manager in (PackageManager.APT, PackageManager.BREW):
            installer = Installer(manager)
            with (
                mock.patch.object(install, 'which', return_value=Path('/usr/bin/go')),
                mock.patch.dict(os.environ, {'ZE_TEST_ENV': 'kept', 'CGO_ENABLED': '1'}),
                mock.patch.object(install, 'run', return_value=Result([], 0, '', '')) as run,
                redirect_stdout(io.StringIO()),
            ):
                assert installer.install(GOPLS)
                # The parent's own environment must not be edited.
                assert os.environ['CGO_ENABLED'] == '1'

            assert run.call_args.args[0] == ['go', 'install', 'golang.org/x/tools/gopls@latest']
            child = run.call_args.kwargs['env']
            assert child['ZE_TEST_ENV'] == 'kept'
            assert child['CGO_ENABLED'] == '0'

    def test_no_go_yet_is_a_skip_not_a_failure(self) -> None:
        """go is the first row; a tool needing it before it exists must say so."""
        installer = Installer(PackageManager.APT)
        buffer = io.StringIO()
        with (
            mock.patch.object(install, 'which', return_value=None),
            redirect_stdout(buffer),
        ):
            assert not installer.install(GOPLS)
        assert 'go not available yet' in buffer.getvalue()


class TestPipxInstall(unittest.TestCase):
    def test_no_pipx_yet_is_a_skip(self) -> None:
        """pipx must be listed above every tool that installs through it."""
        installer = Installer(PackageManager.APT)
        buffer = io.StringIO()
        with (
            mock.patch.object(install, 'which', return_value=None),
            redirect_stdout(buffer),
        ):
            assert not installer.install(RUFF)
        assert 'pipx not available yet' in buffer.getvalue()

    def test_the_install_forces_a_reinstall(self) -> None:
        """`--force` so a half-installed venv is replaced rather than skipped."""
        installer = Installer(PackageManager.APT)
        with (
            mock.patch.object(install, 'which', return_value=Path('/usr/bin/pipx')),
            mock.patch.object(install, 'run', return_value=Result([], 0, '', '')) as run,
            redirect_stdout(io.StringIO()),
        ):
            assert installer.install(RUFF)
        assert run.call_args.args[0] == ['pipx', 'install', '--force', 'ruff']


class TestNoRoute(unittest.TestCase):
    def test_a_tool_with_no_package_for_this_manager_is_skipped_with_its_note(self) -> None:
        grub = Tool(
            name='grub',
            probe=('grub-mkstandalone',),
            brew=None,
            apt='grub-efi-amd64-bin',
            note='no first-party Homebrew formula',
        )
        installer = Installer(PackageManager.BREW)
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            assert not installer.install(grub)
        assert 'no first-party Homebrew formula' in buffer.getvalue()


class TestVendorGoDeps(unittest.TestCase):
    def test_both_commands_run_in_order(self) -> None:
        with (
            mock.patch.object(install, 'which', return_value=Path('/usr/bin/go')),
            mock.patch.object(install, 'run', return_value=Result([], 0, '', '')) as run,
            redirect_stdout(io.StringIO()),
        ):
            assert install.vendor_go_deps()
        assert [call.args[0] for call in run.call_args_list] == [
            ['go', 'mod', 'tidy'],
            ['go', 'mod', 'vendor'],
        ]

    def test_a_failing_tidy_stops_before_vendor(self) -> None:
        """Vendoring against a go.mod tidy could not fix writes a wrong tree."""
        with (
            mock.patch.object(install, 'which', return_value=Path('/usr/bin/go')),
            mock.patch.object(install, 'run', return_value=Result([], 1, '', 'boom')) as run,
            redirect_stdout(io.StringIO()),
        ):
            assert not install.vendor_go_deps()
        assert len(run.call_args_list) == 1

    def test_no_go_is_a_skip(self) -> None:
        buffer = io.StringIO()
        with mock.patch.object(install, 'which', return_value=None), redirect_stdout(buffer):
            assert not install.vendor_go_deps()
        assert 'go not available' in buffer.getvalue()


if __name__ == '__main__':
    unittest.main()
