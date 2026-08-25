#!/usr/bin/env python3
"""Tests for running commands and reaching root.

Ported from scripts/dev/dev_setup_test.py when `le setup` replaced that script.
The reasoning is kept because the reasoning is the value: each case here records
a way a setup program hung or wrote a password into a config file.
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

from le import process
from le.process import Privilege


def _completed(code: int, out: bytes = b'', err: bytes = b'') -> subprocess.CompletedProcess[bytes]:
    return subprocess.CompletedProcess([], code, out, err)


class TestRun(unittest.TestCase):
    """A command that fails, cannot start, or hangs must all come back the same way.

    `run` never raises for any of them. A caller that had to handle a return
    value and two exception types would get one of the three wrong, and the one
    it got wrong would be the rare one.
    """

    def test_a_failing_command_is_a_result_not_an_exception(self) -> None:
        result = process.run([sys.executable, '-c', 'raise SystemExit(3)'])
        assert result.code == 3
        assert not result.ok

    def test_a_command_that_cannot_start_is_a_result(self) -> None:
        result = process.run(['/nonexistent/binary'])
        assert not result.ok
        assert result.err

    def test_a_command_that_outruns_its_timeout_is_a_result(self) -> None:
        result = process.run([sys.executable, '-c', 'import time; time.sleep(30)'], timeout=0.5)
        assert not result.ok
        assert 'no reply within' in result.err

    def test_output_is_captured_and_decoded(self) -> None:
        result = process.run([sys.executable, '-c', 'print("hello")'])
        assert result.ok
        assert 'hello' in result.out

    def test_undecodable_output_does_not_raise(self) -> None:
        """A tool's complaint is worth showing even when it is not valid UTF-8."""
        result = process.run(
            [sys.executable, '-c', r'import sys; sys.stdout.buffer.write(b"\xff")']
        )
        assert result.ok


class TestComplaint(unittest.TestCase):
    def test_stderr_wins_over_stdout(self) -> None:
        """A tool that failed usually says why on stderr."""
        result = process.Result(['x'], 1, 'from stdout', 'from stderr')
        assert result.complaint() == 'from stderr'

    def test_only_the_first_line(self) -> None:
        """The echoed line is what a reader sees; the rest is in the log."""
        result = process.Result(['x'], 1, '', 'E: nope\nmore\n')
        assert result.complaint() == 'E: nope'

    def test_stdout_is_used_when_stderr_is_silent(self) -> None:
        result = process.Result(['x'], 1, 'said it here', '')
        assert result.complaint() == 'said it here'

    def test_a_silent_failure_still_says_something(self) -> None:
        assert 'exit 7' in process.Result(['x'], 7, '', '').complaint()


class TestPrivilege(unittest.TestCase):
    """No route to root may block on a password prompt.

    Setup runs from a container build and from an agent session with no
    terminal at all. sudo reads its prompt from the stdin it inherits, so "can
    sudo act without a password" has to be answered BEFORE a command runs.
    Answered after, the answer arrives on a run that is already hung with no
    output.
    """

    def _mode(
        self,
        *,
        euid: int = 1000,
        sudo: str | None = '/usr/bin/sudo',
        code: int = 0,
        tty: bool = True,
    ) -> Privilege:
        stdin = mock.Mock()
        stdin.isatty.return_value = tty
        found = Path(sudo) if sudo else None
        with (
            mock.patch('le.process.os.geteuid', return_value=euid),
            mock.patch.object(process, 'which', return_value=found),
            mock.patch('le.process.sys.stdin', stdin),
            mock.patch.object(process, 'run', return_value=process.Result([], code, '', '')),
        ):
            return process.privilege()

    def test_root_needs_no_sudo(self) -> None:
        """A container build runs as root, where sudo is usually not installed."""
        assert self._mode(euid=0, sudo=None) is Privilege.ROOT

    def test_no_sudo_binary_is_no_route(self) -> None:
        assert self._mode(sudo=None) is Privilege.NONE

    def test_passwordless_sudo_is_a_route(self) -> None:
        assert self._mode(code=0) is Privilege.SUDO

    def test_a_password_with_a_terminal_to_type_it_on_is_a_route(self) -> None:
        assert self._mode(code=1, tty=True) is Privilege.PROMPT

    def test_a_password_with_no_terminal_is_no_route(self) -> None:
        """The discriminating case, and the whole reason this function exists.

        sudo would print its prompt and wait forever. Drop the isatty test and
        only this fails: every other case here is already decided by `sudo -n`.
        """
        assert self._mode(code=1, tty=False) is Privilege.NONE

    def test_a_wedged_sudo_is_no_route(self) -> None:
        """An unreachable sudoers source (LDAP, typically) hangs `sudo -n`.

        `run` turns that into a non-zero result rather than an exception, so
        the timeout arrives here as a refusal, which with no tty is NONE.
        """
        stdin = mock.Mock()
        stdin.isatty.return_value = False
        timed_out = process.Result([], 124, '', 'no reply within 15s')
        with (
            mock.patch('le.process.os.geteuid', return_value=1000),
            mock.patch.object(process, 'which', return_value=Path('/usr/bin/sudo')),
            mock.patch('le.process.sys.stdin', stdin),
            mock.patch.object(process, 'run', return_value=timed_out),
        ):
            assert process.privilege() is Privilege.NONE

    def test_the_probe_is_bounded(self) -> None:
        """Without a timeout a wedged sudo holds the whole run open."""
        assert process.SUDO_PROBE_TIMEOUT > 0


