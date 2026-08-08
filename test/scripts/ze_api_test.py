#!/usr/bin/env python3
"""Unit tests for the ze_api observer fail-closed guard.

WHY THIS FILE EXISTS
--------------------
``_emit_sentinel_if_unwinding`` is the guard that un-swallowed a whole class of
silent test failures: three ``.ci`` tests had been dispatching commands the
daemon answers with "unknown command" for months, and every one of them reported
PASS. The guard fixed that -- and was itself completely untested. Deleting its
two call sites restored the false-pass with nothing going red, which makes the
guard exactly the shape of defect it exists to catch.

Nothing else can cover it:

- A ``.ci`` test cannot assert on the sentinel. The runner treats the sentinel as
  an implicit REJECT on every test (``validateLogging``), so a ``.ci`` that
  deliberately triggered it would fail by construction. The assertion has to live
  outside the runner.
- The Go tests in ``internal/test/runner`` cover the CONSUMER (does the runner
  notice a sentinel line?) and say nothing about the PRODUCER (does the observer
  emit one, and early enough for ze to still be alive to relay it?).

So the guard needs a plain unit test, and this is it. It is collected by
``TestPythonUnitTests`` in ``scripts/dev/python_tests_test.go``.

The load-bearing test here is ``test_call_engine_emits_before_sending``: the
original defect was ORDERING, not a missing handler, so a test that only proves
"a sentinel is eventually written" would have passed against the broken code.
"""

import io
import os
import re
import shutil
import sys
import tempfile
import time
import unittest
import unittest.mock
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

# Importing ze_api installs its own sys.excepthook (it routes an uncaught
# observer exception through runtime_fail, which calls sys.exit). Inside a test
# process that would turn an unexpected crash into a confusing exit rather than a
# reported error, so the original is put back immediately.
_saved_excepthook = sys.excepthook
import ze_api  # noqa: E402  # sys.path is set immediately above

sys.excepthook = _saved_excepthook

REPO_ROOT = Path(__file__).resolve().parents[2]


class SentinelTestCase(unittest.TestCase):
    """Base fixture: capture stderr and reset the write-once latch."""

    def setUp(self):
        self.stderr = io.StringIO()
        self._real_stderr = sys.stderr
        sys.stderr = self.stderr
        # _write_sentinel latches so the first (most specific) reason wins. Each
        # test needs a fresh latch or only the first would ever observe a write.
        self._real_latch = ze_api._sentinel_written
        ze_api._sentinel_written = False

    def tearDown(self):
        sys.stderr = self._real_stderr
        ze_api._sentinel_written = self._real_latch

    def written(self) -> str:
        return self.stderr.getvalue()


