#!/usr/bin/env python3
"""Tests for the per-suite wall-clock budget and the suite table behind it.

Ported from scripts/dev/functional_suite_test.py, which tested the same
behaviour by extracting `run_suite` out of mk/test-functional.mk as TEXT,
substituting make's variables into it, and driving the resulting shell with a
stub command. The subject moved to `le/application/functional.py`, so the
driving moved with it: `Run.record` is the accounting half of `run_suite`, and
a test hands it a status and a duration of its own choosing.

Every behaviour that file pinned has a counterpart here. What did NOT come
across is the half that asserted the FILE's shape rather than a behaviour --
that a variable was spelled with `?=`, that `SUITE_RUN_PLUGIN` existed, that
each budget appeared on three separate lines, that `run_suite` had not been
reformatted. Those had a subject because Make could express a budget in three
places that disagreed. `Suite.budget` answers all three questions from one
property, so there is nothing left to hold in step, and the tests that hunted
for the three spellings are replaced by one that asserts the timeout argument,
the runtime line and the warning all carry the same number.

Two behaviours the makefile had and NOTHING tested are covered here: which
directory an isolated binary set lands in, and that ZE_SKIP_SUITES leaves a
suite out of the denominator as well as out of the run.
"""

from __future__ import annotations

import io
import json
import os
import re
import sys
import unittest
from collections.abc import Iterator
from contextlib import contextmanager, redirect_stdout
from pathlib import Path
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from le.application import functional

REPO = Path(__file__).resolve().parents[2]

# The Go constant the module's floor must agree with, and the file that holds
# it. Two numbers that must be equal and live in different languages drift
# silently; this pair is held equal by TestSuiteConcurrency below.
PARALLEL_GO = REPO / 'internal' / 'test' / 'runner' / 'parallel.go'
GO_FLOOR_RE = re.compile(r'(?m)^const SuiteConcurrencyFloor = (\d+)$')
VPP_GO = REPO / 'internal' / 'test' / 'cli' / 'cmd_vpp.go'

# Every make target the build system declares. A suite a run can fail on must be
# a suite a developer can re-run, so the reports name `make ze-functional-<suite>-test`
# for every member and this file holds that name true against the build system.
MAKE_TARGET_RE = re.compile(r'(?m)^([A-Za-z0-9_.-]+)[ \t]*:(?:[^=]|$)')

# A suite with no budget of its own, used wherever a test drives the shared cap.
# Naming the overridden suite there would make the assertion read the override
# and pass for the wrong reason.
SHARED = 'encode'

# The plugin suite's budget is derived from a measurement, not chosen round.
# Spec verify-scope-4, A-1: the suite ran to completion in 855s on 2026-08-19,
# on a box carrying five other sessions at a load average rising 6.6 -> 18.7
# across 32 cores.
PLUGIN_MEASURED_SECONDS = 855

# The warning point must sit this far above that measurement. Below it a busy
# box warns on every run, and a warning that fires every run names no creep.
PLUGIN_WARN_HEADROOM = 1.40

# Everything a test may want to choose. Cleared before each case so a value
# inherited from the shell this run started in cannot decide an assertion.
TUNABLES = (
    'ZE_SUITE_TIMEOUT',
    'ZE_SUITE_TIMEOUT_PLUGIN',
    'ZE_SUITE_TIMEOUT_ENCODE',
    'ZE_SUITE_KILL_AFTER',
    'ZE_SUITE_WARN_PERCENT',
    'ZE_SUITE_CORES',
    'ZE_ENCODE_PARALLEL',
    'ZE_PLUGIN_PARALLEL',
    'ZE_SKIP_SUITES',
    'ZE_SUFFIX',
    'ZE_TEST_CANONICAL',
    'ZE_SCRATCH_DIR',
    'ZE_SESSION_ID',
    'ZE_COVER',
)


@contextmanager
def environment(**values: str) -> Iterator[None]:
    """Run the block with only the tunables the caller named."""
    patched = dict(os.environ)
    for name in TUNABLES:
        patched.pop(name, None)
    patched.update(values)
    with mock.patch.dict(os.environ, patched, clear=True):
        yield


