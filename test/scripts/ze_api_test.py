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
import sys
import unittest
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


if __name__ == "__main__":
    unittest.main()