class TestSentinelFormat(SentinelTestCase):
    """The sentinel line must be one the runner can actually see."""

    def test_line_shape_matches_runner_expectations(self):
        ze_api._write_sentinel("route 10.0.0.0/24 never arrived")
        line = self.written()

        self.assertIn("ZE-OBSERVER-FAIL", line)
        # classifyStderrLine only passes the relay filter for valid slog with a
        # level; ERROR is what survives regardless of ze.log.relay.
        self.assertIn("level=ERROR", line)
        self.assertIn('msg="ZE-OBSERVER-FAIL: route 10.0.0.0/24 never arrived"', line)
        self.assertIn("subsystem=test.observer", line)
        # extractObserverFailLine slices to the enclosing newline, so the whole
        # reason has to sit on one line.
        self.assertEqual(
            line.count("\n"), 1, f"sentinel must be one line, got {line!r}"
        )

    def test_the_escaped_line_matches_the_go_relay_fixture(self):
        """Close the format join between this writer and the Go parser.

        `msg="..."` is Go slog TEXT format, and that format quotes a string
        value with strconv.Quote (log/slog/handler.go): a `"` inside the value
        is written `\\"` and a `\\` is written `\\\\`. This writer used to
        interpolate the reason raw, so a reason carrying a quote closed msg
        early: run 31225029268 relayed
        `... before prefix: got "destination-ipv4"` as `... got ` and turned the
        remainder into a bogus attribute key. The ZE-OBSERVER-FAIL marker
        survived, so the test still failed -- with the cause cut off.

        Nothing checked the join, because it spans two languages. Both
        constants are read out of the Go relay test that consumes them, so a
        change on either side goes red here rather than in CI six months later.
        """
        go_source = (
            REPO_ROOT / "internal/component/plugin/process/stderr_relay_test.go"
        ).read_text()

        reason = re.search(r"const\s+observerSentinelReason\s*=\s*`([^`]*)`", go_source)
        line = re.search(r"const\s+observerSentinelLine\s*=\s*`([^`]*)`", go_source)
        for name, match in (
            ("observerSentinelReason", reason),
            ("observerSentinelLine", line),
        ):
            self.assertIsNotNone(
                match,
                f"{name} not found in stderr_relay_test.go; if the fixture moved, "
                f"point this test at its new home rather than deleting the check",
            )

        ze_api._write_sentinel(reason.group(1))

        self.assertEqual(
            self.written(),
            line.group(1) + "\n",
            "the observer's sentinel line and the line the Go relay test parses "
            "have diverged; a reason containing a quote or a backslash would be "
            "truncated on relay",
        )

    def test_a_reason_holding_a_newline_stays_one_line(self):
        """Line integrity belongs to the single writer, not to its callers.

        `extractObserverFailLine` (internal/test/runner/runner_validate.go)
        slices to the enclosing newline, so a reason that splits the line hides
        its own tail. Two of the three callers flatten newlines before calling
        here; `runtime_fail` invoked directly by an observer does not, and slog
        escaping covers all three because it encodes a newline as `\\n`.
        """
        ze_api._write_sentinel("first line\nsecond line")
        written = self.written()

        self.assertEqual(written.count("\n"), 1, f"got {written!r}")
        self.assertIn(r"first line\nsecond line", written)

    def test_written_once_so_first_reason_wins(self):
        ze_api._write_sentinel("the specific reason")
        ze_api._write_sentinel("a later generic reason")
        line = self.written()

        self.assertIn("the specific reason", line)
        self.assertNotIn("a later generic reason", line)
        self.assertEqual(line.count("ZE-OBSERVER-FAIL"), 1)

    def test_literal_matches_the_go_runner_constant(self):
        """Close the two-point coupling the Python side documents but nothing checked.

        ze_api.py says "Keeping the literal in code and in the runner makes this a
        two-point coupling; change both." Both sides say that, and until now
        neither side enforced it: renaming one leaves every observer failure
        invisible again, silently, with a green suite.
        """
        go_source = (REPO_ROOT / "internal/test/runner/runner_validate.go").read_text()
        match = re.search(r'const\s+observerFailSentinel\s*=\s*"([^"]+)"', go_source)
        self.assertIsNotNone(
            match,
            "observerFailSentinel not found in runner_validate.go; if it moved, "
            "point this test at its new home rather than deleting the check",
        )
        self.assertEqual(
            match.group(1),
            ze_api._OBSERVER_FAIL_SENTINEL,
            "the Python sentinel and the Go runner constant have diverged; "
            "an observer failure would now be invisible to the runner",
        )