def declared_make_targets() -> set[str]:
    """Every make target the Makefile and mk/*.mk fragments declare."""
    corpus = [(REPO / 'Makefile').read_text()]
    fragments = sorted((REPO / 'mk').glob('*.mk'))
    if not fragments:
        raise AssertionError('no mk/*.mk fragments found: layout changed?')
    corpus.extend(path.read_text() for path in fragments)
    targets = set(MAKE_TARGET_RE.findall('\n'.join(corpus)))
    if len(targets) < 50:
        raise AssertionError(
            f'parsed only {len(targets)} make targets; the rule-head regex has rotted'
        )
    return targets


class SuiteRun:
    """One driven call of the accounting half, and everything it left behind."""

    def __init__(self, stdout: str, run: functional.Run) -> None:
        self.stdout = stdout
        self.run = run

    @property
    def failed(self) -> int:
        return self.run.failed

    @property
    def runtimes(self) -> str:
        return '\n'.join(self.run.runtimes)

    def line(self, needle: str) -> str:
        found = [line for line in self.stdout.splitlines() if needle in line]
        if len(found) != 1:
            raise AssertionError(f'expected one {needle!r} line, got {len(found)}:\n{self.stdout}')
        return found[0]

    def failure_group(self) -> dict[str, object]:
        prefix = 'VERIFY FAILURE GROUP: '
        payloads = [
            line.partition(prefix)[2] for line in self.stdout.splitlines() if prefix in line
        ]
        if len(payloads) != 1:
            raise AssertionError(
                f'expected exactly one {prefix!r} line, got {len(payloads)}:\n{self.stdout}'
            )
        decoded: dict[str, object] = json.loads(payloads[0])
        return decoded


def drive(status: int, *, suite: str = SHARED, seconds: int = 0, **values: str) -> SuiteRun:
    """Run the accounting half over one suite that took `seconds` and exited `status`."""
    found = functional.suite_named(suite)
    assert found is not None, suite
    run = functional.Run(suite_total=1)
    out = io.StringIO()
    with environment(**values), redirect_stdout(out):
        run.announce(found)
        run.record(found, seconds, status)
    return SuiteRun(out.getvalue(), run)


class TestCapExpiryIsReportedDistinctly(unittest.TestCase):
    """A suite killed by its cap says so, and still fails.

    Driven over a suite on the SHARED budget. The suite this was written for is
    `plugin`, which has a budget of its own, and driving it here would read that
    budget instead of the one each test sets. TestPerSuiteBudget covers it.
    """

    def test_exit_124_names_the_suite_and_its_budget(self) -> None:
        run = drive(124, ZE_SUITE_TIMEOUT='600s')
        expired = run.line('BUDGET EXPIRED')
        self.assertIn(SHARED, expired)
        self.assertIn('600s', expired)
        self.assertIn('ZE_SUITE_TIMEOUT', expired)

    def test_exit_124_stays_a_suite_failure(self) -> None:
        run = drive(124)
        self.assertEqual(1, run.failed)
        self.assertEqual([SHARED], run.run.failed_names)
        self.assertEqual([SHARED], run.run.expired_names)

    def test_exit_124_publishes_a_timeout_failure_group(self) -> None:
        group = drive(124, ZE_SUITE_TIMEOUT='600s').failure_group()
        self.assertEqual(SHARED, group['stage'])
        self.assertEqual('timeout', group['kind'])
        self.assertEqual(f'suite-budget:{SHARED}', group['group-id'])
        self.assertEqual([SHARED], group['related'])
        self.assertIn('600s', str(group['summary']))

    def test_an_ordinary_failure_is_not_reported_as_a_cap_expiry(self) -> None:
        run = drive(1)
        self.assertNotIn('BUDGET EXPIRED', run.stdout)
        self.assertNotIn('VERIFY FAILURE GROUP:', run.stdout)
        self.assertEqual(1, run.failed)
        self.assertEqual([SHARED], run.run.failed_names)
        self.assertEqual([], run.run.expired_names)

    def test_a_passing_suite_reports_neither(self) -> None:
        run = drive(0)
        self.assertNotIn('BUDGET EXPIRED', run.stdout)
        self.assertNotIn('BUDGET WARNING', run.stdout)
        self.assertEqual(0, run.failed)
        self.assertEqual([], run.run.failed_names)
        self.assertEqual([], run.run.expired_names)

    def test_a_cap_expiry_is_not_also_a_creep_warning(self) -> None:
        # A killed suite is at 100% of its budget by construction. Saying so a
        # second time as a warning would bury the line that names the kill.
        run = drive(124, seconds=1, ZE_SUITE_TIMEOUT='1s')
        self.assertIn('BUDGET EXPIRED', run.stdout)
        self.assertNotIn('BUDGET WARNING', run.stdout)
        self.assertEqual([], run.run.warned_names)


