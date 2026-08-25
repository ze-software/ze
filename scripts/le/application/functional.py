"""Functional `.ci` suites: what each one runs, and the wall-clock budget it runs under.

Ported from mk/test-functional.mk. That file's header lists the two things that
stayed behind and why.

    ./le functional                            the 24 gating suites, in order
    ./le functional --list                     every suite and what it covers
    ./le functional ze-functional-encode-test  one of them
    ./le functional encode-test                the same one, short
    ./le functional --list --json              the suite table, machine-readable

**A suite is named ONCE.** `GATING` is the run list and the progress
denominator, `SUITES` holds what each name runs, and the individual gate and
the gating run read the same record. In Make those were three lists a comment
tied together, and they drifted: `ipsec` sat in `all_suites` with no
`run_suite` line, so it counted toward the denominator, ran nothing, and still
earned every `test/ipsec/*.ci` a merge-gate tier.

THE BUDGET IS THE BEHAVIOUR HERE, not the dispatch. Every suite runs under
`timeout --kill-after=<K> <T>`, in its own process group, because a stuck
subprocess holding an output pipe open makes the runner's own `cmd.Wait()`
block indefinitely after SIGKILL and only `timeout` signals the whole group.
Four things follow from that and each one is a rule this module keeps:

  A kill is REPORTED as a kill. `timeout` answers 124, and before that was
  read as a budget expiry a killed suite looked exactly like a suite whose
  tests were broken: the run printed the N failures the runner had managed to
  emit before the process group died, and nothing said the kill happened. The
  plugin suite produced that on 2026-08-18 at 599.7s against a 600s cap.

  A suite that is CREEPING toward its cap warns while it is still green.
  Raising a cap is not a fix on its own (ai/rules/completion.md), so the number
  has to stay visible as it climbs. The warning point is ZE_SUITE_WARN_PERCENT
  of the budget.

  A slow suite gets its OWN budget rather than raising everybody's. The shared
  cap is what protects the other 23, and `plugin` holds 663 `.ci` files. Its
  1500s is DERIVED: 855s measured on 2026-08-19 (spec verify-scope-4, A-1) on a
  box carrying five other sessions at a load average rising 6.6 -> 18.7 across
  32 cores, and the warning point must sit 40% above that measurement or a
  contended box warns on every run and the warning stops meaning anything.
  855 * 1.40 / 0.80 = 1496s, rounded up to the whole minute. The kill then
  lands at 1.75x the measurement, which is a wedge and not a busy box.

  A budget the report reads and `timeout` does not is worse than no budget: the
  run says 1500s while the kill lands at 600s. `Suite.budget` answers all three
  questions -- the `timeout` argument, the runtime line, and the warning
  arithmetic -- so they cannot disagree. In Make each override owed three
  separate spellings and a test that hunted for all of them.

SUITE CONCURRENCY. `encode` and `plugin` run through the bgp runner, and their
`-p` used to be the constant 8 on every host. 8 is what GitHub's 4-vCPU hosted
runner survives, so a 32-core workstation ran the plugin suite at a quarter of
the width it can carry. It is derived instead: floor at what the smallest
supported host runs today, cap at the core count. The FLOOR keeps CI
byte-identical and mirrors `runner.SuiteConcurrencyFloor`
(internal/test/runner/parallel.go); `functional_test.py` holds the two numbers
equal so neither can move alone. The CAP is measured rather than picked: on a
32-core box the plugin suite runs at 96% parallel efficiency at 8, 88% at 16,
74% at 32 and 36% at 64, and 64 lands inside the two-run spread at 32. Neither
figure transfers to the other 22 suites, which keep `DefaultSuiteConcurrency`'s
2x CPUs: this measurement never covered them.

ISOLATED BINARIES, BY DEFAULT. Every suite builds its OWN throwaway set (ze,
ze-test, ze-stripped) under `<scratch>/testbin-<suffix>/bin/` and runs frozen
against it with `ZE_TEST_NO_BUILD=1`, so a suite never recompiles the dev
binaries mid-run and you can keep building and editing while it runs. The
binaries take the canonical names because `.ci` tests exec `ze` and
`ze-stripped` by bare name, and they live in a `bin/` SUBDIR because ze derives
its config and database directory from its own location and only recognises a
parent named bin or sbin (internal/core/paths/paths.go isBinDir) -- without it
`ze config archive` answers "cannot determine database location".

    ZE_SUFFIX=<name>     a stable directory, KEPT on exit. Not isolated WITHIN a
                         session: two runs that pick one name share the
                         throwaway root's etc/ze and corrupt each other's test
                         database. Reserve it for a single serial run you want
                         to keep, and prefer the default for concurrent work.
    ZE_TEST_CANONICAL=1  opt out: run the session's own ze-test in place, which
                         is what release and CI reproducibility ask for.
    ZE_COVER=1           record which Go packages each suite EXECUTES. The DUT
                         binaries are built `-cover` and each suite gets its own
                         GOCOVERDIR. THE PATH MUST BE ABSOLUTE: a `.ci` that
                         declares tmpfs= runs with proc.Dir set to the per-test
                         directory, so a relative root resolves against THAT
                         directory and the emit fails silently.
    ZE_SKIP_SUITES       comma-separated names the gating run skips, for a
                         Docker environment with no agent-browser or no native
                         process control.

`ze-test` itself is deliberately NOT instrumented: it is the harness, not the
subject, and what it executed is not what the map is about.
"""

