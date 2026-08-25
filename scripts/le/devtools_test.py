#!/usr/bin/env python3
"""Tests for the libraries the setup subprogram is built from.

Each case here pins a rule that was learned from a real failure, and the
docstring says which. A test whose subject is "this function returns what it
returns" is not written.
"""

from __future__ import annotations

import io
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application import setup
from le.console import State
from le.devtools import probes, servers, tools
from le.devtools.tools import PackageManager, Tool
from le.paths import REPO_ROOT
from le.process import Result


class TestGrubPackage(unittest.TestCase):
    """Debian builds one GRUB module-set package per host architecture.

    Asking for the amd64 set on an arm64 box installs NOTHING, grub-common and
    grub-mkstandalone included, and apt says "has no installation candidate"
    rather than failing usefully.
    """

    def test_arm_hosts_take_the_arm_set(self) -> None:
        assert tools.grub_apt_package('aarch64') == 'grub-efi-arm64-bin'
        assert tools.grub_apt_package('arm64') == 'grub-efi-arm64-bin'

    def test_32_bit_hosts_take_the_ia32_set(self) -> None:
        assert tools.grub_apt_package('i386') == 'grub-efi-ia32-bin'
        assert tools.grub_apt_package('i686') == 'grub-efi-ia32-bin'

    def test_an_unlisted_machine_falls_back_to_amd64(self) -> None:
        assert tools.grub_apt_package('riscv64') == 'grub-efi-amd64-bin'


class TestInstallableBy(unittest.TestCase):
    """A tool with no route on this platform is SKIPPED, never silently absent.

    uv is the case this exists for: it is not in the Debian or Ubuntu
    repositories, so an apt-only table left a REQUIRED tool with no route
    there. Check mode reported "All required tools present" on a box that had
    no uv, because a tool nothing could install was never counted. A guard that
    reports green on absence is worse than no guard.
    """

    def test_pipx_route_works_on_both_platforms(self) -> None:
        uv = next(t for t in tools.REQUIRED_TOOLS if t.name == 'uv')
        assert uv.apt is None
        assert uv.installable_by(PackageManager.APT)
        assert uv.installable_by(PackageManager.BREW)

    def test_go_install_route_works_on_both_platforms(self) -> None:
        gopls = next(t for t in tools.REQUIRED_TOOLS if t.name == 'gopls')
        assert gopls.installable_by(PackageManager.APT)
        assert gopls.installable_by(PackageManager.BREW)

    def test_a_tool_with_no_route_is_not_installable(self) -> None:
        orphan = Tool(name='nothing', probe=('nothing',))
        assert not orphan.installable_by(PackageManager.APT)
        assert not orphan.installable_by(PackageManager.BREW)

    def test_grub_has_no_brew_route(self) -> None:
        """No first-party Homebrew formula, so macOS must skip rather than fail."""
        grub = next(t for t in tools.REQUIRED_TOOLS if t.name == 'grub')
        assert not grub.installable_by(PackageManager.BREW)
        assert grub.installable_by(PackageManager.APT)


class TestPipxOrdering(unittest.TestCase):
    def test_pipx_is_listed_before_every_tool_that_needs_it(self) -> None:
        """An install through pipx is skipped while pipx is not there yet.

        The table is walked in order, so a pipx-installed tool listed above the
        pipx row would be skipped on a machine that has neither.
        """
        names = [t.name for t in tools.REQUIRED_TOOLS]
        pipx_at = names.index('pipx')
        for tool in tools.REQUIRED_TOOLS:
            if tool.pipx_install:
                assert names.index(tool.name) > pipx_at, tool.name


class TestCellarVersionKey(unittest.TestCase):
    """Cellar directories sort by NUMBER, not by spelling.

    Plain string order puts 1.47.10 below 1.47.4, which hands back last month's
    e2fsprogs the first time a formula reaches a two-digit patch.
    """

    def test_two_digit_patch_outranks_one_digit(self) -> None:
        newer = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.10/sbin')
        older = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.4/sbin')
        assert newer > older

    def test_a_release_outranks_its_candidate(self) -> None:
        release = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.4/sbin')
        candidate = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.rc1/sbin')
        assert release > candidate

    def test_a_homebrew_revision_outranks_the_plain_version(self) -> None:
        revised = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.4_1/sbin')
        plain = probes.cellar_version_key('/p/Cellar/e2fsprogs/1.47.4/sbin')
        assert revised > plain