class TestSuiteRuntimeRecorded(unittest.TestCase):
    """The runtime is recorded per suite, and creep warns before it kills."""

    def test_runtime_is_reported_against_the_budget(self) -> None:
        run = drive(0, seconds=12, ZE_SUITE_TIMEOUT='600s')
        self.assertRegex(run.stdout, r'suite encode took 12s of its 600s budget \(2%\)')

    def test_runtime_is_accumulated_for_the_end_of_run_summary(self) -> None:
        run = drive(0, seconds=12, ZE_SUITE_TIMEOUT='600s')
        self.assertRegex(run.runtimes, r'encode 12s of 600s \(2%\)')

    def test_a_suite_below_the_warning_level_does_not_warn(self) -> None:
        run = drive(0, seconds=12, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_WARN_PERCENT='80')
        self.assertNotIn('BUDGET WARNING', run.stdout)
        self.assertEqual([], run.run.warned_names)

    def test_a_suite_at_the_warning_level_warns_while_still_green(self) -> None:
        run = drive(0, seconds=1, ZE_SUITE_TIMEOUT='1s', ZE_SUITE_WARN_PERCENT='80')
        self.assertIn('BUDGET WARNING', run.stdout)
        self.assertEqual(['encode'], run.run.warned_names)
        self.assertEqual(0, run.failed, 'a warning must not turn a green suite red')

    def test_the_budget_unit_is_read_rather_than_assumed(self) -> None:
        # `timeout` takes s, m, h, d and a bare number of seconds. A minute
        # budget read as 2 seconds would warn on every suite.
        run = drive(0, seconds=1, ZE_SUITE_TIMEOUT='2m')
        self.assertRegex(run.stdout, r'suite encode took 1s of its 2m budget \(0%\)')
        self.assertNotIn('BUDGET WARNING', run.stdout)

    def test_a_bare_number_is_seconds(self) -> None:
        run = drive(0, seconds=6, ZE_SUITE_TIMEOUT='600')
        self.assertRegex(run.stdout, r'suite encode took 6s of its 600 budget \(1%\)')

    def test_an_unusable_budget_says_so_instead_of_dividing_by_zero(self) -> None:
        # 0s is the bottom of the range: it kills every suite immediately, and
        # it is not a denominator. The runtime is still recorded.
        for budget in ('0s', 'abc', '1.5m'):
            with self.subTest(budget=budget):
                run = drive(0, seconds=3, ZE_SUITE_TIMEOUT=budget)
                self.assertRegex(
                    run.stdout,
                    rf'suite encode took 3s \(budget {re.escape(budget)} is not a duration',
                )
                self.assertIn('unmeasurable budget', run.runtimes)
                self.assertNotIn('BUDGET WARNING', run.stdout)
                self.assertEqual(0, run.failed)


class TestTheShippedBudgets(unittest.TestCase):
    """The numbers a run gets when nobody overrides anything.

    The Make version of this drove `make --dry-run` to see the recipe as MAKE
    expanded it, because the test's own variable substitution could not see a
    `$$` written as `$` or a default the recipe never read. There is no
    expansion step here, so what survives is the half that mattered: an
    un-overridden run reports the budget the repository ships.
    """

    def test_a_shared_suite_gets_the_shipped_cap(self) -> None:
        run = drive(0, seconds=1)
        self.assertRegex(
            run.stdout, rf'suite encode took 1s of its {functional.DEFAULT_BUDGET} budget'
        )

    def test_plugin_gets_the_budget_the_repository_ships(self) -> None:
        shipped = functional.BUDGET_DEFAULTS['plugin']
        self.assertNotEqual(
            functional.DEFAULT_BUDGET, shipped, 'the override must differ from the cap'
        )
        run = drive(0, suite='plugin', seconds=1)
        self.assertRegex(run.stdout, rf'suite plugin took 1s of its {shipped} budget')