class TestEmitSentinelIfUnwinding(SentinelTestCase):
    """Polarity of the guard: fire while unwinding, stay silent otherwise."""

    def test_fires_while_an_exception_propagates(self):
        try:
            raise AssertionError("expected 3 routes, saw 0")
        except AssertionError:
            # Inside the handler sys.exc_info() is set, which is what the
            # observer's `finally` sees while the exception is still in flight.
            ze_api._emit_sentinel_if_unwinding()

        line = self.written()
        self.assertIn("ZE-OBSERVER-FAIL", line)
        self.assertIn("AssertionError", line)
        self.assertIn("expected 3 routes, saw 0", line)

    def test_silent_when_nothing_is_in_flight(self):
        ze_api._emit_sentinel_if_unwinding()
        self.assertEqual(self.written(), "")

    def test_silent_after_an_except_handler_recovered(self):
        """A recovered error must not fail the test.

        This is the false-positive surface the guard's docstring reasons about.
        Python clears sys.exc_info() once a handler returns, so a `finally`
        reached after a successful `except` sees (None, None, None).
        """
        try:
            try:
                raise ValueError("transient, handled on purpose")
            except ValueError:
                pass
        finally:
            ze_api._emit_sentinel_if_unwinding()

        self.assertEqual(self.written(), "")

    def test_silent_for_system_exit(self):
        """runtime_fail exits via SystemExit after writing its own sentinel."""
        try:
            raise SystemExit(1)
        except SystemExit:
            ze_api._emit_sentinel_if_unwinding()
        self.assertEqual(self.written(), "")

    def test_silent_for_keyboard_interrupt(self):
        try:
            raise KeyboardInterrupt
        except KeyboardInterrupt:
            ze_api._emit_sentinel_if_unwinding()
        self.assertEqual(self.written(), "")

    def test_newlines_in_the_reason_are_flattened(self):
        try:
            raise RuntimeError("first line\nsecond line")
        except RuntimeError:
            ze_api._emit_sentinel_if_unwinding()

        line = self.written()
        self.assertEqual(
            line.count("\n"), 1, "a multi-line reason must not split the sentinel"
        )
        self.assertIn("first line second line", line)


def _bare_api() -> "ze_api.API":
    """An API with only the attributes _call_engine/wait_for_shutdown touch.

    Built with __new__ rather than __init__ so the test never depends on
    transport env vars (ze.plugin.hub.token, ze.engine.fd) or on opening fds.
    """
    api = ze_api.API.__new__(ze_api.API)
    api._tls_mode = False
    api._engine_fd = -1
    api._callback_fd = -1
    api._engine_buf = b""
    api._callback_buf = b""
    api._req_id = 0
    api._shutdown = True
    api._pending_requests = []
    return api


class TestGuardIsWiredIntoTheEngineCalls(SentinelTestCase):
    """The guard has to run at the right MOMENT, not merely run."""

    def test_call_engine_emits_before_sending(self):
        """The defect was ordering, so this asserts ordering.

        An observer's `finally: dispatch(api, 'request shutdown')` stops ze --
        and ze is what relays the observer's stderr to the runner. A sentinel
        written after that dispatch has no carrier left: the runner sees a clean
        exit 0 and the test passes having proven nothing. The guard must fire on
        the way IN, while ze is still up.
        """
        api = _bare_api()
        # What stderr held at the instant the RPC was put on the wire. This is
        # the whole assertion: a sentinel written after this point would have no
        # relay left, because the RPC being sent is the one that stops ze.
        stderr_at_send = []

        def record_send(_fd, _req_id, _method, _params=None):
            stderr_at_send.append(self.written())

        def record_read(*_args, **_kwargs):
            return None  # makes _call_engine raise, ending the call

        api._send_rpc = record_send
        api._read_line = record_read

        try:
            raise AssertionError("observer assertion blew up")
        except AssertionError:
            with self.assertRaises(RuntimeError):
                api._call_engine(
                    "ze-plugin-engine:dispatch-command", {"command": "request shutdown"}
                )

        self.assertEqual(
            len(stderr_at_send), 1, "the engine call should still have been attempted"
        )
        self.assertIn(
            "ZE-OBSERVER-FAIL",
            stderr_at_send[0],
            "the sentinel must already be on stderr when the RPC that stops ze is sent; "
            "writing it afterwards is the swallow this guard exists to close",
        )

    def test_wait_for_shutdown_emits_while_unwinding(self):
        """The second call site: observers whose `finally` waits without dispatching."""
        api = _bare_api()
        try:
            raise AssertionError("no EOR ever arrived")
        except AssertionError:
            api.wait_for_shutdown(timeout=0.01)

        line = self.written()
        self.assertIn("ZE-OBSERVER-FAIL", line)
        self.assertIn("no EOR ever arrived", line)

    def test_wait_for_shutdown_silent_on_a_clean_exit(self):
        api = _bare_api()
        api.wait_for_shutdown(timeout=0.01)
        self.assertEqual(self.written(), "")