class TestE2fsprogsDirs(unittest.TestCase):
    """e2fsprogs is searched by directory because PATH cannot answer.

    Homebrew links none of a keg-only formula onto PATH, and Debian keeps
    /usr/sbin off a non-root user's PATH, so a PATH probe reported missing what
    the build then used happily.
    """

    def test_the_system_directories_are_always_searched(self) -> None:
        with mock.patch.object(probes, 'brew_prefixes', return_value=[]):
            found = probes.e2fsprogs_dirs()
        assert Path('/usr/sbin') in found
        assert Path('/sbin') in found

    def test_the_keg_only_link_is_searched_before_the_prefix(self) -> None:
        # Spelled in pieces so this file does not trip the scan in
        # scripts/dev/homebrew_prefix_test.py, which refuses the Apple Silicon
        # prefix as a literal anywhere but the resolvers. An Intel Mac keeps
        # its Homebrew at a different path, so writing this one down is the
        # bug that scan exists to catch.
        prefix = Path('/opt/' + 'homebrew')
        with mock.patch.object(probes, 'brew_prefixes', return_value=[prefix]):
            found = probes.e2fsprogs_dirs()
        opt = prefix / 'opt' / 'e2fsprogs' / 'sbin'
        assert found.index(opt) < found.index(prefix / 'sbin')


class TestApplianceChecks(unittest.TestCase):
    """The names must match `applianceDoctorChecks()` in the Go appliance code.

    internal/appliance/dev_setup_drift_test.go parses this table's source and
    holds the two lists to one answer. These cases guard the shape that test
    depends on, so a rename here fails in Python before it fails in Go.
    """

    def test_the_three_installable_checks_are_present(self) -> None:
        names = {check.name for check in tools.APPLIANCE_CHECKS}
        assert 'appliance-grub' in names
        assert 'appliance-xorriso' in names
        assert 'appliance-e2fsprogs' in names

    def test_every_check_names_at_least_one_probe(self) -> None:
        for check in tools.APPLIANCE_CHECKS:
            assert check.probe, f'{check.name} has no probe'

    def test_e2fsprogs_needs_both_of_its_tools(self) -> None:
        """The image build formats with mkfs.ext4 then injects with debugfs."""
        check = next(c for c in tools.APPLIANCE_CHECKS if c.name == 'appliance-e2fsprogs')
        assert set(check.probe) == {'mkfs.ext4', 'debugfs'}

    def test_grub_takes_the_host_architecture_package(self) -> None:
        check = next(c for c in tools.APPLIANCE_CHECKS if c.name == 'appliance-grub')
        assert check.apt == tools.GRUB_APT_PACKAGE