class TestPerSuiteBudget(unittest.TestCase):
    """A suite named in ZE_SUITE_TIMEOUT_<SUITE> runs on its own budget.

    The plugin suite holds 663 `.ci` files and needs more wall clock than the
    600s the other 23 suites share. Raising the shared cap to fit it takes that
    margin away from all of them, which is what the cap exists to give them, so
    the budget is per suite and the shared default stays 600s.
    """

    def test_the_overridden_suite_uses_its_own_budget(self) -> None:
        run = drive(
            0, suite='plugin', seconds=1, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_PLUGIN='1500s'
        )
        self.assertRegex(run.stdout, r'suite plugin took 1s of its 1500s budget')
        self.assertNotIn('600s', run.stdout)

    def test_a_suite_without_an_override_keeps_the_shared_budget(self) -> None:
        run = drive(0, seconds=1, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_PLUGIN='1500s')
        self.assertRegex(run.stdout, r'suite encode took 1s of its 600s budget')
        self.assertNotIn('1500s', run.stdout)

    def test_the_runtime_summary_row_carries_the_suites_own_budget(self) -> None:
        run = drive(
            0, suite='plugin', seconds=1, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_PLUGIN='1500s'
        )
        self.assertRegex(run.runtimes, r'plugin 1s of 1500s \(0%\)')

    def test_the_warning_is_measured_against_the_suites_own_budget(self) -> None:
        # The discriminating case: against the shared 1s budget this suite is at
        # 100% and warns; against its own 600s it is at 0%. A warning computed
        # from the shared default would make the overridden suite warn on every
        # run, and a warning that always fires names no creep.
        run = drive(
            0, suite='plugin', seconds=1, ZE_SUITE_TIMEOUT='1s', ZE_SUITE_TIMEOUT_PLUGIN='600s'
        )
        self.assertNotIn('BUDGET WARNING', run.stdout)
        self.assertEqual([], run.run.warned_names)

    def test_the_warning_still_fires_when_the_suites_own_budget_is_the_tight_one(self) -> None:
        run = drive(
            0, suite='plugin', seconds=1, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_PLUGIN='1s'
        )
        warning = run.line('BUDGET WARNING')
        self.assertIn('of its 1s budget', warning)
        self.assertIn('raise ZE_SUITE_TIMEOUT_PLUGIN', warning)
        self.assertEqual(['plugin'], run.run.warned_names)
        self.assertEqual(0, run.failed, 'a warning must not turn a green suite red')

    def test_a_cap_expiry_names_the_variable_that_owns_the_budget(self) -> None:
        # Telling the reader to raise ZE_SUITE_TIMEOUT when the kill came from
        # ZE_SUITE_TIMEOUT_PLUGIN sends them to raise the cap for all 24 suites.
        for suite, budget, variable in (
            ('plugin', '1500s', 'ZE_SUITE_TIMEOUT_PLUGIN'),
            (SHARED, '600s', 'ZE_SUITE_TIMEOUT'),
        ):
            with self.subTest(suite=suite):
                run = drive(
                    124,
                    suite=suite,
                    ZE_SUITE_TIMEOUT='600s',
                    ZE_SUITE_TIMEOUT_PLUGIN='1500s',
                )
                wanted = f'its {budget} wall-clock budget ({variable})'
                self.assertIn(wanted, run.line('BUDGET EXPIRED'))
                self.assertIn(wanted, str(run.failure_group()['summary']))

    def test_any_suite_may_be_given_a_budget_of_its_own(self) -> None:
        # In Make an override owed a SUITE_RUN_<SUITE>, an arm in run_suite's
        # `case`, and that SUITE_RUN_<SUITE> on two separate lines, so only the
        # one suite anybody had written all four for could have a budget. The
        # variable is derived from the suite's name now, so encode gets one by
        # setting it and nothing else.
        run = drive(0, seconds=1, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_ENCODE='900s')
        self.assertRegex(run.stdout, r'suite encode took 1s of its 900s budget')
        self.assertIn(
            'ZE_SUITE_TIMEOUT_ENCODE',
            drive(124, ZE_SUITE_TIMEOUT='600s', ZE_SUITE_TIMEOUT_ENCODE='900s').line(
                'BUDGET EXPIRED'
            ),
        )


