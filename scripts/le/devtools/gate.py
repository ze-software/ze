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

    def short(self, area: str) -> str:
        """The name to TYPE, with the area's own prefix removed.

        `le rfc ze-rfc-check` says rfc twice. The area is already chosen by
        then, so repeating it in the gate name is noise the reader supplies
        and the program ignores. `le rfc check` is the same gate.

        `name` stays the Make target, because that is what every shim, doc,
        rule and journal row spells, and what a reader greps for. This is a
        rendering of it, not a replacement.

        A gate whose target does not begin with the area's prefix keeps its
        full name: `ze-discovery-index-check` sitting in `rules` has no
        redundancy to remove, and inventing one would hide where it lives.
        """
        for prefix in (f'ze-{area}-', f'ze-{area.replace("-", "")}-'):
            if self.name.startswith(prefix):
                return self.name[len(prefix) :]
        return self.name

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
        """The gate called `name`, by either spelling.

        Both are accepted on purpose. The short name is what a person types
        now; the full target name is what every existing doc, rule, shim and
        muscle memory in the repository still says. Refusing one of them would
        break callers to make a point about naming.
        """
        for gate in self.gates:
            if gate.name == name or gate.short(self.area) == name:
                return gate
        return None

    def names(self) -> tuple[str, ...]:
        """The names to offer a reader who mistyped: the short ones."""
        return tuple(gate.short(self.area) for gate in self.gates)

    def checks(self) -> tuple[Gate, ...]:
        """The gates that only report. Safe to run over a tree you care about."""
        return tuple(gate for gate in self.gates if not gate.writes)

    def writers(self) -> tuple[Gate, ...]:
        """The gates that regenerate a file."""
        return tuple(gate for gate in self.gates if gate.writes)

    def render_list(self) -> None:
        """Print every gate, by the name a reader types, with what it is for."""
        shorts = [gate.short(self.area) for gate in self.gates]
        width = max((len(s) for s in shorts), default=0)
        for gate, short in zip(self.gates, shorts, strict=True):
            mark = 'writes' if gate.writes else 'checks'
            echo(f'  {short:<{width}}  {mark}  {gate.why}')


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
    if script is not None:
        # In-process: `le` is a Python program and so is the gate, so an
        # import and a call reach it without an interpreter start. The gate's
        # environment goes with it -- `call` applies it to os.environ and
        # restores it -- because a forked gate gets its environment from its
        # process and an imported one must be given the same view.
        #
        # An earlier version skipped the in-process route whenever an
        # environment was supplied. Every gate an area dispatches carries the
        # toolchain environment, so that made the route unreachable and the
        # whole mechanism inert.
        try:
            return call(script, argv[2:], env=env)
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