from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
import time
from collections.abc import Sequence
from dataclasses import dataclass, field
from pathlib import Path

from le import gateapp
from le.console import echo
from le.devtools.gate import Gate, GateSet, run_gate
from le.devtools.toolchain import toolchain
from le.paths import REPO_ROOT
from le.process import stream

__all__ = [
    'GATES',
    'GATING',
    'SUITES',
    'BinarySet',
    'Options',
    'Run',
    'Suite',
    'action',
    'add_arguments',
    'binary_root',
    'catalogue',
    'command_line',
    'duration_seconds',
    'execute',
    'main',
    'options',
    'parallel',
    'prepare',
    'release',
    'run_gating',
    'run_named',
    'skipped',
    'suite_named',
    'warn_percent',
]

# The shared per-suite wall-clock cap, and the grace `timeout` allows between
# TERM and KILL. Overridable from the environment, which is how a make command
# line reaches here: GNU make puts a command-line variable into the recipe
# environment.
DEFAULT_BUDGET = '600s'
KILL_AFTER = '10s'

# The percentage of its budget a suite may use before the run says so while the
# suite is still green.
DEFAULT_WARN_PERCENT = 80

# A suite that owes more wall clock than the shared cap gives. A name here reads
# ZE_SUITE_TIMEOUT_<SUITE> in place of ZE_SUITE_TIMEOUT, everywhere: the
# `timeout` that kills it, the runtime line, the warning arithmetic, and the
# variable the reports tell you to raise. Lower plugin's when the suite is split
# or made faster, and read the derivation in this module's docstring first.
BUDGET_DEFAULTS: dict[str, str] = {'plugin': '1500s'}

# The smallest default concurrency the two measured suites hand out. Equal to
# runner.SuiteConcurrencyFloor by construction, not by coincidence: it is what
# ZE_PLUGIN_PARALLEL ran the plugin suite at on GitHub's 4-vCPU hosted runner,
# so on a small host the floor wins and CI is byte-identical.
PARALLEL_FLOOR = 8

# The name the isolated set carries and every `.ci` execs by. A gate's argv
# opens with it, and `_command` swaps in the binary this run resolved.
ZE_TEST = 'ze-test'

# The colours the budget report uses. Unconditional, as the recipe's printf was:
# the reader of a failing verify run is looking at a terminal.
RED = '\033[31m'
YELLOW = '\033[33m'
GREEN = '\033[32m'
RESET = '\033[0m'


@dataclass(frozen=True)
class Suite:
    """One functional suite: what it runs, and why it is a suite of its own.

    `args` is the `ze-test` command line, minus the binary. `scaled` says the
    suite takes the derived concurrency; a suite whose `-p` is written into
    `args` has a fixed one, and a suite with neither takes the runner's own
    default.
    """

    name: str
    args: tuple[str, ...]
    why: str
    scaled: bool = False
    chaos: bool = False

    @property
    def target(self) -> str:
        """The make target that runs this suite alone."""
        return f'ze-functional-{self.name}-test'

    @property
    def rerun(self) -> str:
        """The command a failure report tells the reader to type.

        The twin of functionalSuiteRerun (scripts/status/verify_run.go). A
        failure group whose rerun is empty, or names a target make answers with
        `No rule to make target`, is the one group a reader cannot act on.
        """
        return f'make {self.target}'

    @property
    def budget_var(self) -> str:
        """The environment variable that OWNS this suite's budget.

        Telling the reader to raise ZE_SUITE_TIMEOUT when the kill came from
        ZE_SUITE_TIMEOUT_PLUGIN sends them to raise the cap for all 24 suites.
        """
        stem = self.name.upper().replace('-', '_')
        own = f'ZE_SUITE_TIMEOUT_{stem}'
        if self.name in BUDGET_DEFAULTS or os.environ.get(own):
            return own
        return 'ZE_SUITE_TIMEOUT'

    @property
    def budget(self) -> str:
        """This suite's wall-clock cap, as `timeout` spells a duration."""
        var = self.budget_var
        fallback = BUDGET_DEFAULTS.get(self.name, DEFAULT_BUDGET)
        if var != 'ZE_SUITE_TIMEOUT':
            return os.environ.get(var) or fallback
        return os.environ.get(var) or DEFAULT_BUDGET

    def command(self) -> tuple[str, ...]:
        """The suite's own command: the bare binary name and its arguments.

        The name rather than a path, because the path depends on which isolated
        set this run built, and resolving it at import would make every `le`
        command pay for a session lookup it does not need.
        """
        args = self.args
        if self.scaled:
            args = (*args, '-p', parallel(self.name))
        return (ZE_TEST, *args)