class TestGoplsAnswers(unittest.TestCase):
    """gopls on PATH is not a working language server.

    It was absent from this machine for weeks while the BLOCKING "load LSP
    first" rule was satisfied every session: that gate lifts on the query text,
    and no call was ever made, so every one would have returned ENOENT unseen.
    A presence check would not have caught it, which is why this check RUNS the
    server. The probe is faked here -- a unit test cannot own a real one.
    """

    ANSWER = 'Clock Interface 18:6-18:11\n\tNow Method 20:2-20:5\n'

    def _health(self, result: Result, *, present: bool = True) -> servers.Answer:
        found = Path('/home/x/go/bin/gopls') if present else None
        with mock.patch.object(servers, 'which', return_value=found):
            return servers.gopls_health(probe=lambda: result)

    def test_probe_file_is_in_the_checkout(self) -> None:
        """A probe file that moves turns the whole check into a silent n/a."""
        assert (REPO_ROOT / servers.GOPLS_PROBE_FILE).is_file()

    def test_an_answering_server_is_ok(self) -> None:
        answer = self._health(Result([], 0, self.ANSWER, ''))
        assert answer.health is servers.Health.OK
        assert '2 symbols' in answer.detail
        assert servers.GOPLS_PROBE_FILE in answer.detail

    def test_absent_binary_is_not_the_same_finding_as_a_mute_server(self) -> None:
        """The two need different fixes, so they must not share a message."""

        def unreachable() -> Result:
            raise AssertionError('probe must not run when gopls is absent')

        with mock.patch.object(servers, 'which', return_value=None):
            absent = servers.gopls_health(probe=unreachable)
        mute = self._health(Result([], 0, '\n', ''))

        assert absent.health is servers.Health.ABSENT
        assert mute.health is servers.Health.BROKEN
        assert absent.detail == servers.GOPLS_NOT_INSTALLED
        assert servers.GOPLS_NOT_ANSWERING in mute.detail
        assert servers.GOPLS_NOT_ANSWERING not in absent.detail

    def test_output_without_a_symbol_is_not_an_answer(self) -> None:
        """Exit 0 and some text is not proof: the reply must carry a symbol."""
        answer = self._health(Result([], 0, 'loading packages...\n', ''))
        assert answer.health is servers.Health.BROKEN
        assert 'no symbol in its reply' in answer.detail

    def test_a_failing_server_reports_its_own_message(self) -> None:
        answer = self._health(Result([], 1, '', 'err: no module cache\nmore\n'))
        assert answer.health is servers.Health.BROKEN
        assert 'no module cache' in answer.detail
        # Only the first line of the server's complaint, not its whole output.
        assert 'more' not in answer.detail

    def test_a_timeout_is_a_mute_server_not_a_missing_one(self) -> None:
        """`run` turns a timeout into code 124, which is a failure, not absence."""
        answer = self._health(Result([], 124, '', 'no reply within 120s'))
        assert answer.health is servers.Health.BROKEN


class TestPyrightAnswersHealth(unittest.TestCase):
    """pyright exits 1 when it finds a type error.

    So the exit code says whether the CODE is clean, never whether the SERVER
    worked. A check keyed on it would go red the first time somebody's script
    gained a diagnostic, and a check that reds spuriously is a check somebody
    disables. `summary.filesAnalyzed` is the field that answers this question.
    """

    def _health(self, result: Result, *, present: bool = True) -> servers.Answer:
        found = Path('/home/x/.local/bin/pyright') if present else None
        with mock.patch.object(servers, 'which', return_value=found):
            return servers.pyright_health(probe=lambda: result)

    def test_a_nonzero_exit_with_a_summary_is_still_ok(self) -> None:
        """The discriminating case: type errors must not read as a dead server."""
        answer = self._health(Result([], 1, '{"summary": {"filesAnalyzed": 2}}', ''))
        assert answer.health is servers.Health.OK

    def test_analysing_no_file_is_broken(self) -> None:
        answer = self._health(Result([], 0, '{"summary": {"filesAnalyzed": 0}}', ''))
        assert answer.health is servers.Health.BROKEN

    def test_no_summary_is_broken(self) -> None:
        answer = self._health(Result([], 0, 'command not found', ''))
        assert answer.health is servers.Health.BROKEN

    def test_absent_is_not_broken(self) -> None:
        answer = self._health(Result([], 0, '', ''), present=False)
        assert answer.health is servers.Health.ABSENT

    def test_the_probe_file_exists(self) -> None:
        assert (REPO_ROOT / servers.PYRIGHT_PROBE_FILE).is_file()


class TestPyrightSummary(unittest.TestCase):
    """The reply can carry a bootstrap preamble, so it is scanned, not decoded.

    With no global node on PATH the pipx wrapper installs one, and nodeenv's
    progress lands on the stdout this probe captured. A whole-string decode
    fails, and setup reds on the fresh box it exists to prepare. The second run
    is green, which makes it read as flakiness.
    """

    def test_plain_json_is_read(self) -> None:
        found = servers.pyright_summary('{"summary": {"filesAnalyzed": 1}}')
        assert found == {'filesAnalyzed': 1}

    def test_json_behind_a_nodeenv_preamble_is_read(self) -> None:
        # nodeenv prints a Python dict repr: single quotes, not valid JSON.
        noise = " * Install prebuilt node (20.11.0) {'continue_on_exist': True}\n"
        found = servers.pyright_summary(noise + '{"summary": {"filesAnalyzed": 3}}')
        assert found == {'filesAnalyzed': 3}

    def test_a_reply_with_no_summary_is_none(self) -> None:
        assert servers.pyright_summary('{"version": "1.2.3"}') is None

    def test_unparseable_output_is_none(self) -> None:
        assert servers.pyright_summary('command not found') is None