class TestGuardedDispatchNeverRaises(SentinelTestCase):
    """dispatch() must hand the failure back, not throw it past the caller."""

    def test_engine_rpc_error_becomes_an_error_result(self):
        """A raising dispatch is what makes a failure unwind through the shutdown.

        _call_engine raises RuntimeError when the engine answers an RPC error --
        an unknown command, a denial. The guarded dispatch returns it instead, so
        the caller's own runtime_fail runs while ze is still alive.
        """
        api = _bare_api()

        def boom(_method, _params=None):
            raise RuntimeError("RPC error from dispatch-command: unknown command")

        api._call_engine = boom

        result = ze_api.dispatch(api, "show bgp nonsense")
        self.assertEqual(result["status"], "error")
        self.assertIn("unknown command", result["data"])
        self.assertEqual(
            self.written(), "", "a handled RPC error is not an observer crash"
        )

    def test_none_response_becomes_an_error_result(self):
        api = _bare_api()
        api._call_engine = lambda _method, _params=None: None

        result = ze_api.dispatch(api, "show bgp summary")
        self.assertEqual(result["status"], "error")


class TestDispatchUntilShortCircuit(SentinelTestCase):
    """A hard command error must not burn the whole attempt budget."""

    def test_unknown_command_stops_polling_immediately(self):
        """Polling cannot fix an unknown command, so retrying it only adds latency.

        Before this, a typo'd command in an observer cost attempts*delay (5s at
        the defaults) before the caller saw the error -- on every such test.
        """
        api = _bare_api()
        calls = []

        def boom(_method, _params=None):
            calls.append(1)
            raise RuntimeError("RPC error from dispatch-command: unknown command")

        api._call_engine = boom

        result = ze_api.dispatch_until(
            api,
            "show bgp nonsense",
            lambda r: r.get("status") == "done",
            attempts=20,
            delay=0.01,
        )
        self.assertEqual(result["status"], "error")
        self.assertEqual(
            len(calls),
            1,
            "an unresolvable command must be surfaced on the first attempt, "
            f"not retried {len(calls)} times",
        )

    def test_a_transient_not_ready_result_is_still_retried(self):
        """The short circuit must not defeat the poll it lives inside."""
        api = _bare_api()
        results = [
            {"status": "pending"},
            {"status": "pending"},
            {"status": "done"},
        ]
        calls = []

        def staged(_method, _params=None):
            calls.append(1)
            return {"result": results[min(len(calls) - 1, len(results) - 1)]}

        api._call_engine = staged

        result = ze_api.dispatch_until(
            api,
            "show bgp summary",
            lambda r: r.get("status") == "done",
            attempts=20,
            delay=0.01,
        )
        self.assertEqual(result["status"], "done")
        self.assertEqual(len(calls), 3)

    def test_a_command_level_error_result_is_still_retried(self):
        """Only TRANSPORT-level unknown-command short-circuits.

        A command that resolves but reports an error payload may well succeed on
        a later poll (a peer that is not up yet, a RIB not yet populated), so the
        short circuit must not swallow that case.
        """
        api = _bare_api()
        calls = []

        def staged(_method, _params=None):
            calls.append(1)
            if len(calls) < 3:
                return {"result": {"status": "error", "error": "peer not established"}}
            return {"result": {"status": "done"}}

        api._call_engine = staged

        result = ze_api.dispatch_until(
            api,
            "show bgp peer 10.0.0.1 detail",
            lambda r: r.get("status") == "done",
            attempts=20,
            delay=0.01,
        )
        self.assertEqual(result["status"], "done")
        self.assertEqual(len(calls), 3)