def _cores() -> int:
    """The core count the concurrency derivation caps at.

    ZE_SUITE_CORES exists so a test can drive a small host without owning one.
    A count that is missing or not a number falls back to the floor rather than
    to an empty `-p`, which is what the recipe's `[ "$n" -gt "$f" ] 2>/dev/null`
    did with `unknown`.
    """
    raw = os.environ.get('ZE_SUITE_CORES')
    if raw is not None:
        # PRESENT and empty is not the same as absent, and it means the floor.
        # A container that cannot say how many cores it has sets it to nothing,
        # and `?=` did not overwrite an empty command-line value either.
        stripped = raw.strip()
        return int(stripped) if stripped.isdigit() else PARALLEL_FLOOR
    # Affinity rather than the machine's total, which is what `nproc` reports
    # and therefore what the recipe measured.
    if hasattr(os, 'sched_getaffinity'):
        return len(os.sched_getaffinity(0)) or PARALLEL_FLOOR
    return os.cpu_count() or PARALLEL_FLOOR


def parallel(suite: str) -> str:
    """The `-p` one scaled suite runs at: floor at 8, cap at the core count.

    `ZE_<SUITE>_PARALLEL` wins, because an operator's own value must beat a
    derivation, and overriding one suite must not move the other.
    """
    stem = suite.upper().replace('-', '_')
    own = os.environ.get(f'ZE_{stem}_PARALLEL', '').strip()
    if own:
        return own
    return str(max(_cores(), PARALLEL_FLOOR))


def warn_percent() -> int:
    """The percentage of its budget at which a green suite is warned about."""
    raw = os.environ.get('ZE_SUITE_WARN_PERCENT', '').strip()
    return int(raw) if raw.isdigit() else DEFAULT_WARN_PERCENT


def kill_after() -> str:
    """The grace `timeout` allows between its TERM and its KILL."""
    return os.environ.get('ZE_SUITE_KILL_AFTER', '').strip() or KILL_AFTER


SCALE = {'': 1, 's': 1, 'm': 60, 'h': 3600, 'd': 86400}


def duration_seconds(text: str) -> int:
    """A `timeout` duration in seconds, or 0 when it is not one this can measure.

    Zero is the answer for `0s`, for a non-number, and for a fractional duration
    `timeout` accepts and integer arithmetic cannot divide by. The caller prints
    the runtime anyway and skips the percentage, because a budget nobody can
    measure against is not a reason to lose the measurement.
    """
    suffix = text[-1:] if text[-1:] in 'smhd' else ''
    number = text[: -len(suffix)] if suffix else text
    if not number.isdigit():
        return 0
    return int(number) * SCALE[suffix]


# The gating suites, in the order `./le functional` runs them. This list is the
# progress denominator, the run order, and the population from which every
# `.ci`'s verify tier is derived (scripts/dev/rfc_requirements.py,
# functional_suites). A suite missing from here runs only when it is named.
#
# It is a literal, and it is read from the FILE TEXT by two consumers that must
# be able to ask the same question of an older revision: the tier derivation
# above compares this list against HEAD's, and scripts/status/verify_run_test.go
# checks every member has a rerun target. A comprehension over SUITES would say
# the same thing and neither of them could read it.
GATING: tuple[str, ...] = (
    'encode',
    'plugin',
    'parse',
    'decode',
    'reload',
    'ui',
    'editor',
    'managed',
    'l2tp',
    'firewall',
    'policy',
    'ipsec',
    'ldp',
    'rsvpte',
    'isis',
    'ospf',
    'ospfv3',
    'web',
    'install',
    'appliance',
    'l2tp-wire',
    'isis-wire',
    'ospf-wire',
    'runner',
)


