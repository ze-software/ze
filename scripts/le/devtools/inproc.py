"""Running a Python gate in THIS process instead of forking one.

Almost every gate `le` runs is a Python script, and `le` is a Python program.
Forking an interpreter to reach one is a fork to do what an import and a call
already do.

Speed is the smaller half of the reason and it is uneven: measured here,
`ze-rules-index-check` goes from 58ms to 17ms while `ze-rules-lint` barely
moves, because the interpreter start it saves is a fixed ~19ms and the rest is
the gate's own work. A cheap gate gains most.

The larger half is that a crash in a forked gate is an exit code, while a crash
in an imported one is an exception with a traceback through both programs at
once.

**Four things are borrowed and given back around every call**, because running
a script as a module is not the same as running it as a script, and each
difference broke a real gate here:

    sys.argv       set, because the script reads it through argparse
    sys.modules    the module is registered while its body runs: `@dataclass`
                   resolves a field type through `sys.modules[cls.__module__]`,
                   and an unregistered module makes that None. Removed after,
                   so the next call gets a fresh module and no global survives
    sys.path       the script's own directory, which Python adds when a script
                   is RUN and not when a module is imported by path.
                   `rules_condensed.py` does `import rules_router`, a sibling
    the exit code  `SystemExit` becomes a return value rather than the end of
                   `le`

**What makes this safe here, and what would make it unsafe elsewhere.** Every
script reached this way was checked for the three things that make in-process
execution wrong, and none does any: no `os.chdir`, which would move the working
directory of the whole program; no `os._exit`, which would take `le` down with
it; no module-level work that must not happen twice. A script that grows one
must go back to a fork, and `Gate.python_script` returning None is the switch.
"""

from __future__ import annotations

import importlib.util
import inspect
import os
import sys
import traceback
from collections.abc import Callable, Mapping, Sequence

from le.paths import REPO_ROOT

__all__ = ['CannotImport', 'call', 'importable']


class CannotImport(Exception):
    """The script could not be loaded, or does not offer a `main()`.

    Raised rather than returned so a caller cannot mistake it for the gate's
    own verdict. A gate that will not import is a `le` defect; a gate that
    imports and returns non-zero is the gate doing its job.
    """


def call(script: str, args: Sequence[str] = (), env: Mapping[str, str] | None = None) -> int:
    """Import `script` and run its `main()`. Returns the exit code it meant.

    `script` is repo-relative, the way the Make recipe spelled it.

    `env` is applied to `os.environ` for the duration and restored after. A
    forked gate gets its environment from the process it runs in; an imported
    one has to be given the same view, or the two routes are not the same run.
    Refusing the in-process route whenever an environment was supplied was the
    other option, and it made the route unreachable: every gate an area
    dispatches carries the toolchain environment, so nothing ever took it.
    """
    path = REPO_ROOT / script
    if not path.is_file():
        raise CannotImport(f'{script}: no such file')

    spec = importlib.util.spec_from_file_location(path.stem.replace('-', '_'), path)
    if spec is None or spec.loader is None:
        raise CannotImport(f'{script}: not importable as a module')

    module = importlib.util.module_from_spec(spec)
    directory = str(path.parent)

    saved_argv = sys.argv
    saved_environ = dict(os.environ)
    displaced = sys.modules.get(spec.name)
    added_path = directory not in sys.path

    if env is not None:
        os.environ.clear()
        os.environ.update(env)
    sys.argv = [str(path), *args]
    sys.modules[spec.name] = module
    if added_path:
        sys.path.insert(0, directory)

    # ONE scope over both the module body and `main()`. An earlier version
    # restored sys.path after the body and before the call, so a sibling
    # import inside `main()` -- which is where `rules_condensed` does its --
    # failed on a name sitting next to it on disk.
    try:
        try:
            spec.loader.exec_module(module)
        except SystemExit as stop:
            # Exiting during import is a verdict too: argparse rejecting the
            # argv, or a guard at module level.
            return _code(stop)
        except BaseException:
            raise CannotImport(f'{script}: failed to import\n{traceback.format_exc()}') from None

        try:
            main = module.main
        except AttributeError:
            raise CannotImport(f'{script}: declares no main()') from None

        try:
            result = main(*_call_args(main, args))
        except SystemExit as stop:
            return _code(stop)
    finally:
        sys.argv = saved_argv
        if env is not None:
            os.environ.clear()
            os.environ.update(saved_environ)
        if added_path and directory in sys.path:
            sys.path.remove(directory)
        if displaced is None:
            sys.modules.pop(spec.name, None)
        else:
            sys.modules[spec.name] = displaced

    # A `main()` returning None finished without saying otherwise, which is
    # exit 0. Several of these are declared `-> None` and signal failure only
    # through `sys.exit`.
    return 0 if result is None else int(result)


def _call_args(main: Callable[..., object], args: Sequence[str]) -> tuple[list[str], ...]:
    """What to pass `main`, which comes in three shapes across these scripts.

        def main()           reads sys.argv itself; pass nothing
        def main(argv=None)  None makes argparse read sys.argv; pass nothing
        def main(argv)       required, so it must be given a list

    Passing the list to the first shape is a TypeError, and withholding it from
    the third is the same error the other way. `sys.argv` is set for all three,
    so the first two behave identically either way and only the third needs it.

    **The third gets a FULL argv, program name included, because that is what
    the shape means.** A script declaring `main(argv)` is written to be called
    as `main(sys.argv)` from its own `__main__` guard, and its first act is
    `argv[1:]` to drop the program name. Handing it a bare option list makes it
    discard the first OPTION instead.

    That was live and silent. `rfc_requirements.main` does exactly this, so
    `le rfc ze-rfc-check` ran with no flags at all. Both spellings exit 2 on
    this tree, so the exit code could not tell them apart -- the failure was
    invisible from the outside, which is what makes it worth naming here.
    """
    try:
        parameters = list(inspect.signature(main).parameters.values())
    except (TypeError, ValueError):
        return ()
    required = [
        p
        for p in parameters
        if p.default is inspect.Parameter.empty
        and p.kind in (p.POSITIONAL_ONLY, p.POSITIONAL_OR_KEYWORD)
    ]
    # sys.argv, not the bare option list. `sys.argv[0]` is already the script,
    # set by `call` before this runs, so this hands over exactly what the
    # script's own `__main__` guard would have handed it.
    return (list(sys.argv),) if required else ()


def _code(stop: SystemExit) -> int:
    """The exit code a `SystemExit` carries.

    `sys.exit()` and `sys.exit(None)` both mean success. `sys.exit("message")`
    means failure, and the message has already been printed by convention in
    these scripts.
    """
    if stop.code is None:
        return 0
    if isinstance(stop.code, int):
        return stop.code
    return 1


def importable(script: str) -> bool:
    """Whether `script` is on disk and declares a `main()`, without running it.

    For the test that holds every in-process gate to the contract, so a gate
    pointed at a script with no `main()` is caught before a sweep meets it.
    """
    path = REPO_ROOT / script
    if not path.is_file():
        return False
    try:
        source = path.read_text(encoding='utf-8', errors='replace')
    except OSError:
        return False
    return any(line.startswith('def main(') for line in source.splitlines())