class TestSuiteBudgetContract(unittest.TestCase):
    """The cap's own shape: finite, overridable, and applied through `timeout`."""

    def setUp(self) -> None:
        self.binaries = functional.BinarySet(directory=Path('/nowhere/bin'), remove=False)

    def test_the_cap_is_overridable_from_the_environment(self) -> None:
        # Which is how a make command line reaches it: GNU make puts a
        # command-line variable into the recipe environment.
        with environment(
            ZE_SUITE_TIMEOUT='1200s',
            ZE_SUITE_TIMEOUT_PLUGIN='1800s',
            ZE_SUITE_KILL_AFTER='30s',
            ZE_SUITE_WARN_PERCENT='55',
        ):
            encode = functional.suite_named('encode')
            plugin = functional.suite_named('plugin')
            assert encode is not None and plugin is not None
            self.assertEqual('1200s', encode.budget)
            self.assertEqual('1800s', plugin.budget)
            self.assertEqual('30s', functional.kill_after())
            self.assertEqual(55, functional.warn_percent())

    def test_the_cap_stays_finite_and_kills_the_process_group(self) -> None:
        # Removing the cap reopens the hang it was added for: a stuck subprocess
        # holding an output pipe made cmd.Wait() block indefinitely, and only
        # `timeout` signals the whole process group.
        with environment():
            suite = functional.suite_named(SHARED)
            assert suite is not None
            argv = functional.command_line(suite, self.binaries)
        self.assertEqual('timeout', argv[0])
        self.assertEqual(f'--kill-after={functional.KILL_AFTER}', argv[1])
        self.assertEqual(functional.DEFAULT_BUDGET, argv[2])
        self.assertRegex(functional.DEFAULT_BUDGET, r'^\d+[smhd]?$', 'the cap must be finite')
        self.assertRegex(functional.KILL_AFTER, r'^\d+[smhd]?$')

    def test_every_per_suite_budget_is_applied_on_every_path(self) -> None:
        # A budget the report reads and `timeout` does not is worse than no
        # override: the run says 1500s while the kill lands at 600s. One
        # property answers all three questions, and this is the assertion that
        # they are one answer.
        for name, budget in functional.BUDGET_DEFAULTS.items():
            with self.subTest(suite=name):
                self.assertRegex(budget, r'^\d+[smhd]?$', 'a per-suite budget must be finite')
                with environment():
                    suite = functional.suite_named(name)
                    assert suite is not None
                    self.assertEqual(budget, functional.command_line(suite, self.binaries)[2])
                run = drive(124, suite=name, seconds=1)
                self.assertIn(f'of its {budget} budget', run.stdout)
                self.assertIn(f'its {budget} wall-clock budget', run.line('BUDGET EXPIRED'))

    def test_the_plugin_budget_keeps_its_warning_above_the_measurement(self) -> None:
        # The number is derived, not picked: 855s measured, and the warning
        # point must sit 40% above it, so 855 * 1.40 / 0.80 = 1496s rounded up
        # to the whole minute. Lowering the budget without making the suite
        # faster puts the warning back on top of a normal run.
        budget = functional.duration_seconds(functional.BUDGET_DEFAULTS['plugin'])
        warn_point = budget * functional.DEFAULT_WARN_PERCENT / 100
        self.assertGreaterEqual(
            warn_point,
            PLUGIN_MEASURED_SECONDS * PLUGIN_WARN_HEADROOM,
            f'the plugin suite measured {PLUGIN_MEASURED_SECONDS}s, so a '
            f'{functional.DEFAULT_WARN_PERCENT}% warning at {warn_point:.0f}s leaves less '
            f'than {PLUGIN_WARN_HEADROOM:.0%} of headroom and will fire on a busy box',
        )
        self.assertGreater(
            budget,
            functional.duration_seconds(functional.DEFAULT_BUDGET),
            'an override below the shared cap is not an override',
        )


