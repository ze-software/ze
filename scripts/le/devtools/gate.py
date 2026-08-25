"""A gate: one named check, the command that runs it, and why it exists.

Most of what the Makefile held was this shape. A target name, a one-line
recipe that ran a script, and a comment block carrying the only thing a reader
could not reconstruct -- what the gate is for and what it caught. Make could
express the first two and had nowhere to put the third except a comment no
program reads.

Here the three travel together. `Gate.why` is data, so `le <area> --list`
prints it, a help text is derived rather than written twice, and a gate with no
stated reason is visible as one.

Streaming, not capturing. A gate can run for minutes and a reader needs its
output as it appears, so `run` inherits stdout and stderr rather than piping
them. That is the opposite of `le.process.run`, which captures because its
callers parse what came back.
"""

from __future__ import annotations

from collections.abc import Sequence
from dataclasses import dataclass, field

from le.console import echo
from le.devtools.inproc import CannotImport, call
from le.paths import REPO_ROOT
from le.process import stream

__all__ = ['Gate', 'GateSet', 'run_gate']


@dataclass(frozen=True)
class Gate:
    """One check, by the name it had as a Make target.

    `name` keeps the `ze-` spelling the repository already uses everywhere, so
    a reference in a doc, a rule, or a journal row still names something real.

    `json` is the variant that prints a machine-readable report. It is a flag
    on the same gate rather than a second gate, because they are one check with
    two renderings, and listing them apart doubled the Makefile's target count
    for no reader's benefit.
    """

    name: str
    argv: tuple[str, ...]
    why: str
    json_flag: str | None = None
    writes: bool = False

    @property
    def python_script(self) -> str | None:
        """The repo-relative script this gate runs, when it runs one directly.

        `('python3', 'scripts/dev/x.py', ...)` is the shape almost every gate
        has, and it is the shape that can run in THIS process rather than a
        forked one (`le/devtools/inproc.py`). Anything else -- `go`, a shell,
        a script invoked some other way -- is None and forks.

        Derived from the argv rather than declared beside it, so a gate cannot
        say it is a Python gate while running something else.
        """
        if len(self.argv) >= 2 and self.argv[0] == 'python3' and self.argv[1].endswith('.py'):
            return self.argv[1]
        return None

    def command(self, *, as_json: bool = False) -> list[str]:
        """The argv to run, with the JSON flag appended when asked for."""
        if as_json and self.json_flag:
            return [*self.argv, self.json_flag]
        return list(self.argv)

    @property
    def has_json(self) -> bool:
        return self.json_flag is not None


@dataclass(frozen=True)
class GateSet:
    """Every gate one area declares, and the order they run in.

    Order is the declaration order. A set that runs as a group runs them in it,
    so a cheap structural check reports before an expensive one starts.
    """

    area: str
    gates: tuple[Gate, ...] = field(default=())

    def find(self, name: str) -> Gate | None:
        for gate in self.gates:
            if gate.name == name:
                return gate
        return None

    def names(self) -> tuple[str, ...]:
        return tuple(gate.name for gate in self.gates)

    def checks(self) -> tuple[Gate, ...]:
        """The gates that only report. Safe to run over a tree you care about."""
        return tuple(gate for gate in self.gates if not gate.writes)

    def writers(self) -> tuple[Gate, ...]:
        """The gates that regenerate a file."""
        return tuple(gate for gate in self.gates if gate.writes)

    def render_list(self) -> None:
        """Print every gate with what it is for."""
        width = max((len(gate.name) for gate in self.gates), default=0)
        for gate in self.gates:
            mark = 'writes' if gate.writes else 'checks'
            echo(f'  {gate.name:<{width}}  {mark}  {gate.why}')


def run_gate(gate: Gate, *, as_json: bool = False, env: dict[str, str] | None = None) -> int:
    """Run one gate and return its exit code.

    Output is NOT captured: a gate can run for minutes, and a reader needs to
    see it happen. A caller that wants the text runs the command itself through
    `le.process.run`.
    """
    argv = gate.command(as_json=as_json)
    if not as_json:
        echo(f'==> {gate.name}')

    script = gate.python_script
    if script is not None and env is None:
        # In-process: `le` is a Python program and so is the gate, so an
        # import and a call reach it without an interpreter start. Only when
        # `env` is None -- a gate needing a changed environment needs a real
        # process to hold it, and mutating os.environ for the rest of the run
        # is not a trade worth 18ms.
        try:
            return call(script, argv[2:])
        except CannotImport as why:
            # Never silently a different answer: say the import failed and
            # fall back, so a gate that stops importing is visible rather
            # than quietly forking forever.
            echo(f'  (in-process import failed, forking) {why}')

    return stream(argv, cwd=REPO_ROOT, env=env)


def run_all(gates: Sequence[Gate], *, env: dict[str, str] | None = None) -> list[str]:
    """Run every gate and return the names of those that failed.

    Every gate runs even after one fails. A run that stops at the first red
    reports one problem per invocation, and the point of a gate sweep is to
    hand back the whole list.
    """
    failed: list[str] = []
    for gate in gates:
        if run_gate(gate, env=env) != 0:
            failed.append(gate.name)
    return failed
