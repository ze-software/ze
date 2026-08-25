#!/usr/bin/env python3
"""End-to-end tests for `le setup`, and the probes that need a whole tool row.

Ported from scripts/dev/dev_setup_test.py when `le setup` replaced that script.

These run the program rather than a function, because the property they check
is a property of the run: that a probe-only pass changes nothing, that an
unsupported platform says so and exits nonzero, and that the report a reader
sees is the report the exit code was derived from.
"""

from __future__ import annotations

import io
import subprocess
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application import setup
from le.devtools import probes
from le.devtools.tools import REQUIRED_TOOLS, Tool
from le.process import Result

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPTS = REPO_ROOT / 'scripts'


def _run_setup(*args: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[bytes]:
    """Run the subprogram standalone, the way a developer would."""
    environment = {'PATH': '/usr/bin:/bin', 'HOME': str(Path.home())}
    if env is not None:
        environment.update(env)
    environment['PYTHONPATH'] = str(SCRIPTS)
    return subprocess.run(
        [sys.executable, '-m', 'le.application.setup', *args],
        cwd=str(REPO_ROOT),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        env=environment,
        check=False,
    )


class TestCheckMode(unittest.TestCase):
    def test_check_mode_runs_and_reports(self) -> None:
        result = _run_setup('--check', env={'PATH': __import__('os').environ.get('PATH', '')})
        output = result.stdout.decode()
        assert 'Ze dev setup' in output
        assert '[present' in output

    def test_check_mode_does_not_install(self) -> None:
        """A probe changes nothing. Nothing else in this file can prove that."""
        result = _run_setup('--check', env={'PATH': __import__('os').environ.get('PATH', '')})
        output = result.stdout.decode()
        assert '[installed' not in output
        assert 'brew install' not in output
        assert 'pipx install' not in output
        assert 'apt-get install' not in output

    def test_a_stripped_path_finds_nothing_and_exits_nonzero(self) -> None:
        result = _run_setup('--check')
        assert result.returncode != 0


class TestUnsupportedPlatform(unittest.TestCase):
    def test_unsupported_prints_the_list_and_exits_nonzero(self) -> None:
        """No brew and no apt: say what is needed rather than failing silently."""
        result = _run_setup('--check', env={'PATH': ''})
        assert result.returncode != 0
        output = result.stdout.decode()
        assert 'Unsupported platform' in output
        assert 'Manual installation' in output

    def test_the_manual_list_names_every_tool_and_its_probes(self) -> None:
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            setup._print_manual_list()
        listed = buffer.getvalue()
        assert 'Required:' in listed
        assert 'Optional:' in listed
        for tool in REQUIRED_TOOLS:
            assert tool.name in listed


class TestStandaloneMatchesDispatch(unittest.TestCase):
    def test_the_module_runs_on_its_own(self) -> None:
        """`python3 -m le.application.setup --help` must work with no install."""
        result = subprocess.run(
            [sys.executable, '-m', 'le.application.setup', '--help'],
            cwd=str(REPO_ROOT),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            env={'PYTHONPATH': str(SCRIPTS), 'PATH': '/usr/bin:/bin'},
            check=False,
        )
        assert result.returncode == 0
        assert '--check' in result.stdout.decode()


class TestStaticcheckPin(unittest.TestCase):
    """The feature-tag matrix needs the pinned tool, not stale presence.

    A different staticcheck on PATH runs and reports different findings than CI
    will, and nothing says why. That is worse than one that is absent.
    """

    def _staticcheck(self) -> Tool:
        found = [tool for tool in REQUIRED_TOOLS if tool.name == 'staticcheck']
        assert len(found) == 1, 'staticcheck must occur once in REQUIRED_TOOLS'
        return found[0]

    def test_the_pin_is_carried_into_the_install(self) -> None:
        from le.devtools.tools import STATICCHECK_VERSION

        assert self._staticcheck().probe == ('staticcheck',)
        assert self._staticcheck().go_install == (
            f'honnef.co/go/tools/cmd/staticcheck@{STATICCHECK_VERSION}'
        )

    def _probe_with(self, out: str, code: int = 0) -> bool:
        with (
            mock.patch.object(probes, 'which', return_value=Path('/test/bin/staticcheck')),
            mock.patch.object(probes, 'run', return_value=Result([], code, out, '')),
        ):
            return probes.probe(self._staticcheck())

    def test_the_expected_version_is_accepted(self) -> None:
        from le.devtools.tools import STATICCHECK_VERSION

        assert self._probe_with(f'staticcheck {STATICCHECK_VERSION} (v0.7.0)\n')

    def test_a_stale_version_is_rejected(self) -> None:
        assert not self._probe_with('staticcheck 2025.1.1 (v0.6.1)\n')

    def test_a_failing_tool_is_rejected(self) -> None:
        """Exit code first: a tool that ran and failed proves nothing."""
        assert not self._probe_with('staticcheck 2026.1 (v0.7.0)\n', code=1)

    def test_an_absent_tool_is_not_probed(self) -> None:
        with (
            mock.patch.object(probes, 'which', return_value=None),
            mock.patch.object(probes, 'run') as run,
        ):
            assert not probes.probe(self._staticcheck())
            run.assert_not_called()

    def test_a_timed_out_probe_is_rejected(self) -> None:
        """`run` reports a timeout as a non-zero result, so this is a rejection."""
        assert not self._probe_with('', code=124)


class TestGoplsIsRequired(unittest.TestCase):
    """gopls backs the agent LSP tool, which session-start makes BLOCKING.

    That gate lifts on the query text, so an absent server is invisible to it.
    This setup row is what installs it.
    """

    def test_gopls_is_required_and_installs_through_go(self) -> None:
        found = [tool for tool in REQUIRED_TOOLS if tool.name == 'gopls']
        assert len(found) == 1
        assert found[0].required
        assert found[0].go_install == 'golang.org/x/tools/gopls@latest'


class TestPyrightIsRequired(unittest.TestCase):
    """The Python half of the same story.

    scripts/dev/*.py and .claude/hooks/*.py were read 405 times in the measured
    transcript store with no symbol server on the machine at all.
    """

    def test_pyright_is_required_and_installs_through_pipx(self) -> None:
        found = [tool for tool in REQUIRED_TOOLS if tool.name == 'pyright']
        assert len(found) == 1
        assert found[0].required
        assert found[0].pipx_install == 'pyright'

    def test_mypy_is_required_and_installs_through_pipx(self) -> None:
        """The type gate `le lint` runs. Absent, that gate reports green on nothing."""
        found = [tool for tool in REQUIRED_TOOLS if tool.name == 'mypy']
        assert len(found) == 1
        assert found[0].required
        assert found[0].pipx_install == 'mypy'


if __name__ == '__main__':
    unittest.main()
