"""Whether a language server ANSWERS, which is not the same as being installed.

A binary on PATH is not a working language server, and the difference is
invisible until a call is made. On one of the two dev machines gopls was absent
for weeks while the BLOCKING "load LSP first" rule was satisfied every session,
because that gate lifts on the query text and no call was ever made: every one
of them would have returned ENOENT. A presence check would not have caught it
either. So these checks RUN the server and require an answer.

The Python half is the same story. scripts/dev/*.py and .claude/hooks/*.py were
read 405 times in the measured transcript store with no symbol server on the
machine at all, so every one of those reads was a whole file where a symbol was
the question (ai/rules/context-economy.md).
"""

from __future__ import annotations

import json
import re
from collections.abc import Callable
from dataclasses import dataclass
from enum import Enum
from typing import Any

from le.paths import REPO_ROOT
from le.process import Result, run, which

__all__ = ['Answer', 'Health', 'gopls_health', 'pyright_health', 'pyright_summary']


class Health(Enum):
    """Whether a language server answered, and if not, whose problem it is.

    OK       the server answered.
    ABSENT   nothing on PATH. A DIFFERENT problem with a different fix, and the
             tool table already installs it.
    BROKEN   it ran and gave nothing usable: a timeout, a non-zero exit, or a
             reply with no answer in it. Typically a broken module cache, a
             package that does not build, or a version mismatch -- none of
             which installing the server again repairs.
    NA       there is nothing here to ask about, so the question does not apply.
    """

    OK = 'ok'
    ABSENT = 'absent'
    BROKEN = 'broken'
    NA = 'na'


@dataclass(frozen=True)
class Answer:
    """What a server probe found, and a sentence for whoever must fix it."""

    health: Health
    detail: str


Probe = Callable[[], Result]


# --- gopls ----------------------------------------------------------------
#
# The probe file is internal/core/clock/clock.go: 128 lines, and its package
# imports `time` and nothing else, which is the smallest dependency footprint
# in the repository. gopls must load and type-check the package to answer, so a
# probe on a package with a wide import graph would measure the graph rather
# than the server.
GOPLS_PROBE_FILE = 'internal/core/clock/clock.go'

# gopls type-checks the package before it answers, so the first run on a cold
# Go build cache pays for the standard library too. Measured warm on this
# machine: 3.4s. The timeout is ~35x that, which leaves a cold cache room
# without letting a hung server hold the run open indefinitely. A check that
# reds spuriously is a check somebody disables.
GOPLS_PROBE_TIMEOUT = 120

# One line of `gopls symbols` output: a name, a symbol kind, and a range.
GOPLS_SYMBOL_LINE = re.compile(r'\b[A-Z][A-Za-z]+\s+\d+:\d+-\d+:\d+')

GOPLS_NOT_INSTALLED = 'gopls is not installed; the gopls row above installs it'
GOPLS_NOT_ANSWERING = 'gopls is present but not answering'


def gopls_probe() -> Result:
    """Ask the language server for the symbols of one small file.

    Split out from `gopls_health` so a test can fake the answer instead of
    shelling out to a real server it cannot control.
    """
    return run(
        ['gopls', 'symbols', GOPLS_PROBE_FILE],
        cwd=REPO_ROOT,
        timeout=GOPLS_PROBE_TIMEOUT,
    )


def gopls_health(probe: Probe = gopls_probe) -> Answer:
    """Whether gopls answers, and why not when it does not."""
    if which('gopls') is None:
        return Answer(Health.ABSENT, GOPLS_NOT_INSTALLED)
    if not (REPO_ROOT / GOPLS_PROBE_FILE).is_file():
        return Answer(Health.NA, f'no {GOPLS_PROBE_FILE} to probe')

    result = probe()
    if not result.ok:
        return Answer(Health.BROKEN, f'{GOPLS_NOT_ANSWERING}: {result.complaint()}')

    symbols = [line for line in result.out.splitlines() if GOPLS_SYMBOL_LINE.search(line)]
    if not symbols:
        return Answer(
            Health.BROKEN,
            f'{GOPLS_NOT_ANSWERING}: no symbol in its reply for {GOPLS_PROBE_FILE}',
        )
    return Answer(Health.OK, f'{len(symbols)} symbols from {GOPLS_PROBE_FILE}')