class TestRunPrivileged(unittest.TestCase):
    def _run(
        self,
        argv: list[str],
        *,
        mode: Privilege = Privilege.SUDO,
        code: int = 0,
        **kwargs: object,
    ) -> tuple[bool, str, mock.Mock]:
        with (
            mock.patch.object(process, 'privilege', return_value=mode),
            mock.patch.object(process, 'run') as run,
            redirect_stdout(io.StringIO()),
        ):
            run.return_value = process.Result([], code, '', 'E: nope\nmore\n')
            ok, detail = process.run_privileged(argv, **kwargs)  # type: ignore[arg-type]
        return ok, detail, run

    def test_sudo_is_always_given_n(self) -> None:
        """The regression that motivated one helper for all three callers.

        `sudo tee` used to be handed the file's content on stdin while sudo was
        free to prompt on that same stdin: the prompt eats the content and the
        drop-in gets written with a password in it, or nothing. `-n` means no
        code path can reach a prompt, so a piped stdin only ever reaches the
        command.
        """
        _, _, run = self._run(['tee', '/etc/x.conf'], stdin=b'a = 0\n')
        assert run.call_args.args[0] == ['sudo', '-n', 'tee', '/etc/x.conf']
        assert run.call_args.kwargs['stdin'] == b'a = 0\n'

    def test_root_runs_the_command_bare(self) -> None:
        _, _, run = self._run(['apt-get', 'update'], mode=Privilege.ROOT)
        assert run.call_args.args[0] == ['apt-get', 'update']

    def test_the_echoed_line_says_what_actually_ran(self) -> None:
        """A root run must not print a `sudo` it did not use.

        The echoed line is what a reader copies when the step fails, and on a
        container build there is no sudo on the box to copy it to.
        """
        buffer = io.StringIO()
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.ROOT),
            mock.patch.object(process, 'run', return_value=process.Result([], 0, '', '')),
            redirect_stdout(buffer),
        ):
            process.run_privileged(
                ['tee', '/etc/x.conf'],
                stdin=b'a = 0\n',
                shown='echo "a = 0" | {sudo}tee /etc/x.conf',
            )
        said = buffer.getvalue()
        assert 'echo "a = 0" | tee /etc/x.conf' in said
        assert 'sudo' not in said

    def test_a_brace_in_the_shown_text_is_not_a_format_field(self) -> None:
        """`.format` would raise KeyError out of a helper that only echoes.

        A shell brace expansion or an awk program in the shown text reaches
        this, and the failure would be an exception rather than a wrong line.
        """
        buffer = io.StringIO()
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.ROOT),
            mock.patch.object(process, 'run', return_value=process.Result([], 0, '', '')),
            redirect_stdout(buffer),
        ):
            process.run_privileged(['awk', '{print $1}'], shown='{sudo}awk {print $1}')
        assert '{print $1}' in buffer.getvalue()

    def test_a_password_is_asked_for_once_then_never_prompted_again(self) -> None:
        """sudo wants a password and a terminal is attached to type it on.

        `sudo -v` is the only interactive call, and every command after it is
        still `-n`, so a piped stdin cannot reach a prompt even here.
        """
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.PROMPT),
            mock.patch.object(process, 'run') as run,
            redirect_stdout(io.StringIO()),
        ):
            run.return_value = process.Result([], 0, '', '')
            ok, _ = process.run_privileged(['apt-get', 'update'])

        assert ok
        calls = [call.args[0] for call in run.call_args_list]
        assert calls[0] == ['sudo', '-v']
        assert calls[1] == ['sudo', '-n', 'apt-get', 'update']

    def test_a_refused_password_runs_nothing(self) -> None:
        with (
            mock.patch.object(process, 'privilege', return_value=Privilege.PROMPT),
            mock.patch.object(process, 'run') as run,
            redirect_stdout(io.StringIO()),
        ):
            run.return_value = process.Result([], 1, '', '')
            ok, detail = process.run_privileged(['apt-get', 'update'])

        assert not ok
        assert 'could not authenticate' in detail
        assert [call.args[0] for call in run.call_args_list] == [['sudo', '-v']]

    def test_no_route_runs_nothing(self) -> None:
        ok, detail, run = self._run(['apt-get', 'update'], mode=Privilege.NONE)
        assert not ok
        run.assert_not_called()
        assert 'no password-free route to root' in detail

    def test_failure_carries_the_command_s_own_first_line(self) -> None:
        ok, detail, _ = self._run(['apt-get', 'install', '-y', 'x'], code=100)
        assert not ok
        assert 'E: nope' in detail
        assert 'more' not in detail


if __name__ == '__main__':
    unittest.main()