SUITES: tuple[Suite, ...] = (
    Suite(
        name='encode',
        args=('bgp', 'encode', '--all'),
        scaled=True,
        why='BGP wire encoding; one of the two suites whose concurrency was measured',
    ),
    Suite(
        name='plugin',
        args=('bgp', 'plugin', '--all'),
        scaled=True,
        why='plugin behavior: 663 .ci files, and the one suite with a budget of its own',
    ),
    Suite(name='parse', args=('bgp', 'parse', '--all'), why='config parsing'),
    Suite(name='decode', args=('bgp', 'decode', '--all'), why='wire decoding'),
    Suite(
        name='reload',
        args=('bgp', 'reload', '--all', '-p', '1'),
        why='config reload; serial, because it shares the kernel routing table with managed',
    ),
    Suite(name='ui', args=('ui', '--all'), why='CLI and completion, against ze-stripped'),
    Suite(name='editor', args=('editor', '--all'), why='the TUI editor (.et files)'),
    Suite(
        name='managed',
        args=('managed', '--all', '-p', '1'),
        why='managed config; serial, because it shares the kernel routing table with reload',
    ),
    Suite(name='l2tp', args=('l2tp', '--all'), why='L2TP'),
    Suite(name='firewall', args=('firewall', '--all'), why='firewall'),
    Suite(name='policy', args=('policy', '--all'), why='policy routing'),
    Suite(
        name='ipsec',
        args=('ipsec', '--all'),
        why=(
            'IPsec/IKEv2 (test/ipsec/*.ci). It was declared gating and dispatched by'
            ' nothing, so it counted toward the progress denominator, never ran, and'
            ' still earned every tag in test/ipsec/ a merge-gate tier'
        ),
    ),
    Suite(name='ldp', args=('ldp', '--all'), why='LDP'),
    Suite(name='rsvpte', args=('rsvpte', '--all'), why='RSVP-TE'),
    Suite(name='isis', args=('isis', '--all'), why='IS-IS config and doctor'),
    Suite(name='ospf', args=('ospf', '--all'), why='OSPF config and doctor'),
    Suite(name='ospfv3', args=('ospfv3', '--all'), why='OSPFv3 config and doctor'),
    Suite(
        name='web',
        args=('web', '--all'),
        chaos=True,
        why='the web UI; the only suite that starts the chaos dashboard (option=server:kind=chaos)',
    ),
    Suite(name='install', args=('install', '--all'), why='installer, PXE, kernel config'),
    Suite(
        name='appliance',
        args=('appliance', '--all'),
        why='the appliance CLI: build, iso, list, serial-login',
    ),
    Suite(name='l2tp-wire', args=('l2tp-wire', '--all'), why='L2TP wire level'),
    Suite(name='isis-wire', args=('isis-wire', '--all'), why='IS-IS wire-level decode'),
    Suite(name='ospf-wire', args=('ospf-wire', '--all'), why='OSPFv2 wire-level decode'),
    Suite(
        name='runner',
        args=('runner', '--all'),
        why=(
            'the test-runner primitives (test/runner/*.ci). Host-safe: it spawns only'
            ' sh and tail helpers, no ze daemon and no privileged tooling, which is why'
            ' it stays in the gating run'
        ),
    ),
    # Shipped, and outside the gating run. Each needs platform tooling or a
    # fixture setup ze-precommit-verify does not provide, so each is release
    # evidence rather than a merge gate, and a `.ci` here earns no verify tier.
    Suite(
        name='static',
        args=('static', '--all'),
        why='static routes; needs the Linux daemon (release evidence only)',
    ),
    Suite(
        name='traffic',
        args=('traffic', '--all'),
        why='traffic control; needs the Linux daemon (release evidence only)',
    ),
    Suite(
        name='flow-export',
        args=('flow-export', '--all'),
        why=(
            'sFlow v5, NetFlow v9 and IPFIX export; needs the Linux daemon and, for'
            ' packet sampling, CAP_NET_ADMIN plus kernel psample (release evidence only)'
        ),
    ),
    Suite(
        name='vpp',
        args=('vpp', '--all'),
        why=(
            'the VPP stub; it carries no -p because its serial default lives in the'
            ' command itself (release evidence only)'
        ),
    ),
    Suite(
        name='vrrp',
        args=('vrrp', '--all'),
        why='VRRP config, show and doctor (release evidence only)',
    ),
)


def suite_named(name: str) -> Suite | None:
    """The suite called `name`, by its bare name or by its make target."""
    for suite in SUITES:
        if name in (suite.name, suite.target):
            return suite
    return None


def catalogue() -> list[dict[str, object]]:
    """Every suite, as plain records. `./le functional --list --json` prints it.

    It exists because the port would otherwise BLIND a guard, which is the
    failure `le gates --json` was written for a day earlier. Two consumers
    outside this program need to know which suites gate: the rerun-target guard
    in scripts/status/verify_run_test.go, and the `.ci` evidence tier in
    scripts/dev/rfc_requirements.py. Both used to read `all_suites="..."` out of
    the makefile recipe. A recipe that delegates names nothing, and a population
    derived from it silently becomes empty, which reads as a passing guard over
    no rows at all.

    `gating` is the field they ask for, and it is derived from `GATING` rather
    than declared beside each suite, so a suite cannot say it gates while the
    run does not run it.
    """
    return [
        {
            'name': suite.name,
            'gating': suite.name in GATING,
            'target': suite.target,
            'rerun': suite.rerun,
            'budget': suite.budget,
            'budget-variable': suite.budget_var,
            'command': list(suite.command()),
            'why': suite.why,
        }
        for suite in SUITES
    ]


