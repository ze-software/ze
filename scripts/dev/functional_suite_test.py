#!/usr/bin/env python3
"""Unit tests for the per-suite wall-clock budget in mk/test-functional.mk.

Every functional suite runs under `timeout --kill-after=<K> <T>`, and `timeout`
returns 124 when it kills the suite. Before this file existed, `run_suite`
counted 124 as a plain suite failure, so a suite that ran out of budget looked
exactly like a suite whose tests were broken: the run printed the N test
failures the runner had managed to emit before the process group died, and
nothing said the kill had happened. That is what the plugin suite produced on
2026-08-18, when it measured 599.7s against a 600s cap and five of its tests
reported `start ze-peer: context canceled`.

Two behaviors are pinned here:

- a cap expiry is REPORTED as one (`BUDGET EXPIRED`, naming the suite and its
  budget, plus a `VERIFY FAILURE GROUP:` line so tmp/ze-verify-failures.json
  carries it as kind `timeout` rather than as a nameless suite failure), while
  still counting as a suite failure;
- every suite's runtime is recorded against its budget, and a suite that
  crosses ZE_SUITE_WARN_PERCENT is warned about while it is still green.
  Raising a cap is not a fix on its own (ai/rules/completion.md), so the number
  has to stay visible as it creeps back;
- a cap expiry tells the reader what to re-run, with the same command an
  ordinary suite failure carries (functionalSuiteRerun, scripts/status/verify_run.go),
  and every suite the gating run can fail on has that target. A failure group
  whose `rerun` is empty, or names a target make answers with `No rule to make
  target`, is the one group a reader cannot act on;
- a suite named in the ZE_SUITE_TIMEOUT_<SUITE> family gets that budget in
  place of the shared one, everywhere: the `timeout` that kills it, the runtime
  line, the warning arithmetic, and the variable the reports tell you to raise.
  The shared cap is what protects the other 23 suites, so one slow suite must
  not be able to raise it for everybody.

The tests run the REAL shell from the makefile. `run_suite` is extracted from
the recipe, the handful of make variables it reads are substituted, and the
function is driven with a stub command whose exit code and duration each test
chooses. Extraction is fail-closed: a make variable this file does not know
about, or a `run_suite` whose recipe lines moved, fails the test rather than
silently testing a stale copy.
"""

from __future__ import annotations

import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
MAKEFILE = REPO / "mk" / "test-functional.mk"

# The gating suite list, and the rule heads of every make target the repository
# declares. A suite a run can fail on must be a suite a developer can re-run, so
# the reports name `make ze-functional-<suite>-test` for every member and this
# file holds that name true against the build system.
ALL_SUITES_RE = re.compile(r'(?m)^\s*all_suites="([^"]+)"')
MAKE_TARGET_RE = re.compile(r"(?m)^([A-Za-z0-9_.-]+)[ \t]*:(?:[^=]|$)")
SUITE_HINT_RE = re.compile(r'printf "  (make \S+)\\n" "\$\$suite"')


def all_suites() -> list[str]:
    """Return the suites `make ze-functional-test` runs, from their one source."""
    matches = ALL_SUITES_RE.findall(MAKEFILE.read_text())
    if len(matches) != 1:
        raise AssertionError(
            f"{MAKEFILE}: expected one all_suites= line, found {len(matches)}"
        )
    suites = matches[0].split()
    if len(suites) < 20:
        raise AssertionError(
            f"parsed only {len(suites)} gating suites; the all_suites regex has rotted"
        )
    return suites


def declared_make_targets() -> set[str]:
    """Return every make target the Makefile and mk/*.mk fragments declare."""
    corpus = [(REPO / "Makefile").read_text()]
    fragments = sorted((REPO / "mk").glob("*.mk"))
    if not fragments:
        raise AssertionError("no mk/*.mk fragments found: layout changed?")
    corpus.extend(path.read_text() for path in fragments)
    targets = set(MAKE_TARGET_RE.findall("\n".join(corpus)))
    if len(targets) < 50:
        raise AssertionError(
            f"parsed only {len(targets)} make targets; the rule-head regex has rotted"
        )
    return targets


def suite_rerun(suite: str) -> str:
    """Return the command that re-runs one suite (functionalSuiteRerun's twin)."""
    return f"make ze-functional-{suite}-test"


# The recipe lines that open and close the run_suite definition. They are matched
# whole, tab indentation and make line continuation included, so a reformatting
# of the recipe fails loudly here instead of extracting half a function.
FN_OPEN = "\trun_suite() { \\"
FN_CLOSE = "\t}; \\"

