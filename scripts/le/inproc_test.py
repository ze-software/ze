#!/usr/bin/env python3
"""Tests for running a Python gate in this process rather than forking one.

Every case here records something that broke when a script written to be RUN
was instead IMPORTED. Running and importing differ in four ways, and three of
them produced a failure that read as a defect in the script rather than in how
it was loaded.
"""

from __future__ import annotations

import io
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.devtools.gate import Gate, run_all, run_gate
from le.devtools.inproc import CannotImport, call, importable
from le.paths import REPO_ROOT
from le.registry import REGISTRY


def _script(body: str, name: str = 'probe.py') -> str:
    """Write a throwaway script under the repo and return its relative path.

    Under the repo because `call` resolves against REPO_ROOT, which is what
    every real gate path is relative to.
    """
    directory = Path(tempfile.mkdtemp(dir=REPO_ROOT / 'tmp'))
    (directory / name).write_text(body)
    return str((directory / name).relative_to(REPO_ROOT))


class TestExitCode(unittest.TestCase):
    """`SystemExit` is a return value here, not the end of `le`."""

    def test_a_returned_int_is_the_code(self) -> None:
        assert call(_script('def main():\n    return 3\n')) == 3

    def test_returning_none_is_success(self) -> None:
        """Several of these scripts are `-> None` and fail only via sys.exit."""
        assert call(_script('def main():\n    pass\n')) == 0

    def test_sys_exit_with_a_code_is_that_code(self) -> None:
        assert call(_script('import sys\ndef main():\n    sys.exit(2)\n')) == 2

    def test_bare_sys_exit_is_success(self) -> None:
        assert call(_script('import sys\ndef main():\n    sys.exit()\n')) == 0

    def test_sys_exit_with_a_message_is_failure(self) -> None:
        with redirect_stdout(io.StringIO()):
            assert call(_script('import sys\ndef main():\n    sys.exit("no")\n')) == 1

    def test_exiting_during_import_is_still_a_verdict(self) -> None:
        """argparse rejecting argv exits before `main` is ever reached."""
        assert call(_script('import sys\nsys.exit(4)\ndef main():\n    return 0\n')) == 4


class TestArgv(unittest.TestCase):
    def test_the_script_sees_its_arguments(self) -> None:
        body = 'import sys\ndef main():\n    return len(sys.argv) - 1\n'
        assert call(_script(body), ['a', 'b']) == 2

    def test_argv_zero_is_the_script(self) -> None:
        body = "import sys\ndef main():\n    return 0 if sys.argv[0].endswith('probe.py') else 9\n"
        assert call(_script(body)) == 0

    def test_our_own_argv_is_restored(self) -> None:
        """`le` outlives the gate and reads its own argv afterwards."""
        before = list(sys.argv)
        call(_script('def main():\n    return 0\n'), ['x'])
        assert sys.argv == before

    def test_argv_is_restored_even_when_the_gate_fails(self) -> None:
        before = list(sys.argv)
        call(_script('import sys\ndef main():\n    sys.exit(1)\n'), ['x'])
        assert sys.argv == before


class TestMainSignature(unittest.TestCase):
    """Three shapes exist across these scripts and all three must work."""

    def test_main_taking_nothing(self) -> None:
        assert call(_script('def main():\n    return 0\n')) == 0

    def test_main_with_an_optional_argv(self) -> None:
        body = 'def main(argv=None):\n    return 0 if argv is None else 9\n'
        assert call(_script(body)) == 0

    def test_main_with_a_required_argv_and_no_guard_gets_the_options(self) -> None:
        """With no `__main__` guard, nothing states a convention.

        Such a `main` is called by us alone, so the options are what it wants.
        Handing it a program name it never asked for would make it read that as
        an argument.
        """
        body = 'def main(argv):\n    return len(argv)\n'
        assert call(_script(body), ['a', 'b', 'c']) == 3

    def test_the_program_name_is_argv_zero_for_that_shape(self) -> None:
        body = (
            'import sys\n'
            "def main(argv):\n    return 0 if argv[0].endswith('probe.py') else 9\n"
            "if __name__ == '__main__':\n    sys.exit(main(sys.argv))\n"
        )
        assert call(_script(body), ['--flag']) == 0

    def test_a_tail_convention_script_gets_no_program_name(self) -> None:
        """The opposite convention, and the one that broke when I guessed.

        `spec-citation-check.py` ends `sys.exit(main(sys.argv[1:]))`, so its
        `main` never trims. Handing it a full argv makes it read the program
        name as an option: it exited 0 run directly and 2 through `le`.
        """
        body = (
            'import sys\n'
            'def main(argv):\n    return len(argv)\n'
            "if __name__ == '__main__':\n    sys.exit(main(sys.argv[1:]))\n"
        )
        assert call(_script(body), ['--a', '--b']) == 2

    def test_the_script_decides_not_the_caller(self) -> None:
        """One body, two guards, two answers. The guard is the whole signal."""
        core = 'def main(argv):\n    return len(argv)\n'
        whole = _script(
            'import sys\n' + core + "if __name__ == '__main__':\n    sys.exit(main(sys.argv))\n"
        )
        tail = _script(
            'import sys\n' + core + "if __name__ == '__main__':\n    sys.exit(main(sys.argv[1:]))\n"
        )
        assert call(whole, ['--x']) == 2
        assert call(tail, ['--x']) == 1