class TestSuiteConcurrency(unittest.TestCase):
    """The two measured suites size themselves to the host, within bounds.

    Their `-p` was the constant 8, which is what GitHub's 4-vCPU hosted runner
    survives. Every host got that number, so a 32-core box ran the 663-test
    plugin suite at a quarter of the width it can carry. The floor keeps the
    small host exactly where it was; the cap is the core count, past which the
    measured parallel efficiency falls to 36%.
    """

    def concurrency(self, **values: str) -> dict[str, str]:
        with environment(**values):
            return {name: functional.parallel(name) for name in ('plugin', 'encode')}

    def test_concurrency_is_derived_not_pinned(self) -> None:
        for cores in ('16', '32', '64'):
            with self.subTest(cores=cores):
                got = self.concurrency(ZE_SUITE_CORES=cores)
                self.assertEqual(cores, got['plugin'])
                self.assertEqual(
                    cores,
                    got['encode'],
                    'encode was measured separately and moves with plugin only because '
                    'both were pinned at the same constant',
                )

    def test_small_host_keeps_the_floor(self) -> None:
        go_floor = GO_FLOOR_RE.search(PARALLEL_GO.read_text())
        self.assertIsNotNone(go_floor, f'{PARALLEL_GO}: SuiteConcurrencyFloor moved or was renamed')
        assert go_floor is not None
        self.assertEqual(
            go_floor.group(1),
            str(functional.PARALLEL_FLOOR),
            'the module floor and runner.SuiteConcurrencyFloor are the same measured '
            'figure; one may not move without the other',
        )
        # A 4-vCPU CI runner, and the degenerate inputs a container can produce.
        for cores in ('1', '2', '4', '8', '', 'unknown'):
            with self.subTest(cores=cores):
                got = self.concurrency(ZE_SUITE_CORES=cores)
                self.assertEqual(
                    str(functional.PARALLEL_FLOOR),
                    got['plugin'],
                    'a host at or below the floor, or one that cannot say how many cores '
                    'it has, must get exactly what CI runs today',
                )
                self.assertEqual(str(functional.PARALLEL_FLOOR), got['encode'])

    def test_explicit_parallel_wins(self) -> None:
        got = self.concurrency(ZE_SUITE_CORES='32', ZE_PLUGIN_PARALLEL='3')
        self.assertEqual('3', got['plugin'], "an operator's own value must beat the derivation")
        self.assertEqual('32', got['encode'], 'overriding one suite must not move the other')

    def test_the_scaled_suites_carry_the_derived_value_into_the_command(self) -> None:
        binaries = functional.BinarySet(directory=Path('/nowhere/bin'), remove=False)
        with environment(ZE_SUITE_CORES='16'):
            for name in ('encode', 'plugin'):
                with self.subTest(suite=name):
                    suite = functional.suite_named(name)
                    assert suite is not None
                    self.assertEqual(['-p', '16'], functional.command_line(suite, binaries)[-2:])

    def test_serial_suites_stay_serial(self) -> None:
        # register.go records that reload and managed share the kernel routing
        # table, so they run one test at a time. In Make each suite was spelled
        # twice -- once on the aggregate line and once in its own target -- and
        # a `-p` that survived in only one of them made the suite serial for
        # `make ze-functional-test` and parallel for the developer re-running
        # it. There is one spelling now, and this asserts that: the gating run
        # and the individual gate read the same record.
        binaries = functional.BinarySet(directory=Path('/nowhere/bin'), remove=False)
        with environment():
            for name in ('reload', 'managed'):
                with self.subTest(suite=name):
                    suite = functional.suite_named(name)
                    assert suite is not None
                    self.assertEqual(['-p', '1'], functional.command_line(suite, binaries)[-2:])
                    gate = functional.GATES.find(suite.target)
                    self.assertIsNotNone(gate)
                    assert gate is not None
                    self.assertEqual(suite.command(), gate.argv)
        # vpp carries no -p here: its serial default lives in the command
        # itself, and this was never measured either.
        vpp = functional.suite_named('vpp')
        assert vpp is not None
        self.assertNotIn('-p', vpp.args)
        self.assertFalse(vpp.scaled)
        self.assertRegex(
            VPP_GO.read_text(),
            r'fs\.IntVar\(&cli\.parallel, "p", 1,',
            f"{VPP_GO}: the vpp suite's default concurrency must stay 1",
        )


class TestEverySuiteCanBeRerun(unittest.TestCase):
    """A suite a run can fail on is a suite a developer can re-run.

    Nothing executes the command a failure report prints, so a suite added to
    the gating list without its own target leaves the report naming a target
    make answers with `No rule to make target`. That is how `make ze-<suite>-test`
    survived for all 24 suites.
    """

    def test_every_gating_suite_has_an_individual_target(self) -> None:
        targets = declared_make_targets()
        for name in functional.GATING:
            with self.subTest(suite=name):
                suite = functional.suite_named(name)
                assert suite is not None
                self.assertIn(
                    suite.target,
                    targets,
                    f'suite {name} gates, so a failed run names `{suite.rerun}`; '
                    f'that target must exist',
                )

    def test_every_shipped_suite_has_an_individual_target(self) -> None:
        # The non-gating half too: a suite with a record and no target is a
        # suite the release-evidence matrix cannot run.
        targets = declared_make_targets()
        for suite in functional.SUITES:
            with self.subTest(suite=suite.name):
                self.assertIn(suite.target, targets)

    def test_the_failed_suite_hint_names_that_target_family(self) -> None:
        targets = {f'make {name}' for name in declared_make_targets()}
        for suite in functional.SUITES:
            with self.subTest(suite=suite.name):
                self.assertEqual(f'make ze-functional-{suite.name}-test', suite.rerun)
                self.assertIn(suite.rerun, targets)