# Everything run_suite reads from make. Any other $(...) in the extracted body
# means the makefile grew a knob this file does not drive; extraction refuses.
MAKE_VARS = (
    "GO",
    "ZE_SKIP_SUITES",
    "ZE_SUITE_TIMEOUT",
    "ZE_SUITE_TIMEOUT_PLUGIN",
    "ZE_SUITE_WARN_PERCENT",
)

# A suite with no budget of its own, used wherever a test drives the shared cap.
# Naming the overridden suite there would make the assertion read the override
# and pass for the wrong reason.
SHARED_BUDGET_SUITE = "encode"

# The plugin suite's budget is derived from a measurement, not chosen round.
# Spec verify-scope-4, A-1: `make ze-functional-plugin-test ZE_SUITE_TIMEOUT=1800s`
# ran the suite to completion in 855s on 2026-08-19, on a box carrying five
# other sessions at a load average rising 6.6 -> 18.7 across 32 cores.
PLUGIN_MEASURED_SECONDS = 855

# The warning point (ZE_SUITE_WARN_PERCENT of the budget) must sit this far
# above that measurement. Below it a busy box warns on every run, and a warning
# that fires every run names no creep.
PLUGIN_WARN_HEADROOM = 1.40

SCALE = {"": 1, "s": 1, "m": 60, "h": 3600, "d": 86400}


def duration_seconds(text: str) -> int:
    """Return a `timeout` duration in seconds, the way run_suite reads one."""
    suffix = text[-1] if text and text[-1].isalpha() else ""
    number = text[: -len(suffix)] if suffix else text
    if not number.isdigit() or suffix not in SCALE:
        raise AssertionError(f"{text!r} is not a duration run_suite can measure")
    return int(number) * SCALE[suffix]


# The variables the recipe sets before it defines run_suite: the accumulators, and
# the coverage root, empty here because coverage is off. The tests
# set them up the same way, so a name added to one side and not the other shows
# up as an unbound-variable error under `set -u`.
PRELUDE = """set -u
cover_root=""
suite_total=1
suite_index=0
total=0
failed=0
failed_names=""
skipped_names=""
expired_names=""
warned_names=""
runtimes=""
"""

EPILOGUE = (
    "printf 'STATE failed=%s failed_names=[%s] expired_names=[%s] "
    "warned_names=[%s] skipped_names=[%s]\\n' "
    '"$failed" "$failed_names" "$expired_names" "$warned_names" "$skipped_names"\n'
    "printf 'RUNTIMES %b' \"$runtimes\"\n"
)


def extract_run_suite(
    timeout: str,
    warn_percent: str,
    skip_suites: str = "",
    plugin_timeout: str = "1500s",
) -> str:
    """Return run_suite as runnable POSIX shell, with make's variables resolved."""
    lines = MAKEFILE.read_text().splitlines()
    if FN_OPEN not in lines:
        raise AssertionError(
            f"{MAKEFILE}: no line {FN_OPEN!r}; run_suite moved or was reformatted"
        )
    start = lines.index(FN_OPEN)
    end = None
    for index in range(start + 1, len(lines)):
        if lines[index] == FN_CLOSE:
            end = index
            break
    if end is None:
        raise AssertionError(
            f"{MAKEFILE}: no line {FN_CLOSE!r} after run_suite; the function has no end"
        )

    body = "\n".join(line[1:].removesuffix(" \\") for line in lines[start : end + 1])
    values = {
        "GO": "go",
        "ZE_SKIP_SUITES": skip_suites,
        "ZE_SUITE_TIMEOUT": timeout,
        "ZE_SUITE_TIMEOUT_PLUGIN": plugin_timeout,
        "ZE_SUITE_WARN_PERCENT": warn_percent,
    }
    for name in MAKE_VARS:
        body = body.replace(f"$({name})", values[name])
    body = body.replace("$$", "$")
    leftover = re.findall(r"\$\([A-Za-z_][A-Za-z0-9_]*\)", body)
    if leftover:
        raise AssertionError(
            f"run_suite reads make variable(s) this test does not resolve: {sorted(set(leftover))}. "
            "Add them to MAKE_VARS and give each one a value."
        )
    return body