def _py(script: str, *args: str) -> tuple[str, ...]:
    return ('python3', f'scripts/dev/{script}', *args)


GATES = GateSet(
    area='functional',
    gates=(
        *(Gate(name=suite.target, argv=suite.command(), why=suite.why) for suite in SUITES),
        Gate(
            name='ze-functional-docker-exec-selftest',
            argv=_py('docker_exec_checked.py', '--selftest'),
            why=(
                'every verdict of the fail-open scan fires on a known fixture, so the'
                ' scan below cannot pass vacuously. It runs first, and the make target'
                ' runs the pair'
            ),
        ),
        Gate(
            name='ze-functional-docker-exec-check',
            argv=_py('docker_exec_checked.py'),
            why=(
                'the fail-open call-site ratchet over the functional harness Python.'
                ' docker_exec_quiet (test/interop/interop.py) returns "" on ANY non-zero'
                ' exit, so a caller that does not test the value for emptiness turns a'
                ' command that FAILED into a passing assertion over nothing. The flagged'
                ' set is derived to a fixpoint, so a new wrapper is covered the day it is'
                ' written, and the floor in test/health/docker-exec-baseline.json may'
                ' only go DOWN'
            ),
        ),
    ),
)


# ─── The isolated binary set ────────────────────────────────────────────────


@dataclass(frozen=True)
class BinarySet:
    """Where the binaries a run executes live, and whether they are throwaway."""

    directory: Path
    remove: bool
    canonical: bool = False

    @property
    def ze_test(self) -> Path:
        return self.directory / ZE_TEST

    def environment(self) -> dict[str, str]:
        """The Go environment plus what freezes the runner against this set.

        A canonical run adds nothing: it IS the session's binary set, and the
        runner rebuilding it in place is the point of asking for it.
        """
        env = toolchain().environment()
        if self.canonical:
            return env
        env['ZE_TEST_NO_BUILD'] = '1'
        env['ZE_BIN'] = str(self.directory / 'ze')
        env['ZE_TEST_BIN'] = str(self.ze_test)
        return env