class TestWaitForDaemonReady(unittest.TestCase):
    """The standalone-driver readiness wait (spec-fixit-migrate-sleeps-infra P1)."""

    def setUp(self):
        self._dir = tempfile.mkdtemp()
        self._cwd = os.getcwd()
        os.chdir(self._dir)
        self.addCleanup(shutil.rmtree, self._dir, True)
        self.addCleanup(os.chdir, self._cwd)

    def test_returns_the_pid_once_both_files_are_present(self):
        Path("daemon.pid").write_text("4242\n")
        Path("daemon.ready").write_text("")
        self.assertEqual(ze_api.wait_for_daemon_ready(attempts=3, delay=0.01), 4242)

    def test_an_empty_pid_file_is_waited_out_not_parsed(self):
        """The race this helper exists to close.

        ze creates daemon.pid before writing it, so an exists() check can be
        followed by a read of an EMPTY file. The old hand-rolled loop did
        exactly that and died on int(""); the pid parse must be part of the
        WAIT, so a half-written file is simply "not yet".
        """
        Path("daemon.pid").write_text("")
        Path("daemon.ready").write_text("")

        writes = []
        real_sleep = time.sleep

        def fill(_delay):
            writes.append(1)
            if len(writes) == 2:
                Path("daemon.pid").write_text("77\n")
            real_sleep(0)

        with unittest.mock.patch.object(time, "sleep", fill):
            pid = ze_api.wait_for_daemon_ready(attempts=10, delay=0.01)

        self.assertEqual(pid, 77)

    def test_timeout_names_the_missing_file(self):
        Path("daemon.pid").write_text("9\n")
        with self.assertRaises(TimeoutError) as caught:
            ze_api.wait_for_daemon_ready(attempts=2, delay=0.01)
        self.assertIn("daemon.ready", str(caught.exception))

    def test_a_ready_file_alone_is_not_enough(self):
        """Readiness without a readable pid is not readiness.

        The driver's very next act is os.kill(pid, SIGTERM); returning before a
        pid exists would hand it a zero.
        """
        Path("daemon.ready").write_text("")
        with self.assertRaises(TimeoutError) as caught:
            ze_api.wait_for_daemon_ready(attempts=2, delay=0.01)
        self.assertIn("daemon.pid", str(caught.exception))


class TestWaitForOutput(unittest.TestCase):
    """The kernel-readback wait (spec-fixit-migrate-sleeps-infra P3)."""

    def test_returns_the_first_output_satisfying_the_predicate(self):
        out = ze_api.wait_for_output(
            ["printf", "table inet ze_pr"],
            lambda text: "ze_pr" in text,
            attempts=3,
            delay=0.01,
        )
        self.assertEqual(out, "table inet ze_pr")

    def test_a_failing_command_is_retried_not_raised(self):
        """`nft list table` exits non-zero until the table exists.

        That is the normal state while the daemon is still applying, so it must
        count as "not yet" rather than kill the driver.
        """
        script = Path(self._tmp()) / "flaky.sh"
        marker = script.parent / "attempts"
        script.write_text(
            "#!/bin/sh\n"
            f'n=$(cat "{marker}" 2>/dev/null || echo 0)\n'
            f'n=$((n + 1)); echo "$n" > "{marker}"\n'
            '[ "$n" -lt 3 ] && exit 1\n'
            'echo "table inet ze_pr"\n'
        )
        script.chmod(0o755)

        out = ze_api.wait_for_output(
            [str(script)], lambda text: "ze_pr" in text, attempts=10, delay=0.01
        )
        self.assertEqual(out.strip(), "table inet ze_pr")
        self.assertEqual(marker.read_text().strip(), "3")

    def test_an_absence_wait_completes_when_the_command_starts_failing(self):
        """A withdrawn nft table presents as a non-zero exit, not as empty output.

        The predicate is therefore consulted with "" on a failed turn, so
        `lambda out: "ze_pr" not in out` -- how every teardown assertion is
        spelled -- can ever become true.

        Asserted by CALL COUNT, not by return value: a version that skipped the
        predicate whenever the command failed would still return "" here, after
        burning the entire attempt budget. Only the count separates "satisfied
        on turn 1" from "gave up on turn 200".
        """
        marker = Path(self._tmp()) / "attempts"
        script = marker.parent / "always-fails.sh"
        script.write_text(
            "#!/bin/sh\n"
            f'n=$(cat "{marker}" 2>/dev/null || echo 0)\n'
            f'echo "$((n + 1))" > "{marker}"\n'
            "exit 1\n"
        )
        script.chmod(0o755)

        out = ze_api.wait_for_output(
            [str(script)], lambda text: "ze_pr" not in text, attempts=25, delay=0.01
        )
        self.assertEqual(out, "")
        self.assertEqual(
            marker.read_text().strip(),
            "1",
            "an absence wait must be satisfied on the first failing turn, "
            "not after the whole budget",
        )

    def test_exhaustion_returns_the_last_output_instead_of_raising(self):
        """The caller's own assertions must produce the failure message.

        Raising here would collapse every converted driver's specific per-phase
        message into one generic timeout.
        """
        out = ze_api.wait_for_output(
            ["printf", "nothing useful"],
            lambda _text: False,
            attempts=2,
            delay=0.01,
        )
        self.assertEqual(out, "nothing useful")

    def test_a_missing_binary_is_retried_not_raised(self):
        out = ze_api.wait_for_output(
            ["/nonexistent/ze-not-a-real-binary"],
            lambda text: text != "",
            attempts=2,
            delay=0.01,
        )
        self.assertEqual(out, "")

    def _tmp(self):
        path = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, path, True)
        return path