# --- pyright --------------------------------------------------------------
#
# The probe file is this module. `gopls_health` carries an NA state for the day
# its probe file is renamed out from under it; a probe that names a file inside
# the running program cannot reach that state, so `pyright_health` never
# returns NA. Repointing this at another file means restoring that branch.
PYRIGHT_PROBE_FILE = 'scripts/le/devtools/servers.py'

# Measured warm on this machine: 0.5s. The first run after install downloads a
# node runtime, which is the slow case this budget is for. Same reasoning as
# GOPLS_PROBE_TIMEOUT: generous enough that a cold run never reds spuriously.
PYRIGHT_PROBE_TIMEOUT = 120

PYRIGHT_NOT_INSTALLED = 'pyright is not installed; the pyright row above installs it'
PYRIGHT_NOT_ANSWERING = 'pyright is present but not answering'


def pyright_probe() -> Result:
    """Ask the language server to analyse one file and report what it did."""
    return run(
        ['pyright', '--outputjson', PYRIGHT_PROBE_FILE],
        cwd=REPO_ROOT,
        timeout=PYRIGHT_PROBE_TIMEOUT,
    )


def pyright_summary(out: str) -> dict[str, Any] | None:
    """The analysis summary from a reply that may carry a bootstrap preamble.

    `json.loads(out)` was wrong here, and it was wrong on exactly one run: the
    first. With no global node on PATH the pipx wrapper installs one, and
    `_install_node_env` (pyright/node.py) runs nodeenv through
    `subprocess.run(args, check=True)` with no redirection, so nodeenv's
    progress lands on the stdout this probe captured. Only the npm path is
    silenced (`install_pyright`, pyright/_utils.py). The reply is then valid
    JSON with text in front of it, the whole decode fails, and setup reds on
    the fresh Linux box it exists to prepare. The second run is green, which
    makes it read as flakiness.

    Returns None when no JSON object in `out` carries a summary. The preamble
    is not itself valid JSON -- nodeenv prints a Python dict repr, single
    quotes -- so scanning is unambiguous rather than a guess at where the reply
    starts.
    """
    decoder = json.JSONDecoder()
    start = out.find('{')
    while start != -1:
        try:
            value, _ = decoder.raw_decode(out, start)
        except ValueError:
            value = None
        if isinstance(value, dict) and isinstance(value.get('summary'), dict):
            summary: dict[str, Any] = value['summary']
            return summary
        start = out.find('{', start + 1)
    return None


def pyright_health(probe: Probe = pyright_probe) -> Answer:
    """Whether pyright answers, and why not when it does not.

    One difference from gopls decides the rest of this: pyright exits 1 when it
    finds a type error, so the exit code says whether the CODE is clean, never
    whether the SERVER worked. A check keyed on it would go red the first time
    somebody's script gained a diagnostic, and a check that reds spuriously is
    a check somebody disables. `summary.filesAnalyzed` is the field that
    answers the question asked here.
    """
    if which('pyright') is None:
        return Answer(Health.ABSENT, PYRIGHT_NOT_INSTALLED)

    result = probe()
    summary = pyright_summary(result.out)
    try:
        analysed = int(summary['filesAnalyzed'])  # type: ignore[index]
    except (ValueError, KeyError, TypeError):
        detail = result.complaint() if result.err.strip() else None
        why = detail or f'no summary in its reply for {PYRIGHT_PROBE_FILE}'
        return Answer(Health.BROKEN, f'{PYRIGHT_NOT_ANSWERING}: {why}')

    if analysed < 1:
        return Answer(
            Health.BROKEN,
            f'{PYRIGHT_NOT_ANSWERING}: analysed no file for {PYRIGHT_PROBE_FILE}',
        )
    return Answer(Health.OK, f'{analysed} file analysed from {PYRIGHT_PROBE_FILE}')