class TestCapExpiryTellsTheReaderWhatToRun(unittest.TestCase):
    """The budget failure group carries the same rerun as an ordinary one.

    classifyFunctional (scripts/status/verify_run.go) fills `rerun` from
    functionalSuiteRerun for a plain suite failure. A cap expiry publishes its
    own group, so an empty field there makes the one failure that already cost
    the run its whole budget the only one with no next step.
    """

    def test_the_budget_group_names_the_suites_own_target(self) -> None:
        targets = declared_make_targets()
        for name in (SHARED, 'install'):
            with self.subTest(suite=name):
                suite = functional.suite_named(name)
                assert suite is not None
                group = drive(124, suite=name).failure_group()
                self.assertEqual(suite.rerun, group.get('rerun', ''))
                self.assertIn(suite.target, targets)


class TestTheSuiteListIsOneList(unittest.TestCase):
    """Declaration and dispatch are the same list, and cannot drift.

    `ipsec` sat in the makefile's `all_suites` with no `run_suite` line: it
    counted toward the progress denominator, ran nothing, and still earned
    every `test/ipsec/*.ci` a merge-gate tier, because a comment was all that
    tied the two lists together. `scripts/dev/rfc_requirements.py` had to check
    for that. It cannot happen here -- the gating run resolves each name to the
    record it runs -- and this is the assertion that keeps it so.
    """

    def test_every_gating_name_resolves_to_a_suite(self) -> None:
        for name in functional.GATING:
            with self.subTest(suite=name):
                self.assertIsNotNone(
                    functional.suite_named(name),
                    f'{name} gates and nothing runs it',
                )

    def test_the_gating_list_holds_no_duplicate(self) -> None:
        self.assertEqual(len(set(functional.GATING)), len(functional.GATING))

    def test_no_suite_is_declared_twice(self) -> None:
        names = [suite.name for suite in functional.SUITES]
        self.assertEqual(len(set(names)), len(names))

    def test_the_gating_population_is_the_one_the_tier_derivation_reads(self) -> None:
        # scripts/dev/rfc_requirements.py derives every `.ci`'s verify tier from
        # this list, so its size is a fact about what the merge gate proves. The
        # bound is the same one that file's own guard uses: a parse that finds
        # fewer than 20 has rotted.
        self.assertGreaterEqual(len(functional.GATING), 20)
        self.assertIn('plugin', functional.GATING)
        self.assertIn('editor', functional.GATING)
        self.assertNotIn('traffic', functional.GATING)

    def test_a_shipped_suite_outside_the_gating_list_is_not_run_by_a_bare_sweep(self) -> None:
        outside = [s.name for s in functional.SUITES if s.name not in functional.GATING]
        self.assertEqual(['static', 'traffic', 'flow-export', 'vpp', 'vrrp'], outside)


class TestTheSkipList(unittest.TestCase):
    """ZE_SKIP_SUITES leaves a suite out of the denominator as well as the run."""

    def test_a_skipped_suite_is_named_and_not_counted(self) -> None:
        with environment(ZE_SKIP_SUITES='firewall,web'):
            self.assertEqual({'firewall', 'web'}, functional.skipped())

    def test_an_empty_skip_list_skips_nothing(self) -> None:
        with environment():
            self.assertEqual(set(), functional.skipped())
        with environment(ZE_SKIP_SUITES=''):
            self.assertEqual(set(), functional.skipped())

    def test_a_skipped_suite_is_reported_at_the_end(self) -> None:
        run = functional.Run(suite_total=1)
        suite = functional.suite_named('web')
        assert suite is not None
        out = io.StringIO()
        with environment(), redirect_stdout(out):
            run.skip(suite)
            code = run.summarise()
        self.assertEqual(0, code)
        self.assertIn('SKIPPED suites (ZE_SKIP_SUITES): web', out.getvalue())