class TestObserverBudget(unittest.TestCase):
    """Guard 2 of plan/spec-fixit-test-harness-fail-open-guards.md.

    VALIDATES: the observer sizes its waits from the runner's deadline, so both
    stay reachable inside it.
    PREVENTS: the constant that made ZE-OBSERVER-FAIL unreachable at all 17 call
    sites. Every one ran a 10s, 15s or 20s .ci timeout against a 30.0s default,
    so the runner always killed the process before the diagnosis could fire.
    """

    def test_go_durations_parse(self):
        for raw, want in (
            ("30s", 30.0),
            ("1.5s", 1.5),
            ("250ms", 0.25),
            ("1m30s", 90.0),
            ("2m", 120.0),
            ("1h", 3600.0),
        ):
            with self.subTest(raw=raw):
                self.assertAlmostEqual(ze_api._parse_go_duration(raw), want)

    def test_an_unreadable_duration_raises_rather_than_reading_zero(self):
        # Zero is the dangerous answer: every derived wait would expire at once,
        # so this must raise and let the caller keep its own default.
        for raw in ("", "30", "abc", "30s junk", "junk30s", "30x"):
            with self.subTest(raw=raw):
                with self.assertRaises(ValueError):
                    ze_api._parse_go_duration(raw)

    def test_budget_is_none_when_the_runner_did_not_publish_one(self):
        for key in ("ze.test.budget", "ze_test_budget", "ZE_TEST_BUDGET"):
            os.environ.pop(key, None)
        self.assertIsNone(ze_api._test_budget_seconds())

    def test_budget_is_read_from_the_runner_variable(self):
        os.environ["ze_test_budget"] = "20s"
        self.addCleanup(os.environ.pop, "ze_test_budget", None)
        got = ze_api._test_budget_seconds()
        self.assertIsNotNone(got)
        self.assertAlmostEqual(got, 20.0)

    def test_an_unreadable_budget_falls_back_rather_than_to_zero(self):
        os.environ["ze_test_budget"] = "not-a-duration"
        self.addCleanup(os.environ.pop, "ze_test_budget", None)
        self.assertIsNone(ze_api._test_budget_seconds())

    def test_both_shares_stay_inside_the_deadline_that_kills_the_process(self):
        # The property that makes the diagnosis reachable: each wait, and their
        # sum, must finish before the runner's deadline. Checked at the three
        # budgets every current caller uses.
        for budget in (10.0, 15.0, 20.0):
            with self.subTest(budget=budget):
                eor = budget * ze_api._EOR_BUDGET_SHARE
                shutdown = budget * ze_api._SHUTDOWN_BUDGET_SHARE
                self.assertLess(eor, budget)
                self.assertLess(shutdown, budget)
                self.assertLess(eor + shutdown, budget)
                # And large enough to be useful: the measured replay for these
                # tests is 1.1s to 1.6s, so the smallest window keeps ~4x margin.
                self.assertGreater(eor, 3.0)


if __name__ == "__main__":
    unittest.main()