def _scratch_dir() -> Path:
    """This session's own directory, or `tmp` off-session.

    LOOKED UP, never recomputed. `scripts/dev/session-scratch.sh` is the one
    shell implementation of the rule, shared with the hooks, and make and Go
    each implement it for their own callers (mk/helper-session.mk,
    internal/test/sessionpath). Asking it beats writing a fourth copy, and it
    prints `<session>/scratch`, whose parent is the directory wanted here.

    `ZE_SCRATCH_DIR` in the environment wins, so a make recipe that already
    resolved it hands the answer over rather than paying for it twice.
    """
    named = os.environ.get('ZE_SCRATCH_DIR', '').strip()
    if named:
        return REPO_ROOT / named
    try:
        found = subprocess.run(
            ['scripts/dev/session-scratch.sh', '--path'],
            cwd=REPO_ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return REPO_ROOT / 'tmp'
    printed = found.stdout.strip()
    if found.returncode != 0 or not printed:
        return REPO_ROOT / 'tmp'
    return (REPO_ROOT / printed).parent


def _canonical_bin_dir() -> Path:
    """Where the session's own binaries live: `<session>/bin`, or `bin` off-session."""
    if os.environ.get('ZE_SESSION_ID', '').strip():
        return _scratch_dir() / 'bin'
    return REPO_ROOT / 'bin'


def _cover_root() -> Path | None:
    """The absolute coverage root, or None when ZE_COVER is off.

    ABSOLUTE because a `.ci` that declares tmpfs= runs its process with proc.Dir
    set to the per-test directory, so a relative GOCOVERDIR resolves against
    THAT directory: the emit fails, the Go runtime prints "coverage meta-data
    emit failed" on the child's stderr, and that is silent data loss AND a
    stderr change a `.ci` can assert on.
    """
    if not os.environ.get('ZE_COVER', '').strip():
        return None
    return (_scratch_dir() / 'scratch' / 'covdata').resolve()


def _tags(*parts: str) -> str:
    """One `-tags` string: the parts asked for, plus whatever ZE_TAGS adds."""
    return ' '.join((*parts, *toolchain().extra_tags))


def _build_commands(binaries: Path, *, chaos: bool) -> list[list[str]]:
    """The builds one isolated set needs, in order.

    The DUT build mirrors runner.TestBuildTags (internal/test/runner/runner.go):
    the zetest test plugins, the full command surface, and the default feature
    gates, with NO version ldflags so `ze show version` prints "ze dev"
    (test/parse/cli-version-show.ci). ze-stripped's tags match the Makefile's
    stripped rule. The chaos dashboard is a second compile of cmd/ze under its
    own tags, built BESIDE the ze binary the run uses, which is where
    cmd_web.go looks for it.
    """
    features = toolchain().features
    cover = ['-cover'] if os.environ.get('ZE_COVER', '').strip() else []
    commands = [
        [
            'go',
            'build',
            *cover,
            '-tags',
            _tags('ze_core', 'ze_distro', 'ze_setup', 'zetest', *features),
            '-o',
            str(binaries / 'ze'),
            './cmd/ze',
        ],
        [
            'go',
            'build',
            *cover,
            '-tags',
            _tags('ze_core', 'ze_ssh'),
            '-o',
            str(binaries / 'ze-stripped'),
            './cmd/ze',
        ],
        # NOT instrumented: ze-test is the harness, not the subject.
        [
            'go',
            'build',
            '-tags',
            _tags('ze_test', *features),
            '-o',
            str(binaries / ZE_TEST),
            './cmd/ze',
        ],
    ]
    if chaos:
        commands.append(
            [
                'go',
                'build',
                '-tags',
                _tags('ze_chaos', 'ze_bgp'),
                '-o',
                str(binaries / 'ze-chaos'),
                './cmd/ze',
            ]
        )
    return commands


def binary_root(label: str) -> tuple[Path, bool]:
    """The throwaway root this invocation builds into, and whether to remove it.

    An explicit ZE_SUFFIX is a stable name shared across the run and KEPT, so
    the cleanup is a no-op. Otherwise the name carries the PID and what is being
    run, which is what stops two concurrent invocations, and two suites named on
    one command line, from deleting each other's binaries.
    """
    suffix = os.environ.get('ZE_SUFFIX', '').strip()
    scratch = _scratch_dir()
    if suffix:
        return scratch / f'testbin-{suffix}', False
    return scratch / f'testbin-pid-{os.getpid()}-{label}', True


def prepare(label: str, *, chaos: bool) -> BinarySet | None:
    """Build the set this invocation runs against. None means a build failed.

    In auto mode the directory is scoped by PID and by what is being run, so two
    concurrent invocations, and two suites named on one command line, never let
    one cleanup delete the other's binaries. It sits under the session's own
    directory, so a set survives a crash that stops the cleanup from running and
    is swept with the session rather than left at the tmp/ root.
    """
    if os.environ.get('ZE_TEST_CANONICAL', '').strip():
        return BinarySet(directory=_canonical_bin_dir(), remove=False, canonical=True)

    root, remove = binary_root(label)
    binaries = root / 'bin'
    binaries.mkdir(parents=True, exist_ok=True)
    echo(f'Building isolated test binaries in {binaries}/ (ze, ze-test, ze-stripped)...')
    env = toolchain().environment()
    for argv in _build_commands(binaries, chaos=chaos):
        if stream(argv, cwd=REPO_ROOT, env=env) != 0:
            if remove:
                shutil.rmtree(root, ignore_errors=True)
            return None
    return BinarySet(directory=binaries, remove=remove)


def release(binaries: BinarySet) -> None:
    """Remove a throwaway set. A named or canonical one is left where it is."""
    if binaries.remove:
        shutil.rmtree(binaries.directory.parent, ignore_errors=True)


def command_line(suite: Suite, binaries: BinarySet) -> list[str]:
    """The full argv one suite runs: the cap, then the binary, then the suite.

    `timeout` runs the suite in its own process group and signals the whole
    group on expiry, so leaked grandchildren (ze daemons, tacacs-mocks) die
    with it.
    """
    argv = list(suite.command())
    argv[0] = str(binaries.ze_test)
    return ['timeout', f'--kill-after={kill_after()}', suite.budget, *argv]


def execute(suite: Suite, binaries: BinarySet, *, cover: Path | None = None) -> tuple[int, int]:
    """Run one suite. Returns its exit status and how long it took, in seconds."""
    env = binaries.environment()
    if cover is not None:
        cover.mkdir(parents=True, exist_ok=True)
        env['GOCOVERDIR'] = str(cover)
    started = time.monotonic()
    status = stream(command_line(suite, binaries), cwd=REPO_ROOT, env=env)
    return status, int(time.monotonic() - started)


# ─── The gating run, and its budget report ──────────────────────────────────


@dataclass
class Run:
    """What a gating run has seen so far, and the report it prints as it goes.

    This is `run_suite`'s accounting half. It is a value rather than a set of
    shell accumulators so that a test can drive it with a status and a duration
    of its own choosing and read what it decided.
    """

    suite_total: int = 0
    index: int = 0
    total: int = 0
    failed: int = 0
    failed_names: list[str] = field(default_factory=list)
    expired_names: list[str] = field(default_factory=list)
    warned_names: list[str] = field(default_factory=list)
    skipped_names: list[str] = field(default_factory=list)
    runtimes: list[str] = field(default_factory=list)

    def skip(self, suite: Suite) -> None:
        self.skipped_names.append(suite.name)

    def announce(self, suite: Suite) -> None:
        """Print the progress line, before the suite runs."""
        self.total += 1
        self.index += 1
        echo()
        echo(f'[{self.index}/{self.suite_total}] suite {suite.name}')

    def record(self, suite: Suite, seconds: int, status: int) -> None:
        """Judge one finished suite: its runtime, its budget, and its verdict.

        A kill is reported as a kill and nothing else. A killed suite is at 100%
        of its budget by construction, so saying so a second time as a creep
        warning would bury the line that names the kill.
        """
        budget = suite.budget
        allowed = duration_seconds(budget)
        if allowed > 0:
            percent = seconds * 100 // allowed
            echo(f'      suite {suite.name} took {seconds}s of its {budget} budget ({percent}%)')
            self.runtimes.append(f'  {suite.name} {seconds}s of {budget} ({percent}%)')
        else:
            percent = 0
            echo(
                f'      suite {suite.name} took {seconds}s '
                f'(budget {budget} is not a duration this report can measure against)'
            )
            self.runtimes.append(f'  {suite.name} {seconds}s of {budget} (unmeasurable budget)')

        if status == 124:
            summary = (
                f'suite {suite.name} reached its {budget} wall-clock budget'
                f' ({suite.budget_var}) and was killed'
            )
            echo(
                f'{RED}BUDGET EXPIRED  {summary}.'
                f' The test failures above are that kill, not the product.{RESET}'
            )
            echo('VERIFY FAILURE GROUP: ' + _failure_group(suite, summary))
            self.expired_names.append(suite.name)
        elif allowed > 0 and percent >= warn_percent():
            echo(
                f'{YELLOW}BUDGET WARNING  suite {suite.name} used {percent}% of its'
                f' {budget} budget, and the warning level is {warn_percent()}%.'
                f' Make the suite faster or raise {suite.budget_var}'
                f' before it becomes a kill.{RESET}'
            )
            self.warned_names.append(suite.name)

        if status != 0:
            self.failed += 1
            self.failed_names.append(suite.name)

    def summarise(self) -> int:
        """Print the closing report. Returns the process exit code."""
        shared = os.environ.get('ZE_SUITE_TIMEOUT') or DEFAULT_BUDGET
        echo()
        echo(f'──── suite runtimes (default budget {shared}, warning level {warn_percent()}%) ────')
        for line in self.runtimes:
            echo(line)
        if self.warned_names:
            echo(
                f'{YELLOW}BUDGET WARNING  suite(s) near their budget: '
                f'{" ".join(self.warned_names)}{RESET}'
            )
        if self.expired_names:
            echo(
                f'{RED}BUDGET EXPIRED  suite(s) killed at their budget: '
                f'{" ".join(self.expired_names)}{RESET}'
            )
        if self.skipped_names:
            echo()
            echo(f'{YELLOW}SKIPPED suites (ZE_SKIP_SUITES): {" ".join(self.skipped_names)}{RESET}')

        echo()
        echo('════════════════════════════════════════')
        if self.failed:
            echo(f'{RED}FAIL  {self.failed} suite(s) failed: {" ".join(self.failed_names)}{RESET}')
            echo()
            echo(f'{YELLOW}To run failed suites individually:{RESET}')
            for name in self.failed_names:
                found = suite_named(name)
                if found is not None:
                    echo(f'  {found.rerun}')
            echo()
            return 1
        echo(f'{GREEN}PASS  all {self.total} suites{RESET}')
        echo()
        return 0


def _failure_group(suite: Suite, summary: str) -> str:
    """The declared failure group a cap expiry publishes.

    A cap expiry is the one failure that already cost the run its whole budget,
    so it must not also be the only one with no next step: `rerun` carries the
    same command an ordinary suite failure gets from functionalSuiteRerun
    (scripts/status/verify_run.go), and `kind` is timeout rather than a nameless
    suite failure.
    """
    return json.dumps(
        {
            'stage': suite.name,
            'group-id': f'suite-budget:{suite.name}',
            'kind': 'timeout',
            'related': [suite.name],
            'summary': summary,
            'rerun': suite.rerun,
            'parallel': 'stage',
        },
        separators=(',', ':'),
    )


def skipped() -> set[str]:
    """The suites ZE_SKIP_SUITES asks the gating run to leave out."""
    return {name for name in os.environ.get('ZE_SKIP_SUITES', '').split(',') if name}


def run_gating() -> int:
    """Run every gating suite, in order, under its own budget.

    One isolated binary set serves the whole run, built once at the top exactly
    as the recipe built it, and removed however the run ends.
    """
    skip = skipped()
    suites = [suite for name in GATING if (suite := suite_named(name)) is not None]
    run = Run(suite_total=len([s for s in suites if s.name not in skip]))

    binaries = prepare('ze-functional-test', chaos=True)
    if binaries is None:
        return 1
    cover_root = _cover_root()
    try:
        for suite in suites:
            if suite.name in skip:
                run.skip(suite)
                continue
            run.announce(suite)
            cover = cover_root / suite.name if cover_root is not None else None
            if cover is not None:
                shutil.rmtree(cover, ignore_errors=True)
            status, seconds = execute(suite, binaries, cover=cover)
            run.record(suite, seconds, status)
            if cover is not None and cover_root is not None:
                _reduce_coverage(suite, cover, cover_root)
    finally:
        release(binaries)
    return run.summarise()


def _reduce_coverage(suite: Suite, cover: Path, root: Path) -> None:
    """Reduce one suite's raw coverage directory to the packages it reached."""
    files = sum(1 for path in cover.rglob('*') if path.is_file())
    size = sum(path.stat().st_size for path in cover.rglob('*') if path.is_file()) // 1024
    with (root / 'raw-size.txt').open('a', encoding='utf-8') as handle:
        handle.write(f'{suite.name} {files} {size}\n')
    percent = subprocess.run(
        ['go', 'tool', 'covdata', 'percent', f'-i={cover}'],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
        env=toolchain().environment(),
        check=False,
    )
    (root / f'{suite.name}.percent').write_text(percent.stdout + percent.stderr, encoding='utf-8')
    if percent.returncode != 0:
        echo(f'covdata percent failed for suite {suite.name}')
    shutil.rmtree(cover, ignore_errors=True)


def run_named(names: Sequence[str]) -> int:
    """Run the gates the caller named, each under the budget it owns.

    A suite named here gets the cap and nothing else: no progress line, no
    runtime report, and the suite's own exit code, which is what the individual
    make target has always done. The budget report belongs to the run that has
    23 other suites waiting behind this one.

    The first failure STOPS the run, which is what `make a b` does and what the
    two-command docker-exec target did. The pair is the case that needs it: its
    `--selftest` proves the scan's verdicts fire, so a scan that runs after a
    failed selftest reports findings from a checker that has just been shown to
    be broken.
    """
    selected: list[Gate] = []
    for name in names:
        gate = GATES.find(name)
        if gate is None:
            echo(f'no such gate in {GATES.area}: {name}')
            echo(f'try one of: {", ".join(GATES.names())}')
            return 2
        selected.append(gate)

    suites = [suite for gate in selected if (suite := suite_named(gate.name)) is not None]
    binaries: BinarySet | None = None
    if suites:
        label = suites[0].target if len(suites) == 1 else 'ze-functional-test'
        binaries = prepare(label, chaos=any(suite.chaos for suite in suites))
        if binaries is None:
            return 1

    try:
        for gate in selected:
            suite = suite_named(gate.name)
            if suite is not None and binaries is not None:
                status, _ = execute(suite, binaries)
            else:
                status = run_gate(gate, env=gateapp.default_environment(gate))
            if status != 0:
                return status
    finally:
        if binaries is not None:
            release(binaries)
    return 0


def add_arguments(parser: argparse.ArgumentParser) -> None:
    gateapp.add_arguments(parser, GATES)


def options(namespace: argparse.Namespace) -> gateapp.Options:
    return gateapp.options(namespace)


def action(opts: gateapp.Options) -> int:
    """Run what the options select.

    A bare `./le functional` is the GATING run -- the 24 suites in `GATING`,
    with the progress denominator and the budget report -- rather than the
    shared helper's "every check in the area". The area holds five more suites
    that ship and do not gate, and a sweep that ran them would be a different
    command from `make ze-functional-test` wearing its name.
    """
    if opts.listing and opts.as_json:
        # Nothing but the document on stdout: a caller asked for JSON and is
        # going to parse what comes back.
        print(json.dumps(catalogue(), indent=2, sort_keys=True))
        return 0
    if opts.listing or opts.as_json or opts.write:
        return gateapp.action(opts, GATES)
    if opts.names:
        return run_named(opts.names)
    return run_gating()


def main(argv: Sequence[str] | None = None) -> int:
    return gateapp.main(argv, GATES, __doc__, run=action)


Options = gateapp.Options

if __name__ == '__main__':
    sys.exit(main())