class SuiteRun:
    """One driven call of run_suite, with everything it printed and left behind."""

    def __init__(self, stdout: str, stderr: str) -> None:
        self.stdout = stdout
        self.stderr = stderr

    def state(self, name: str) -> str:
        match = re.search(rf"\b{name}=\[([^\]]*)\]", self.stdout)
        if match is None:
            raise AssertionError(f"no {name}=[...] in output:\n{self.stdout}")
        return match.group(1)

    @property
    def failed(self) -> int:
        match = re.search(r"\bfailed=(\d+)", self.stdout)
        if match is None:
            raise AssertionError(f"no failed=N in output:\n{self.stdout}")
        return int(match.group(1))

    @property
    def runtimes(self) -> str:
        _, _, tail = self.stdout.partition("RUNTIMES ")
        return tail

    def failure_group(self) -> dict:
        prefix = "VERIFY FAILURE GROUP: "
        payloads = [
            line.partition(prefix)[2]
            for line in self.stdout.splitlines()
            if prefix in line
        ]
        if len(payloads) != 1:
            raise AssertionError(
                f"expected exactly one {prefix!r} line, got {len(payloads)}:\n{self.stdout}"
            )
        return json.loads(payloads[0])


def expanded_run_suite() -> str:
    """Return run_suite as MAKE expands it, with make's own default values in it.

    extract_run_suite substitutes the make variables itself, so it cannot see a
    `$$` written as `$`, a `%` make ate, or a default the recipe never reads.
    `make --dry-run` takes no admission slot (scripts/dev/ze-run.sh refuses to
    queue make's no-execute modes), so this costs a parse.
    """
    proc = subprocess.run(
        ["make", "--dry-run", "_ze-functional-test-impl"],
        cwd=REPO,
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        raise AssertionError(
            f"make --dry-run exited {proc.returncode}\n{proc.stdout}\n{proc.stderr}"
        )
    lines = proc.stdout.splitlines()
    opens = [i for i, line in enumerate(lines) if line == "run_suite() { \\"]
    if len(opens) != 1:
        raise AssertionError(
            f"expected one expanded run_suite definition, found {len(opens)}"
        )
    start = opens[0]
    end = next((i for i in range(start + 1, len(lines)) if lines[i] == "}; \\"), None)
    if end is None:
        raise AssertionError("the expanded run_suite has no closing line")
    return "\n".join(line.removesuffix(" \\") for line in lines[start : end + 1])


def drive(
    exit_code: int,
    *,
    suite: str = "demo",
    timeout: str = "600s",
    plugin_timeout: str = "1500s",
    warn_percent: str = "80",
    sleep_seconds: int = 0,
    body: str | None = None,
) -> SuiteRun:
    """Run run_suite over a stub command that sleeps, then exits exit_code."""
    script = (
        PRELUDE
        + (
            extract_run_suite(timeout, warn_percent, plugin_timeout=plugin_timeout)
            if body is None
            else body
        )
        + f"\nrun_suite {suite} sh -c 'sleep {sleep_seconds}; exit {exit_code}'\n"
        + EPILOGUE
    )
    with tempfile.TemporaryDirectory() as work:
        path = Path(work) / "drive.sh"
        path.write_text(script)
        proc = subprocess.run(
            ["sh", str(path)], capture_output=True, text=True, check=False
        )
    if proc.returncode != 0:
        raise AssertionError(
            f"driver exited {proc.returncode}\nstdout:\n{proc.stdout}\nstderr:\n{proc.stderr}"
        )
    return SuiteRun(proc.stdout, proc.stderr)


class TestRunSuiteReportsCapExpiryDistinctly(unittest.TestCase):
    """AC-2: a suite killed by its cap says so, and still fails.

    Driven over a suite on the SHARED budget. The suite this was written for is
    `plugin`, which now has a budget of its own, and driving it here would read
    that budget instead of the one each test sets. TestPerSuiteBudget covers it.
    """

    def test_exit_124_names_the_suite_and_its_budget(self) -> None:
        run = drive(124, suite=SHARED_BUDGET_SUITE, timeout="600s")
        self.assertIn("BUDGET EXPIRED", run.stdout)
        expired = [
            line for line in run.stdout.splitlines() if "BUDGET EXPIRED" in line
        ][0]
        self.assertIn(SHARED_BUDGET_SUITE, expired)
        self.assertIn("600s", expired)
        self.assertIn("ZE_SUITE_TIMEOUT", expired)

    def test_exit_124_stays_a_suite_failure(self) -> None:
        run = drive(124, suite=SHARED_BUDGET_SUITE)
        self.assertEqual(1, run.failed)
        self.assertEqual(SHARED_BUDGET_SUITE, run.state("failed_names"))
        self.assertEqual(SHARED_BUDGET_SUITE, run.state("expired_names"))

    def test_exit_124_publishes_a_timeout_failure_group(self) -> None:
        run = drive(124, suite=SHARED_BUDGET_SUITE, timeout="600s")
        group = run.failure_group()
        self.assertEqual(SHARED_BUDGET_SUITE, group["stage"])
        self.assertEqual("timeout", group["kind"])
        self.assertEqual(f"suite-budget:{SHARED_BUDGET_SUITE}", group["group-id"])
        self.assertEqual([SHARED_BUDGET_SUITE], group["related"])
        self.assertIn("600s", group["summary"])

    def test_an_ordinary_failure_is_not_reported_as_a_cap_expiry(self) -> None:
        run = drive(1, suite=SHARED_BUDGET_SUITE)
        self.assertNotIn("BUDGET EXPIRED", run.stdout)
        self.assertNotIn("VERIFY FAILURE GROUP:", run.stdout)
        self.assertEqual(1, run.failed)
        self.assertEqual(SHARED_BUDGET_SUITE, run.state("failed_names"))
        self.assertEqual("", run.state("expired_names"))

    def test_a_passing_suite_reports_neither(self) -> None:
        run = drive(0, suite=SHARED_BUDGET_SUITE)
        self.assertNotIn("BUDGET EXPIRED", run.stdout)
        self.assertNotIn("BUDGET WARNING", run.stdout)
        self.assertEqual(0, run.failed)
        self.assertEqual("", run.state("failed_names"))
        self.assertEqual("", run.state("expired_names"))

    def test_a_cap_expiry_is_not_also_a_creep_warning(self) -> None:
        # A killed suite is at 100% of its budget by construction. Saying so a
        # second time as a warning would bury the line that names the kill.
        run = drive(124, suite=SHARED_BUDGET_SUITE, timeout="1s", sleep_seconds=1)
        self.assertIn("BUDGET EXPIRED", run.stdout)
        self.assertNotIn("BUDGET WARNING", run.stdout)
        self.assertEqual("", run.state("warned_names"))


class TestSuiteRuntimeRecorded(unittest.TestCase):
    """AC-3: the runtime is recorded per suite, and creep warns before it kills."""

    def test_runtime_is_reported_against_the_budget(self) -> None:
        run = drive(0, suite="encode", timeout="600s")
        self.assertRegex(
            run.stdout, r"suite encode took \d+s of its 600s budget \(\d+%\)"
        )

    def test_runtime_is_accumulated_for_the_end_of_run_summary(self) -> None:
        run = drive(0, suite="encode", timeout="600s")
        self.assertRegex(run.runtimes, r"encode \d+s of 600s \(\d+%\)")

    def test_a_suite_below_the_warning_level_does_not_warn(self) -> None:
        run = drive(0, suite="encode", timeout="600s", warn_percent="80")
        self.assertNotIn("BUDGET WARNING", run.stdout)
        self.assertEqual("", run.state("warned_names"))

    def test_a_suite_at_the_warning_level_warns_while_still_green(self) -> None:
        run = drive(0, suite="encode", timeout="1s", warn_percent="80", sleep_seconds=1)
        self.assertIn("BUDGET WARNING", run.stdout)
        self.assertEqual("encode", run.state("warned_names"))
        self.assertEqual(0, run.failed, "a warning must not turn a green suite red")

    def test_the_budget_unit_is_read_rather_than_assumed(self) -> None:
        # `timeout` takes s, m, h, d and a bare number of seconds. A minute
        # budget read as 2 seconds would warn on every suite.
        run = drive(0, suite="encode", timeout="2m")
        self.assertRegex(run.stdout, r"suite encode took \d+s of its 2m budget \(0%\)")
        self.assertNotIn("BUDGET WARNING", run.stdout)

    def test_a_bare_number_is_seconds(self) -> None:
        run = drive(0, suite="encode", timeout="600")
        self.assertRegex(run.stdout, r"suite encode took \d+s of its 600 budget \(0%\)")

    def test_an_unusable_budget_says_so_instead_of_dividing_by_zero(self) -> None:
        # 0s is the bottom of the range: it kills every suite immediately, and it
        # is not a denominator. The runtime is still recorded.
        for budget in ("0s", "abc", "1.5m"):
            with self.subTest(budget=budget):
                run = drive(0, suite="encode", timeout=budget)
                self.assertRegex(
                    run.stdout,
                    rf"suite encode took \d+s \(budget {re.escape(budget)} is not a duration",
                )
                self.assertNotIn("BUDGET WARNING", run.stdout)
                self.assertEqual(0, run.failed)


class TestMakeExpandsTheBudgetReport(unittest.TestCase):
    """The recipe make actually runs, with the cap the repository ships."""

    def test_the_expanded_recipe_reports_a_cap_expiry_at_the_shipped_budget(
        self,
    ) -> None:
        body = expanded_run_suite()
        shipped = re.search(
            r"(?m)^ZE_SUITE_TIMEOUT \?= (\S+)$", MAKEFILE.read_text()
        ).group(1)
        run = drive(124, suite=SHARED_BUDGET_SUITE, body=body)
        expired = [line for line in run.stdout.splitlines() if "BUDGET EXPIRED" in line]
        self.assertEqual(1, len(expired), run.stdout)
        self.assertIn(shipped, expired[0])
        self.assertIn(shipped, run.failure_group()["summary"])

    def test_the_expanded_recipe_records_the_runtime(self) -> None:
        run = drive(0, suite="encode", body=expanded_run_suite())
        self.assertRegex(
            run.stdout, r"suite encode took \d+s of its \S+ budget \(\d+%\)"
        )

    def test_the_expanded_recipe_gives_plugin_the_budget_the_repository_ships(
        self,
    ) -> None:
        shipped = re.search(
            r"(?m)^ZE_SUITE_TIMEOUT_PLUGIN \?= (\S+)$", MAKEFILE.read_text()
        ).group(1)
        shared = re.search(
            r"(?m)^ZE_SUITE_TIMEOUT \?= (\S+)$", MAKEFILE.read_text()
        ).group(1)
        self.assertNotEqual(shared, shipped, "the override must differ from the cap")
        run = drive(0, suite="plugin", body=expanded_run_suite())
        self.assertRegex(run.stdout, rf"suite plugin took \d+s of its {shipped} budget")


class TestPerSuiteBudget(unittest.TestCase):
    """AC-1: a suite named in ZE_SUITE_TIMEOUT_<SUITE> runs on its own budget.

    The plugin suite holds 663 `.ci` files and needs more wall clock than the
    600s the other 23 suites share. Raising the shared cap to fit it takes that
    margin away from all of them, which is what the cap exists to give them, so
    the budget is per suite and the shared default stays 600s.
    """

    def test_the_overridden_suite_uses_its_own_budget(self) -> None:
        run = drive(0, suite="plugin", timeout="600s", plugin_timeout="1500s")
        self.assertRegex(run.stdout, r"suite plugin took \d+s of its 1500s budget")
        self.assertNotIn("600s", run.stdout)

    def test_a_suite_without_an_override_keeps_the_shared_budget(self) -> None:
        run = drive(
            0, suite=SHARED_BUDGET_SUITE, timeout="600s", plugin_timeout="1500s"
        )
        self.assertRegex(
            run.stdout, rf"suite {SHARED_BUDGET_SUITE} took \d+s of its 600s budget"
        )
        self.assertNotIn("1500s", run.stdout)

    def test_the_runtime_summary_row_carries_the_suites_own_budget(self) -> None:
        run = drive(0, suite="plugin", timeout="600s", plugin_timeout="1500s")
        self.assertRegex(run.runtimes, r"plugin \d+s of 1500s \(\d+%\)")

    def test_the_warning_is_measured_against_the_suites_own_budget(self) -> None:
        # The discriminating case: against the shared 1s budget this suite is at
        # 100% and warns; against its own 600s it is at 0%. A warning computed
        # from the shared default would make the overridden suite warn on every
        # run, and a warning that always fires names no creep.
        run = drive(
            0, suite="plugin", timeout="1s", plugin_timeout="600s", sleep_seconds=1
        )
        self.assertNotIn("BUDGET WARNING", run.stdout)
        self.assertEqual("", run.state("warned_names"))

    def test_the_warning_still_fires_when_the_suites_own_budget_is_the_tight_one(
        self,
    ) -> None:
        run = drive(
            0, suite="plugin", timeout="600s", plugin_timeout="1s", sleep_seconds=1
        )
        warning = [line for line in run.stdout.splitlines() if "BUDGET WARNING" in line]
        self.assertEqual(1, len(warning), run.stdout)
        self.assertIn("of its 1s budget", warning[0])
        self.assertIn("raise ZE_SUITE_TIMEOUT_PLUGIN", warning[0])
        self.assertEqual("plugin", run.state("warned_names"))
        self.assertEqual(0, run.failed, "a warning must not turn a green suite red")

    def test_a_cap_expiry_names_the_variable_that_owns_the_budget(self) -> None:
        # Telling the reader to raise ZE_SUITE_TIMEOUT when the kill came from
        # ZE_SUITE_TIMEOUT_PLUGIN sends them to raise the cap for all 24 suites.
        for suite, budget, variable in (
            ("plugin", "1500s", "ZE_SUITE_TIMEOUT_PLUGIN"),
            (SHARED_BUDGET_SUITE, "600s", "ZE_SUITE_TIMEOUT"),
        ):
            with self.subTest(suite=suite):
                run = drive(124, suite=suite, timeout="600s", plugin_timeout="1500s")
                expired = [
                    line for line in run.stdout.splitlines() if "BUDGET EXPIRED" in line
                ][0]
                self.assertIn(f"its {budget} wall-clock budget ({variable})", expired)
                self.assertIn(
                    f"its {budget} wall-clock budget ({variable})",
                    run.failure_group()["summary"],
                )


class TestSuiteBudgetContract(unittest.TestCase):
    """The cap's own shape: finite, overridable, and applied through `timeout`."""

    def setUp(self) -> None:
        self.text = MAKEFILE.read_text()

    def test_the_cap_is_overridable_from_the_command_line(self) -> None:
        for name in (
            "ZE_SUITE_TIMEOUT",
            "ZE_SUITE_TIMEOUT_PLUGIN",
            "ZE_SUITE_KILL_AFTER",
            "ZE_SUITE_WARN_PERCENT",
        ):
            with self.subTest(variable=name):
                self.assertRegex(
                    self.text,
                    rf"(?m)^{name} \?= \S+$",
                    f"{name} must use ?= so a caller can override it",
                )

    def test_the_cap_stays_finite_and_kills_the_process_group(self) -> None:
        # Removing the cap reopens the hang it was added for: a stuck subprocess
        # holding an output pipe made cmd.Wait() block indefinitely, and only
        # `timeout` signals the whole process group.
        self.assertRegex(
            self.text,
            r"(?m)^SUITE_RUN = timeout --kill-after=\$\(ZE_SUITE_KILL_AFTER\) \$\(ZE_SUITE_TIMEOUT\)$",
        )
        match = re.search(r"(?m)^ZE_SUITE_TIMEOUT \?= (\S+)$", self.text)
        self.assertIsNotNone(match)
        self.assertRegex(
            match.group(1), r"^\d+[smhd]?$", "the cap must be a finite duration"
        )

    def test_every_per_suite_budget_is_wired_on_every_path(self) -> None:
        # A budget the report reads and `timeout` does not is worse than no
        # override: the run says 1500s while the kill lands at 600s. Each
        # ZE_SUITE_TIMEOUT_<SUITE> owes a SUITE_RUN_<SUITE>, an arm in
        # run_suite's case, and that SUITE_RUN_<SUITE> on both the aggregate
        # line and the suite's own -impl target.
        overrides = re.findall(
            r"(?m)^ZE_SUITE_TIMEOUT_([A-Z0-9_]+) \?= (\S+)$", self.text
        )
        self.assertTrue(overrides, "no per-suite budget is defined")
        for upper, value in overrides:
            suite = upper.lower().replace("_", "-")
            with self.subTest(suite=suite):
                self.assertRegex(
                    value, r"^\d+[smhd]?$", "a per-suite budget must be finite"
                )
                self.assertRegex(
                    self.text,
                    rf"(?m)^SUITE_RUN_{upper} = timeout "
                    rf"--kill-after=\$\(ZE_SUITE_KILL_AFTER\) "
                    rf"\$\(ZE_SUITE_TIMEOUT_{upper}\)$",
                    f"SUITE_RUN_{upper} must apply the budget through `timeout`",
                )
                self.assertRegex(
                    self.text,
                    rf'(?m)^\t+{re.escape(suite)}\) budget="\$\(ZE_SUITE_TIMEOUT_{upper}\)"; '
                    rf"budget_var=ZE_SUITE_TIMEOUT_{upper} ;; ",
                    "run_suite's budget case must name the suite",
                )
                self.assertRegex(
                    self.text,
                    rf"(?m)^\trun_suite {re.escape(suite)} \$\(SUITE_RUN_{upper}\) ",
                    f"the aggregate run_suite line must use SUITE_RUN_{upper}",
                )
                impl = re.search(
                    rf"(?m)^_ze-functional-{re.escape(suite)}-test-impl:.*\n(\t.*\n)+",
                    self.text,
                )
                self.assertIsNotNone(
                    impl, f"no _ze-functional-{suite}-test-impl target"
                )
                self.assertIn(
                    f"$(SUITE_RUN_{upper})",
                    impl.group(0),
                    f"make ze-functional-{suite}-test must use the same budget",
                )

    def test_the_plugin_budget_keeps_its_warning_above_the_measurement(self) -> None:
        # The number is derived, not picked: 855s measured, and the warning
        # point must sit 40% above it, so 855 * 1.40 / 0.80 = 1496s rounded up
        # to the whole minute. Lowering the budget without making the suite
        # faster puts the warning back on top of a normal run.
        budget = duration_seconds(
            re.search(r"(?m)^ZE_SUITE_TIMEOUT_PLUGIN \?= (\S+)$", self.text).group(1)
        )
        warn_percent = int(
            re.search(r"(?m)^ZE_SUITE_WARN_PERCENT \?= (\d+)$", self.text).group(1)
        )
        warn_point = budget * warn_percent / 100
        self.assertGreaterEqual(
            warn_point,
            PLUGIN_MEASURED_SECONDS * PLUGIN_WARN_HEADROOM,
            f"the plugin suite measured {PLUGIN_MEASURED_SECONDS}s, so a "
            f"{warn_percent}% warning at {warn_point:.0f}s leaves less than "
            f"{PLUGIN_WARN_HEADROOM:.0%} of headroom and will fire on a busy box",
        )
        shared = duration_seconds(
            re.search(r"(?m)^ZE_SUITE_TIMEOUT \?= (\S+)$", self.text).group(1)
        )
        self.assertGreater(
            budget, shared, "an override below the shared cap is not an override"
        )


# The Go constant the makefile floor must agree with, and the file that holds it.
# Two numbers that must be equal and live in different languages drift silently;
# this pair is held equal by TestSmallHostKeepsTheFloor below.
PARALLEL_GO = REPO / "internal" / "test" / "runner" / "parallel.go"
GO_FLOOR_RE = re.compile(r"(?m)^const SuiteConcurrencyFloor = (\d+)$")
VPP_GO = REPO / "internal" / "test" / "cli" / "cmd_vpp.go"


def suite_parallel(**overrides: str) -> dict[str, str]:
    """Return the concurrency make derives, for a host of the caller's choosing.

    The real makefile fragment is included by a throwaway file that adds one
    target, so what is measured is the derivation the suites actually run under
    and not a copy of it. ZE_SUITE_CORES stands in for the host, which is how a
    32-core box can assert what a 4-vCPU CI runner gets.
    """
    with tempfile.TemporaryDirectory() as tmp:
        probe = Path(tmp) / "probe.mk"
        probe.write_text(
            "show:\n"
            '\t@echo "plugin=$(ZE_PLUGIN_PARALLEL) encode=$(ZE_ENCODE_PARALLEL) '
            'floor=$(ZE_SUITE_PARALLEL_FLOOR)"\n'
            "include mk/test-functional.mk\n"
        )
        result = subprocess.run(
            ["make", "-f", str(probe), "show"]
            + [f"{name}={value}" for name, value in overrides.items()],
            cwd=REPO,
            capture_output=True,
            text=True,
            check=True,
        )
    fields = dict(item.split("=", 1) for item in result.stdout.split())
    for name in ("plugin", "encode", "floor"):
        if name not in fields:
            raise AssertionError(f"make printed no {name}=: {result.stdout!r}")
    return fields


class TestSuiteConcurrencyDerivation(unittest.TestCase):
    """The two measured suites size themselves to the host, within bounds.

    ZE_PLUGIN_PARALLEL and ZE_ENCODE_PARALLEL were the constant 8, which is what
    GitHub's 4-vCPU hosted runner survives. Every host got that number, so a
    32-core box ran the 665-test plugin suite at a quarter of the width it can
    carry. The floor keeps the small host exactly where it was; the cap is the
    core count, past which the measured parallel efficiency falls to 36%.
    """

    def test_plugin_concurrency_is_derived_not_pinned(self) -> None:
        text = MAKEFILE.read_text()
        for name in ("ZE_PLUGIN_PARALLEL", "ZE_ENCODE_PARALLEL"):
            with self.subTest(variable=name):
                self.assertRegex(
                    text,
                    rf"(?m)^{name} \?= \$\(ZE_SUITE_PARALLEL\)$",
                    f"{name} must read the derived value, not a constant",
                )
        for cores in ("16", "32", "64"):
            with self.subTest(cores=cores):
                got = suite_parallel(ZE_SUITE_CORES=cores)
                self.assertEqual(cores, got["plugin"])
                self.assertEqual(
                    cores,
                    got["encode"],
                    "encode was measured separately and moves with plugin only "
                    "because both were pinned at the same constant",
                )

    def test_small_host_keeps_the_floor(self) -> None:
        go_floor = GO_FLOOR_RE.search(PARALLEL_GO.read_text())
        self.assertIsNotNone(
            go_floor, f"{PARALLEL_GO}: SuiteConcurrencyFloor moved or was renamed"
        )
        floor = suite_parallel()["floor"]
        self.assertEqual(
            go_floor.group(1),
            floor,
            "the makefile floor and runner.SuiteConcurrencyFloor are the same "
            "measured figure; one may not move without the other",
        )
        # A 4-vCPU CI runner, and the degenerate inputs a container can produce.
        for cores in ("1", "2", "4", "8", "", "unknown"):
            with self.subTest(cores=cores):
                got = suite_parallel(ZE_SUITE_CORES=cores)
                self.assertEqual(
                    floor,
                    got["plugin"],
                    "a host at or below the floor, or one that cannot say how "
                    "many cores it has, must get exactly what CI runs today",
                )
                self.assertEqual(floor, got["encode"])

    def test_explicit_parallel_wins(self) -> None:
        got = suite_parallel(ZE_SUITE_CORES="32", ZE_PLUGIN_PARALLEL="3")
        self.assertEqual(
            "3", got["plugin"], "an operator's own value must beat the derivation"
        )
        self.assertEqual(
            "32",
            got["encode"],
            "overriding one suite must not move the other",
        )

    def test_serial_suites_stay_serial(self) -> None:
        # register.go records that reload and managed share the kernel routing
        # table, so they run one test at a time. Both spellings of each suite
        # (the aggregate run_suite line and the individual target) must say so:
        # a -p that survives in only one of them is a suite that is serial for
        # `make ze-functional-test` and parallel for the developer re-running it.
        text = MAKEFILE.read_text()
        for suite in ("reload", "managed"):
            with self.subTest(suite=suite):
                lines = [
                    line
                    for line in text.splitlines()
                    if re.search(rf"\b{suite} --all\b", line)
                ]
                self.assertEqual(
                    2, len(lines), f"expected two {suite} invocations, got {lines}"
                )
                for line in lines:
                    # The aggregate line carries make's `; \\` continuation, the
                    # individual target does not; strip both to compare the
                    # command itself.
                    command = line.rstrip().rstrip("\\").rstrip().rstrip(";")
                    self.assertTrue(
                        command.endswith("-p 1"),
                        f"{suite} must run serially: {line.strip()!r}",
                    )
        # vpp carries no -p in the makefile: its serial default lives in the
        # command itself, and this spec did not measure it either.
        self.assertNotRegex(text, r"(?m)vpp --all.*-p ")
        self.assertRegex(
            VPP_GO.read_text(),
            r'fs\.IntVar\(&cli\.parallel, "p", 1,',
            f"{VPP_GO}: the vpp suite's default concurrency must stay 1",
        )


class TestEverySuiteCanBeRerun(unittest.TestCase):
    """A suite a run can fail on is a suite a developer can re-run.

    Nothing executes the command a failure report prints, so a suite added to
    all_suites without its own target leaves the report naming a target make
    answers with `No rule to make target`. That is how `make ze-<suite>-test`
    survived for all 24 suites (the journal row on a failure report that names a
    command which does not exist).
    """

    def test_every_gating_suite_has_an_individual_target(self) -> None:
        targets = declared_make_targets()
        for suite in all_suites():
            with self.subTest(suite=suite):
                self.assertIn(
                    f"ze-functional-{suite}-test",
                    targets,
                    f"suite {suite} is in all_suites, so a failed run names "
                    f"`{suite_rerun(suite)}`; that target must exist",
                )

    def test_the_failed_suite_hint_names_that_target_family(self) -> None:
        hints = SUITE_HINT_RE.findall(MAKEFILE.read_text())
        self.assertEqual(
            1, len(hints), "expected one per-suite rerun hint in the FAIL block"
        )
        targets = {f"make {name}" for name in declared_make_targets()}
        for suite in all_suites():
            with self.subTest(suite=suite):
                self.assertEqual(suite_rerun(suite), hints[0] % suite)
                self.assertIn(hints[0] % suite, targets)


class TestCapExpiryTellsTheReaderWhatToRun(unittest.TestCase):
    """The budget failure group carries the same rerun as an ordinary one.

    classifyFunctional (scripts/status/verify_run.go) fills `rerun` from
    functionalSuiteRerun for a plain suite failure. A cap expiry publishes its
    own group from the makefile, so an empty field there makes the one failure
    that already cost the run its whole budget the only one with no next step.
    """

    def test_the_budget_group_names_the_suites_own_target(self) -> None:
        targets = declared_make_targets()
        body = expanded_run_suite()
        for suite in (SHARED_BUDGET_SUITE, "install"):
            with self.subTest(suite=suite):
                group = drive(124, suite=suite, body=body).failure_group()
                self.assertEqual(suite_rerun(suite), group.get("rerun", ""))
                self.assertIn(f"ze-functional-{suite}-test", targets)


if __name__ == "__main__":
    unittest.main()