class TestModuleRegistration(unittest.TestCase):
    def test_a_module_level_dataclass_works(self) -> None:
        """The failure that made three real gates unimportable.

        `@dataclass` resolves a field's type through
        `sys.modules[cls.__module__]`. An unregistered module makes that None,
        and the decorator dies with "'NoneType' object has no attribute
        '__dict__'" -- which reads as a defect in the script.
        """
        body = (
            'from dataclasses import dataclass\n'
            '@dataclass\n'
            'class Thing:\n'
            '    value: int = 0\n'
            'def main():\n'
            '    return Thing().value\n'
        )
        assert call(_script(body)) == 0

    def test_sys_modules_is_left_as_it_was(self) -> None:
        """A module left registered would be reused, carrying its globals."""
        before = set(sys.modules)
        call(_script('def main():\n    return 0\n'))
        assert set(sys.modules) - before == set()

    def test_module_state_does_not_survive_into_the_next_call(self) -> None:
        """A sweep runs one script twice: render, then check."""
        body = 'COUNT = []\ndef main():\n    COUNT.append(1)\n    return len(COUNT)\n'
        script = _script(body)
        assert call(script) == 1
        assert call(script) == 1, 'a global survived into the second call'


class TestSysPath(unittest.TestCase):
    def test_a_sibling_import_resolves(self) -> None:
        """Python adds a script's directory when RUNNING it, not when importing.

        `rules_condensed.py` does `import rules_router`, a file beside it.
        Without the path entry it fails on a name sitting next to it on disk.
        """
        directory = Path(tempfile.mkdtemp(dir=REPO_ROOT / 'tmp'))
        (directory / 'helper_mod.py').write_text('VALUE = 7\n')
        (directory / 'probe.py').write_text(
            'def main():\n    import helper_mod\n    return helper_mod.VALUE\n'
        )
        rel = str((directory / 'probe.py').relative_to(REPO_ROOT))
        assert call(rel) == 7

    def test_the_path_entry_is_given_back(self) -> None:
        before = list(sys.path)
        call(_script('def main():\n    return 0\n'))
        assert sys.path == before


class TestRefusals(unittest.TestCase):
    """A script that will not load is a `le` defect, not the gate's verdict."""

    def test_a_missing_script_is_refused(self) -> None:
        with self.assertRaises(CannotImport):
            call('scripts/dev/there-is-no-such-file.py')

    def test_a_script_with_no_main_is_refused(self) -> None:
        with self.assertRaises(CannotImport):
            call(_script('VALUE = 1\n'))

    def test_a_script_that_raises_on_import_is_refused(self) -> None:
        with self.assertRaises(CannotImport):
            call(_script('raise RuntimeError("boom")\ndef main():\n    return 0\n'))

    def test_an_exception_inside_main_propagates(self) -> None:
        """The PRIMITIVE lets it through, so a caller can decide.

        `run_gate` is where that decision is made, and it counts a raising
        gate as one failure rather than the end of the sweep
        (`TestARaisingGateDoesNotAbortTheSweep`).
        """
        with self.assertRaises(RuntimeError):
            call(_script('def main():\n    raise RuntimeError("inside")\n'))


class TestARaisingGateDoesNotAbortTheSweep(unittest.TestCase):
    """A gate that raises is ONE failure, not the end of the run.

    Forking made this free: an uncaught exception became a non-zero exit code
    the caller collected. In process it propagated, and `le build-terminal-demo`
    ran one check of three before dying with a traceback.
    `demos/terminal/render.py` raises ValueError when an asset is stale, so
    this was live rather than hypothetical.
    """

    def _raising_gate(self) -> Gate:
        script = _script('def main():\n    raise ValueError("stale")\n')
        return Gate(name='ze-probe-check', argv=('python3', script), why='probe')

    def test_it_is_counted_as_a_failure(self) -> None:
        with redirect_stdout(io.StringIO()):
            assert run_gate(self._raising_gate()) == 1

    def test_the_traceback_is_still_shown(self) -> None:
        """It is the reason importing beats forking; losing it costs the point."""
        buffer = io.StringIO()
        with redirect_stdout(buffer):
            run_gate(self._raising_gate())
        said = buffer.getvalue()
        assert 'ValueError' in said
        assert 'stale' in said

    def test_the_gates_behind_it_still_run(self) -> None:
        raising = self._raising_gate()
        after = Gate(
            name='ze-probe-after',
            argv=('python3', _script('def main():\n    return 0\n')),
            why='runs after',
        )
        with redirect_stdout(io.StringIO()):
            failed = run_all([raising, after])
        assert failed == ['ze-probe-check'], 'the second gate must still have run'


class TestEveryInProcessGateCanRun(unittest.TestCase):
    def test_every_python_gate_declares_a_main(self) -> None:
        """Caught here rather than mid-sweep, where it would fork silently."""
        for entry in REGISTRY:
            module = entry.load()
            gates = getattr(module, 'GATES', None)
            if gates is None:
                continue
            for gate in gates.gates:
                script = gate.python_script
                if script is None:
                    continue
                assert importable(script), f'{gate.name}: {script} has no main()'

    def test_a_go_gate_is_not_taken_in_process(self) -> None:
        """Only `python3 <file>.py` qualifies; everything else forks."""
        for entry in REGISTRY:
            module = entry.load()
            gates = getattr(module, 'GATES', None)
            if gates is None:
                continue
            for gate in gates.gates:
                if gate.argv[0] != 'python3':
                    assert gate.python_script is None, gate.name


if __name__ == '__main__':
    unittest.main()