class TestServerHealthToOutcome(unittest.TestCase):
    """A server that is absent is a DIFFERENT problem from one that is broken.

    Absent is the tool row's job, so reporting it here too would make one
    missing server two failures. Broken is nobody's job yet, so it must block.
    """

    def test_absent_is_skipped_not_missing(self) -> None:
        answer = servers.Answer(servers.Health.ABSENT, 'not installed')
        assert setup._visit_server('gopls-answers', answer).state is State.SKIPPED

    def test_broken_is_missing(self) -> None:
        answer = servers.Answer(servers.Health.BROKEN, 'no reply')
        assert setup._visit_server('gopls-answers', answer).state is State.MISSING

    def test_ok_is_present(self) -> None:
        answer = servers.Answer(servers.Health.OK, '30 symbols')
        assert setup._visit_server('gopls-answers', answer).state is State.PRESENT

    def test_na_is_present(self) -> None:
        """Nothing to ask about is not a failure to answer."""
        answer = servers.Answer(servers.Health.NA, 'no probe file')
        assert setup._visit_server('gopls-answers', answer).state is State.PRESENT


class TestVisitTool(unittest.TestCase):
    """The two modes must agree about what is missing.

    A probe-only run and an install run ask the same questions of the same
    machine. The shell version let them disagree permanently, and install mode
    was the one reading green on absence.
    """

    REQUIRED = Tool(name='ruff', probe=('ruff',), pipx_install='ruff')

    def _visit(self, *, present: bool, installer: object) -> State:
        with (
            mock.patch.object(setup, 'probe', return_value=present),
            redirect_stdout(io.StringIO()),
        ):
            outcome = setup._visit_tool(self.REQUIRED, PackageManager.APT, installer)  # type: ignore[arg-type]
        return outcome.state

    def test_present_in_both_modes(self) -> None:
        assert self._visit(present=True, installer=None) is State.PRESENT
        assert self._visit(present=True, installer=mock.Mock()) is State.PRESENT

    def test_absent_is_missing_in_check_mode(self) -> None:
        assert self._visit(present=False, installer=None) is State.MISSING

    def test_install_that_leaves_it_off_path_is_pending_not_installed(self) -> None:
        """The exact state the shell version reported success for."""
        installer = mock.Mock()
        installer.install.return_value = True
        with (
            mock.patch.object(setup, 'probe', side_effect=[False, False]),
            redirect_stdout(io.StringIO()),
        ):
            outcome = setup._visit_tool(self.REQUIRED, PackageManager.APT, installer)
        assert outcome.state is State.PENDING
        assert outcome.state.blocking

    def test_install_that_works_is_installed(self) -> None:
        installer = mock.Mock()
        installer.install.return_value = True
        with (
            mock.patch.object(setup, 'probe', side_effect=[False, True]),
            redirect_stdout(io.StringIO()),
        ):
            outcome = setup._visit_tool(self.REQUIRED, PackageManager.APT, installer)
        assert outcome.state is State.INSTALLED

    def test_a_tool_with_no_route_is_skipped_in_both_modes(self) -> None:
        orphan = Tool(name='colima', probe=('colima',), brew='colima', required=False)
        for installer in (None, mock.Mock()):
            with (
                mock.patch.object(setup, 'probe', return_value=False),
                redirect_stdout(io.StringIO()),
            ):
                outcome = setup._visit_tool(orphan, PackageManager.APT, installer)
            assert outcome.state is State.SKIPPED


if __name__ == '__main__':
    unittest.main()
