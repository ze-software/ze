"""The vocabulary a setup-style report is written in, and how it prints.

The shell version of this threaded four parallel lists through one loop --
missing, pending, installed, skipped -- and appended to them by hand beside
each `print`. The label a reader saw and the list the run was judged by were
two records of one fact, kept in agreement by hand at a dozen call sites. One
of them drifted: an install that succeeded into a directory off PATH printed
`[installed]` and appended nothing, so the run ended "Setup complete" with
exit 0 while `--check` on the same box exited 1.

Here a step returns ONE `Outcome`. The label and the verdict come from the
same value, so they cannot disagree, and the tally is derived at the end
rather than accumulated along the way.
"""

from __future__ import annotations

import sys
from collections.abc import Iterable, Iterator
from dataclasses import dataclass
from enum import Enum

__all__ = ['Outcome', 'Report', 'State', 'echo']


def echo(line: str = '') -> None:
    """Write one line to stdout.

    Wrapped rather than called directly so that output has one exit from this
    program, which is what makes a future `--quiet` or a log file one change
    instead of two hundred.
    """
    print(line)


class State(Enum):
    """What a setup step found, and what that means for the exit code.

    The label is what a reader sees. `blocking` is what the run is judged by.
    They travel together because the shell version let them drift apart.

    PRESENT    already there and working. Nothing to do.
    INSTALLED  this run made it so.
    PENDING    the machine was changed and a human must finish it: a PATH to
               extend, a session to restart. Re-running does not help, so the
               run must not report success.
    SKIPPED    nothing to do here, and nothing wrong: an optional tool, or a
               platform with no package for it.
    MISSING    required and absent, with no route taken to fix it.
    """

    PRESENT = 'present'
    INSTALLED = 'installed'
    PENDING = 'pending'
    SKIPPED = 'skipped'
    MISSING = 'MISSING'

    @property
    def blocking(self) -> bool:
        """Whether a step in this state must fail the run."""
        return self in (State.PENDING, State.MISSING)


@dataclass(frozen=True)
class Outcome:
    """One step's result: what it was, how it went, and why.

    `detail` is written for whoever has to fix it, so it names the command, the
    path, or the complaint rather than restating the state.
    """

    name: str
    state: State
    detail: str = ''

    def line(self) -> str:
        """The report line for this outcome."""
        suffix = f' ({self.detail})' if self.detail else ''
        return f'  [{self.state.value:<9}] {self.name}{suffix}'


@dataclass
class Report:
    """Every outcome of one run, and the verdict derived from them.

    Nothing is tallied as it goes. `outcomes` is the whole record, and every
    question below is answered by reading it, so a step cannot be counted in
    one place and not another.
    """

    outcomes: list[Outcome]

    def __init__(self) -> None:
        self.outcomes = []

    def add(self, outcome: Outcome) -> Outcome:
        """Record one outcome and print its line. Returns it, for a caller
        that wants to branch on what it just recorded."""
        self.outcomes.append(outcome)
        echo(outcome.line())
        return outcome

    def having(self, *states: State) -> Iterator[Outcome]:
        """Every outcome in one of `states`, in the order they happened."""
        return (o for o in self.outcomes if o.state in states)

    @staticmethod
    def _names(outcomes: Iterable[Outcome]) -> str:
        return ', '.join(o.name for o in outcomes)

    def summarise(self) -> int:
        """Print the closing verdict. Returns the process exit code.

        Missing is reported before pending because it is the harder failure:
        pending means the machine changed and a human must finish, missing
        means nothing was done at all.
        """
        echo()

        missing = list(self.having(State.MISSING))
        if missing:
            echo(f'Missing required tools: {self._names(missing)}')
            return 1

        pending = list(self.having(State.PENDING))
        if pending:
            # "Steps", not "install commands": a tool that landed in a
            # directory off PATH needs the PATH fixed, not the install re-run.
            echo(f'Finish the steps above for: {self._names(pending)}')
            echo('Then re-run: ./le setup')
            return 1

        parts: list[str] = []
        installed = list(self.having(State.INSTALLED))
        if installed:
            parts.append(f'installed: {self._names(installed)}')
        skipped = list(self.having(State.SKIPPED))
        if skipped:
            parts.append(f'skipped (optional): {self._names(skipped)}')

        if parts:
            echo(f'Setup complete. {"; ".join(parts)}')
        else:
            echo('Setup complete. All tools already present.')
        return 0

    def check_verdict(self) -> int:
        """The closing verdict for a probe-only run. Returns the exit code.

        A probe changes nothing, so PENDING here never means "this run changed
        the machine and a human must finish". It means the check found
        something only a human CAN do: a plugin this program must not install
        (`le/devtools/editor.py`), a group that needs a fresh login. Both fail
        the run, and they are reported apart because their fixes differ --
        calling a plugin a missing tool sends the reader to the tool table,
        which does not install it.
        """
        echo()

        missing = list(self.having(State.MISSING))
        pending = list(self.having(State.PENDING))

        if missing:
            echo(f'Missing required tools: {self._names(missing)}')
        if pending:
            echo(f'Needs a step only you can take: {self._names(pending)}')
        if missing or pending:
            return 1

        echo('All required tools present.')
        return 0


def fail(message: str) -> int:
    """Report a condition that stops the run before any step, and give the
    exit code to return."""
    print(message, file=sys.stderr)
    return 1