class TestTheIsolatedBinarySet(unittest.TestCase):
    """Where a run's binaries come from, and whether they are thrown away.

    The makefile had this and nothing tested it. The failure it guards against
    is two runs sharing one throwaway root: they race on the build and, worse,
    share the root's etc/ze, so one run's test database writes corrupt the
    other's.
    """

    def test_the_auto_directory_is_scoped_per_process_and_per_target(self) -> None:
        with environment(ZE_SCRATCH_DIR='tmp'):
            first, remove = functional.binary_root('ze-functional-encode-test')
            second, _ = functional.binary_root('ze-functional-plugin-test')
        self.assertTrue(remove, 'a throwaway set must be removed')
        self.assertNotEqual(first, second)
        self.assertIn(str(os.getpid()), first.name)
        self.assertTrue(first.name.endswith('ze-functional-encode-test'))

    def test_an_explicit_suffix_is_stable_and_kept(self) -> None:
        with environment(ZE_SCRATCH_DIR='tmp', ZE_SUFFIX='mine'):
            first, remove = functional.binary_root('ze-functional-encode-test')
            second, _ = functional.binary_root('ze-functional-plugin-test')
        self.assertFalse(remove, 'a named set is kept for inspection')
        self.assertEqual(first, second)
        self.assertTrue(first.name.endswith('testbin-mine'))

    def test_the_set_lands_under_the_session_directory(self) -> None:
        # Not at the tmp/ root: several sessions share one checkout, and a set
        # that outlives a crash must be swept with the session that made it.
        with environment(ZE_SCRATCH_DIR='tmp/session/2026-01-01-abcd'):
            root, _ = functional.binary_root('ze-functional-encode-test')
        self.assertEqual(REPO / 'tmp' / 'session' / '2026-01-01-abcd', root.parent)

    def test_the_runner_is_frozen_against_an_isolated_set(self) -> None:
        binaries = functional.BinarySet(directory=Path('/x/bin'), remove=True)
        with environment():
            env = binaries.environment()
        self.assertEqual('1', env['ZE_TEST_NO_BUILD'])
        self.assertEqual('/x/bin/ze', env['ZE_BIN'])
        self.assertEqual('/x/bin/ze-test', env['ZE_TEST_BIN'])

    def test_a_canonical_run_lets_the_runner_rebuild(self) -> None:
        # ZE_TEST_CANONICAL is the release and CI reproducibility path: it IS
        # the session's own binary set, and freezing it would defeat the point.
        binaries = functional.BinarySet(directory=Path('/x/bin'), remove=False, canonical=True)
        with environment():
            env = binaries.environment()
        self.assertNotIn('ZE_TEST_NO_BUILD', env)
        self.assertNotIn('ZE_BIN', env)

    def test_the_binaries_live_in_a_bin_subdirectory(self) -> None:
        # ze derives its config and database directory from its own location and
        # only recognises a parent named bin or sbin (paths.go isBinDir). A
        # binary directly in the throwaway root answers "cannot determine
        # database location" and breaks `ze config archive`.
        with environment(ZE_SCRATCH_DIR='tmp'):
            root, _ = functional.binary_root('ze-functional-encode-test')
        commands = functional._build_commands(root / 'bin', chaos=False)
        for argv in commands:
            with self.subTest(command=argv[-1]):
                self.assertEqual('bin', Path(argv[argv.index('-o') + 1]).parent.name)

    def test_the_chaos_dashboard_is_built_only_where_it_is_used(self) -> None:
        # Only the web suite starts it (option=server:kind=chaos), and it must
        # sit BESIDE the ze binary the run uses, which is where cmd_web.go
        # looks for it.
        with environment():
            plain = functional._build_commands(Path('/x/bin'), chaos=False)
            with_chaos = functional._build_commands(Path('/x/bin'), chaos=True)
        self.assertEqual(len(plain) + 1, len(with_chaos))
        self.assertTrue(with_chaos[-1][with_chaos[-1].index('-o') + 1].endswith('/bin/ze-chaos'))
        web = functional.suite_named('web')
        assert web is not None
        self.assertTrue(web.chaos)
        self.assertEqual(['web'], [suite.name for suite in functional.SUITES if suite.chaos])

    def test_the_test_harness_is_never_instrumented(self) -> None:
        # ze-test is the harness, not the subject: what IT executed is not what
        # the coverage map is about.
        with environment(ZE_COVER='1'):
            commands = functional._build_commands(Path('/x/bin'), chaos=False)
        instrumented = {
            Path(argv[argv.index('-o') + 1]).name for argv in commands if '-cover' in argv
        }
        self.assertEqual({'ze', 'ze-stripped'}, instrumented)


if __name__ == '__main__':
    unittest.main()
