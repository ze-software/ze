"""Running commands, and reaching root without ever blocking on a prompt.

Two things live here because they are one concern: every command this program
runs goes through `run`, and the ones needing root go through `run_privileged`,
which is `run` with a decision about privilege taken first.

The rule the privileged half exists to keep: a setup program must never wait
for a password nobody is there to type. `sudo` reads its prompt from inherited
stdin, so a run with no terminal waits forever, and `sudo tee` fed a config
line on stdin hands that line to the prompt instead of to the file. So
privilege is decided BEFORE a command runs, sudo is always given `-n`, and a
password is asked for once, by `sudo -v`, only when a terminal is attached to
answer it. Every other case prints the command for a human and reports failure.
A setup program that says what to run is recoverable; one that blocks with no
output is not.
"""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from enum import Enum
from pathlib import Path

from le.console import echo
from le.paths import REPO_ROOT

__all__ = [
    'Command',
    'Privilege',
    'Result',
    'privilege',
    'run',
    'run_privileged',
    'which',
]

Command = list[str]

# `sudo -n true` touches no network and reads a local timestamp file. A second
# of that is already pathological, so this bound exists only to stop a wedged
# sudo -- an unreachable LDAP sudoers source, typically -- holding the run.
SUDO_PROBE_TIMEOUT = 15


@dataclass(frozen=True)
class Result:
    """What a command did.

    `out` and `err` are decoded here rather than at each call site: every
    caller wants text, and `errors='replace'` is the right answer for all of
    them because a tool's complaint is worth showing even when it is not valid
    UTF-8.
    """

    argv: Command
    code: int
    out: str
    err: str

    @property
    def ok(self) -> bool:
        return self.code == 0

    def complaint(self) -> str:
        """The first line worth showing a human when this failed.

        stderr first, then stdout: a tool that failed usually says why on
        stderr, and the ones that do not have already put it on stdout.
        """
        for stream in (self.err, self.out):
            lines = stream.strip().splitlines()
            if lines:
                return lines[0]
        return f'exit {self.code}'


def which(name: str) -> Path | None:
    """The path to `name` on PATH, or None."""
    found = shutil.which(name)
    return Path(found) if found else None


def run(
    argv: Command,
    *,
    cwd: Path | None = None,
    stdin: bytes | None = None,
    env: dict[str, str] | None = None,
    timeout: float | None = None,
) -> Result:
    """Run one command and capture what it said.

    Never raises for a command that ran and failed: that is a `Result` with a
    non-zero code, which every caller must handle anyway. A command that could
    not start at all, or that outran `timeout`, also comes back as a `Result`,
    with the reason in `err`. Callers branch on one thing rather than on a
    return value and two exception types.
    """
    try:
        completed = subprocess.run(
            argv,
            cwd=str(cwd) if cwd else None,
            input=stdin,
            capture_output=True,
            env=env,
            timeout=timeout,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return Result(argv, 124, '', f'no reply within {timeout}s')
    except OSError as err:
        return Result(argv, 127, '', str(err))
    return Result(
        argv,
        completed.returncode,
        completed.stdout.decode(errors='replace'),
        completed.stderr.decode(errors='replace'),
    )


def run_in_repo(argv: Command, *, timeout: float | None = None) -> Result:
    """Run one command with the checkout as its working directory."""
    return run(argv, cwd=REPO_ROOT, timeout=timeout)


def stream(
    argv: Command,
    *,
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
) -> int:
    """Run one command with its output going straight to the terminal.

    The opposite trade from `run`, and the right one for anything that takes
    minutes: a test suite, a linter, a fuzz target. The caller learns only the
    exit code, and the reader watches it happen instead of waiting for a
    transcript that arrives after the fact.

    stdout is flushed first. Ours is buffered and the child writes to the same
    descriptor directly, so without the flush any heading we printed appears
    AFTER the output it introduces.
    """
    sys.stdout.flush()
    try:
        completed = subprocess.run(argv, cwd=str(cwd) if cwd else None, env=env, check=False)
    except OSError as err:
        print(f'  cannot run {argv[0]}: {err}', file=sys.stderr)
        return 127
    return completed.returncode


class Privilege(Enum):
    """How this process can run a root command right now.

    ROOT    already root. No sudo is used, so none needs to be installed.
    SUDO    sudo acts with no password: NOPASSWD, or a live timestamp.
    PROMPT  sudo wants a password and a terminal is attached to type it on.
    NONE    no route to root that would not block. The caller prints the
            command instead of running it.
    """

    ROOT = 'root'
    SUDO = 'sudo'
    PROMPT = 'sudo-prompt'
    NONE = 'none'

    @property
    def prefix(self) -> str:
        """What a human would have to type in front of the command."""
        return '' if self is Privilege.ROOT else 'sudo '


def privilege() -> Privilege:
    """Decide the route to root, without taking it."""
    if os.geteuid() == 0:
        return Privilege.ROOT
    if which('sudo') is None:
        return Privilege.NONE
    probe = run(['sudo', '-n', 'true'], timeout=SUDO_PROBE_TIMEOUT)
    if probe.ok:
        return Privilege.SUDO
    return Privilege.PROMPT if sys.stdin.isatty() else Privilege.NONE


def run_privileged(
    argv: Command,
    *,
    stdin: bytes | None = None,
    shown: str | None = None,
) -> tuple[bool, str]:
    """Run one command as root. Returns (ok, detail).

    `shown` overrides the echoed command for a caller whose argv is not what a
    human would copy: a config drop-in is written by piping into `tee`, and
    printing that argv alone shows a `tee` with no content. It carries a
    `{sudo}` placeholder rather than a literal `sudo `, so the echoed line
    stays true on a root run, where no sudo is used.

    `detail` is written for whoever has to fix it: the command's own first line
    of complaint, or the reason root was out of reach.
    """
    mode = privilege()
    # `.replace`, not `.format`: a `{` anywhere else in the shown text -- a
    # shell brace expansion, an awk program -- would raise KeyError out of a
    # helper whose only job is to echo a line.
    label = shown.replace('{sudo}', mode.prefix) if shown else mode.prefix + ' '.join(argv)

    if mode is Privilege.NONE:
        return (False, f'no password-free route to root for `{label}`')

    if mode is Privilege.PROMPT:
        echo('  sudo needs your password')
        if not run(['sudo', '-v']).ok:
            return (False, f'sudo could not authenticate for `{label}`')

    # `-n` even after `sudo -v` has cached the timestamp: the prompt is what
    # eats a piped stdin, so this way no code path can reach one.
    full = argv if mode is Privilege.ROOT else ['sudo', '-n', *argv]
    echo(f'  Run: {label}')
    result = run(full, stdin=stdin)
    if not result.ok:
        return (False, f'{label}: {result.complaint()}')
    return (True, label)
